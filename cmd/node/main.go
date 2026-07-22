package main

import (
	"bufio"
	"distributed-redis/resp"
	"flag"
	"fmt"
	"net"
	"strings"
	"sync"
)

type Cache struct {
	data map[string][]byte
	mu   sync.RWMutex
}

var cache = Cache{
	data: make(map[string][]byte),
}

type Command struct {
	Name string
	Args []string
}

func parse(line string) (Command, error) {
	fields := strings.Fields(line)

	if len(fields) == 0 {
		return Command{}, fmt.Errorf("empty command")
	}

	cmd := Command{
		Name: strings.ToUpper(fields[0]),
		Args: fields[1:],
	}

	return cmd, nil
}

func main() {
	port := flag.Int("port", 7001, "TCP port")
	flag.Parse()

	addr := fmt.Sprintf(":%d", *port)
	listner, err := net.Listen("tcp", addr)
	if err != nil {
		panic(err)
	}
	defer listner.Close()

	fmt.Println("Listening on :" + addr)

	for {
		conn, err := listner.Accept()
		if err != nil {
			fmt.Println("Error accepting connection:", err)
			continue
		}

		go handleConnection(conn)
	}
}

func handleConnection(conn net.Conn) {
	defer conn.Close()

	fmt.Println("Connected: ", conn.RemoteAddr())

	reader := bufio.NewReader(conn)

	for {
		args, err := resp.Parse(reader)
		if err != nil {
			return
		}

		fmt.Println("Received:", args)

		execute(args, conn)
	}

	fmt.Println("Disconnected: ", conn.RemoteAddr())
}
func execute(args []string, conn net.Conn) {
	if len(args) == 0 {
		resp.WriteError(conn, "empty command")
		return
	}

	cmd := strings.ToUpper(args[0])

	switch cmd {

	case "PING":
		resp.WriteSimpleString(conn, "PONG")

	case "AUTH":
		// Ignore authentication for now.
		resp.WriteSimpleString(conn, "OK")

	case "CLIENT":
		// CLIENT SETNAME ...
		resp.WriteSimpleString(conn, "OK")

	case "HELLO":
		// Minimal implementation.
		resp.WriteSimpleString(conn, "OK")

	case "INFO":
		info := `# Server
redis_version:7.2.0

# Replication
role:master

# Cluster
cluster_enabled:0
`
		resp.WriteBulkString(conn, info)

	case "GET":
		if len(args) != 2 {
			resp.WriteError(conn, "wrong number of arguments for 'get' command")
			return
		}

		cache.mu.RLock()
		value, ok := cache.data[args[1]]
		cache.mu.RUnlock()

		if !ok {
			resp.WriteNull(conn)
			return
		}

		resp.WriteBulkString(conn, string(value))

	case "SET":
		if len(args) != 3 {
			resp.WriteError(conn, "wrong number of arguments for 'set' command")
			return
		}

		cache.mu.Lock()
		cache.data[args[1]] = []byte(args[2])
		cache.mu.Unlock()

		resp.WriteSimpleString(conn, "OK")

	case "DEL":
		if len(args) != 2 {
			resp.WriteError(conn, "wrong number of arguments for 'del' command")
			return
		}

		cache.mu.Lock()
		delete(cache.data, args[1])
		cache.mu.Unlock()

		resp.WriteInteger(conn, 1)

	case "KEYS":
		cache.mu.RLock()

		keys := make([]string, 0, len(cache.data))
		for k := range cache.data {
			keys = append(keys, k)
		}

		cache.mu.RUnlock()

		resp.WriteArray(conn, keys)

	case "CONFIG":
		if len(args) >= 3 && strings.ToUpper(args[1]) == "GET" {
			resp.WriteArray(conn, []string{
				"databases",
				"1",
			})
			return
		}

	case "DBSIZE":
		cache.mu.RLock()
		n := len(cache.data)
		cache.mu.RUnlock()

		resp.WriteInteger(conn, n)

	default:
		resp.WriteError(conn, "unknown command '"+strings.ToLower(cmd)+"'")
	}
}
