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

Run a separate real Redis server on port 6379, then capture results with the same client, workload, and host:

```sh
redis-benchmark -h 127.0.0.1 -p 6379 -t set,get -n 100000 -c 50 -q
redis-benchmark -h 127.0.0.1 -p 6380 -t set,get -n 100000 -c 50 -q
```

Report the measured throughput as `distributed ops/sec ÷ Redis ops/sec × 100%`. Results depend heavily on host, concurrency, payload, and Redis version. No throughput percentage is claimed here because a real Redis server and `redis-benchmark` were not available in the implementation environment. The service has not been validated against the full Redis command set or Redis Cluster protocol.
