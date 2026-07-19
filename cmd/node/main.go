package main

import (
	"bufio"
	"bytes"
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

	fmt.Println("Listening on :7001")

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

	scanner := bufio.NewScanner(conn)

	for scanner.Scan() {
		line := scanner.Text()

		cmd, err := parse(line)
		if err != nil {
			conn.Write([]byte("ERR " + err.Error() + "\n"))
			continue
		}

		response := execute(cmd)

		conn.Write([]byte(response + "\n"))
	}

	fmt.Println("Disconnected: ", conn.RemoteAddr())
}

func execute(cmd Command) string {
	switch cmd.Name {
	case "PING":
		return "PONG"
	case "GET":
		if len(cmd.Args) != 1 {
			return "ERR GET key"
		}

		cache.mu.RLock()
		value, ok := cache.data[cmd.Args[0]]
		cache.mu.RUnlock()

		if !ok {
			return "(nil)"
		}

		return string(value)
	case "SET":
		if len(cmd.Args) != 2 {
			return "ERR SET key value"
		}

		cache.mu.Lock()
		cache.data[cmd.Args[0]] = []byte(cmd.Args[1])
		cache.mu.Unlock()

		return "OK"
	case "DEL":
		if len(cmd.Args) != 1 {
			return "ERR DEL key"
		}

		cache.mu.Lock()
		delete(cache.data, cmd.Args[0])
		cache.mu.Unlock()

		return "OK"
	case "KEYS":
		cache.mu.RLock()
		defer cache.mu.RUnlock()

		var b bytes.Buffer

		for k := range cache.data {
			b.WriteString(k)
			b.WriteByte(' ')
		}

		return b.String()
	default:
		return "ERR unknown command: " + cmd.Name
	}
}
