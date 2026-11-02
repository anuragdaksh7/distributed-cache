# Distributed Redis (learning project)

A small RESP2-speaking, sharded in-memory key/value service written in Go. It is intended for learning and local experiments, not as a production Redis replacement.

## Run

Start three storage nodes in separate terminals:

```sh
go run ./cmd/node --port=7001
go run ./cmd/node --port=7002
go run ./cmd/node --port=7003
```

Start the router:

```sh
go run ./cmd/router --listen=:6380 --nodes=localhost:7001,localhost:7002,localhost:7003 --replicas=2
```

Connect with standard Redis clients:

```sh
redis-cli -p 6380 SET greeting hello
redis-cli -p 6380 GET greeting
redis-benchmark -p 6380 -t set,get -n 100000 -c 50
```

The node accepts RESP2 commands directly as well. The router supports RESP2 simple strings, errors, integers, bulk strings, arrays, nulls, and common RESP3 value types when relaying server replies. Supported data commands include PING, GET, SET, DEL, KEYS, SCAN, DBSIZE and INFO; AUTH, CLIENT, HELLO and CONFIG are minimal compatibility responses.

## Sharding and replication

The router places 128 virtual points per node on a CRC32 consistent-hash ring. Adding or removing a node remaps only the keys adjacent to changed ring points. The `--replicas` option controls how many distinct nodes own each key (default 2; it is capped naturally by the number of nodes). Writes are sent synchronously to all owners. Reads try owners in ring order, so a stopped primary can be bypassed to a live replica. If no owner is reachable, the router returns an error.

This is best-effort replication: there is no disk persistence, replica repair, membership protocol, quorum, or automatic promotion state. A write may have reached one replica even when another is down; restarting a node does not backfill missed writes. Use the same node list and replica count at every router.

## Benchmarking against Redis

### Router baseline before persistent backend connections

Measured from WSL using `redis-benchmark` against the router at `172.26.64.1:6380`. Each run used 100,000 requests, 50 clients, and a 3-byte payload. The router was running with four nodes and two replicas. These are two observed runs; use the same setup and benchmark options to compare changes.

| Run | SET ops/s | SET p50 (ms) | GET ops/s | GET p50 (ms) |
|---:|---:|---:|---:|---:|
| 1 | 221.37 | 194.431 | 458.37 | 86.719 |
| 2 | 252.03 | 166.271 | 564.70 | 71.487 |
| Mean | 236.70 | 180.351 | 511.54 | 79.103 |

### After persistent backend connections

Two runs with four nodes and two replicas, using the same reported benchmark options:

| Run | SET ops/s | SET p50 (ms) | GET ops/s | GET p50 (ms) |
|---:|---:|---:|---:|---:|
| 1 | 5,097.62 | 9.367 | 7,760.36 | 5.847 |
| 2 | 5,080.78 | 9.159 | 8,743.55 | 5.583 |
| Mean | 5,089.20 | 9.263 | 8,251.96 | 5.715 |

Compared with the pre-change means, mean SET throughput increased about 21.5× and GET throughput about 16.1×. Mean p50 latency decreased about 19.5× for SET and 13.8× for GET. Both sets of results use the same reported four-node, two-replica configuration and benchmark options.

Command used:

```sh
redis-benchmark -h 172.26.64.1 -p 6380 -t set,get -n 100000 -c 50 -d 3 -q
```

Run a separate real Redis server on port 6379, then capture results with the same client, workload, and host:

```sh
redis-benchmark -h 127.0.0.1 -p 6379 -t set,get -n 100000 -c 50 -q
redis-benchmark -h 127.0.0.1 -p 6380 -t set,get -n 100000 -c 50 -q
```

Report the measured throughput as `distributed ops/sec ÷ Redis ops/sec × 100%`. Results depend heavily on host, concurrency, payload, and Redis version. The service has not been validated against the full Redis command set or Redis Cluster protocol.

## Possible next major win

Establish a repeatable benchmark with the node count and replica count recorded, then profile the router under that fixed workload. The current router reuses one persistent backend connection per node and serializes requests on that connection. A small connection pool per node could reduce queueing for concurrent requests; benchmark it against the same baseline before keeping the added complexity. A separate reliability milestone is a repeatable node-kill test that confirms keys with a live replica remain readable through the router.
