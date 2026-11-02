package main

import (
	"bufio"
	"flag"
	"fmt"
	"hash/crc32"
	"net"
	"sort"
	"strings"
	"sync"
	"time"

	"distributed-redis/resp"
)

type Node struct {
	ID      int
	Address string
}
type ringPoint struct {
	hash uint32
	node Node
}
type backendConn struct {
	mu     sync.Mutex
	conn   net.Conn
	reader *bufio.Reader
}
type Router struct {
	Nodes    []Node
	Replicas int
	points   []ringPoint
	backends map[string]*backendConn
}

func NewRouter(nodes []Node, replicas int) *Router {
	if replicas < 1 {
		replicas = 1
	}
	r := &Router{Nodes: nodes, Replicas: replicas, backends: make(map[string]*backendConn, len(nodes))}
	for _, node := range nodes {
		if _, exists := r.backends[node.Address]; !exists {
			r.backends[node.Address] = &backendConn{}
		}
		for i := 0; i < 128; i++ {
			key := fmt.Sprintf("%s#%d", node.Address, i)
			r.points = append(r.points, ringPoint{crc32.ChecksumIEEE([]byte(key)), node})
		}
	}
	sort.Slice(r.points, func(i, j int) bool { return r.points[i].hash < r.points[j].hash })
	return r
}

func (r *Router) GetNodes(key string) []Node {
	if len(r.points) == 0 {
		return nil
	}
	h := crc32.ChecksumIEEE([]byte(key))
	start := sort.Search(len(r.points), func(i int) bool { return r.points[i].hash >= h })
	if start == len(r.points) {
		start = 0
	}
	seen := map[string]bool{}
	var out []Node
	for i := 0; i < len(r.points) && len(out) < r.Replicas; i++ {
		n := r.points[(start+i)%len(r.points)].node
		if !seen[n.Address] {
			seen[n.Address] = true
			out = append(out, n)
		}
	}
	return out
}

func GetKey(args []string) (string, error) {
	if len(args) == 0 {
		return "", fmt.Errorf("empty command")
	}
	switch strings.ToUpper(args[0]) {
	case "GET", "DEL":
		if len(args) != 2 {
			return "", fmt.Errorf("invalid command")
		}
		return args[1], nil
	case "SET":
		if len(args) < 3 {
			return "", fmt.Errorf("invalid command")
		}
		return args[1], nil
	default:
		return "", nil
	}
}

func (r *Router) Handle(client net.Conn) {
	defer client.Close()
	reader := bufio.NewReader(client)
	for {
		args, err := resp.Parse(reader)
		if err != nil {
			return
		}
		if len(args) == 0 {
			resp.WriteError(client, "empty command")
			continue
		}
		cmd := strings.ToUpper(args[0])
		if cmd == "KEYS" || cmd == "SCAN" {
			if err := r.ForwardAll(args, client); err != nil {
				resp.WriteError(client, "node unavailable")
			}
			continue
		}
		key, err := GetKey(args)
		if err != nil {
			resp.WriteError(client, err.Error())
			continue
		}
		if key == "" {
			var forwarded bool
			for _, node := range r.Nodes {
				if err := r.Forward(node, args, client); err == nil {
					forwarded = true
					break
				}
			}
			if !forwarded {
				resp.WriteError(client, "node unavailable")
			}
			continue
		}
		owners := r.GetNodes(key)
		if len(owners) == 0 {
			resp.WriteError(client, "no nodes configured")
			continue
		}
		if cmd == "GET" {
			var ok bool
			for _, node := range owners {
				if err := r.Forward(node, args, client); err == nil {
					ok = true
					break
				}
			}
			if !ok {
				resp.WriteError(client, "node unavailable")
			}
			continue
		}
		var response []byte
		live := 0
		for _, node := range owners {
			value, err := r.Request(node, args)
			if err == nil {
				live++
				if response == nil {
					response = value
				}
			}
		}
		if live == 0 {
			resp.WriteError(client, "node unavailable")
		} else {
			_, _ = client.Write(response)
		}
	}
}

func (r *Router) Request(node Node, args []string) ([]byte, error) {
	backend := r.backends[node.Address]
	if backend == nil {
		return nil, fmt.Errorf("unknown backend %q", node.Address)
	}
	backend.mu.Lock()
	defer backend.mu.Unlock()
	for attempt := 0; attempt < 2; attempt++ {
		if backend.conn == nil {
			conn, err := net.DialTimeout("tcp", node.Address, 500*time.Millisecond)
			if err != nil {
				return nil, err
			}
			backend.conn, backend.reader = conn, bufio.NewReader(conn)
		}
		_ = backend.conn.SetDeadline(time.Now().Add(5 * time.Second))
		if err := resp.WriteCommand(backend.conn, args); err == nil {
			value, err := resp.ReadValue(backend.reader)
			if err == nil {
				_ = backend.conn.SetDeadline(time.Time{})
				return value, nil
			}
		}
		_ = backend.conn.Close()
		backend.conn, backend.reader = nil, nil
	}
	return nil, fmt.Errorf("backend %s request failed", node.Address)
}
func (r *Router) Forward(node Node, args []string, client net.Conn) error {
	b, err := r.Request(node, args)
	if err != nil {
		return err
	}
	_, err = client.Write(b)
	return err
}

func (r *Router) ForwardAll(args []string, client net.Conn) error {
	seen := map[string]bool{}
	var keys []string
	cursor := "0"
	for _, node := range r.Nodes {
		command := []string{"KEYS", "*"}
		if strings.EqualFold(args[0], "SCAN") {
			command = []string{"SCAN", "0"}
			if len(args) > 2 {
				command = append(command, args[2:]...)
			}
		}
		b, err := r.Request(node, command)
		if err != nil {
			continue
		}
		value, err := resp.ParseValue(b)
		if err != nil {
			continue
		}
		if strings.EqualFold(args[0], "SCAN") {
			if len(value.Array) > 0 {
				cursor = value.Array[0].Text
			}
			if len(value.Array) > 1 {
				for _, v := range value.Array[1].Array {
					if !seen[v.Text] {
						seen[v.Text] = true
						keys = append(keys, v.Text)
					}
				}
			}
		} else {
			for _, v := range value.Array {
				if !seen[v.Text] {
					seen[v.Text] = true
					keys = append(keys, v.Text)
				}
			}
		}
	}
	sort.Strings(keys)
	if strings.EqualFold(args[0], "SCAN") {
		resp.WriteScan(client, cursor, keys)
	} else {
		resp.WriteArray(client, keys)
	}
	return nil
}

func main() {
	listen := flag.String("listen", ":6380", "router listen address")
	nodeList := flag.String("nodes", "localhost:7001,localhost:7002,localhost:7003", "comma-separated storage nodes")
	replicas := flag.Int("replicas", 2, "copies per key")
	flag.Parse()
	var nodes []Node
	for i, address := range strings.Split(*nodeList, ",") {
		address = strings.TrimSpace(address)
		if address != "" {
			nodes = append(nodes, Node{ID: i + 1, Address: address})
		}
	}
	r := NewRouter(nodes, *replicas)
	listener, err := net.Listen("tcp", *listen)
	if err != nil {
		panic(err)
	}
	defer listener.Close()
	fmt.Printf("Listening on %s with %d nodes and %d replicas\n", *listen, len(nodes), *replicas)
	for {
		conn, err := listener.Accept()
		if err != nil {
			continue
		}
		go r.Handle(conn)
	}
}
