package main

import (
	"bufio"
	"distributed-redis/resp"
	"fmt"
	"hash/fnv"
	"net"
	"strings"
)

type Node struct {
	ID      int
	Address string
}

type Router struct {
	Nodes []Node
}

type Command struct {
	Name string
	Args []string
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
		return "", nil // commands without keys
	}
}

func (r *Router) GetNode(key string) Node {
	h := fnv.New32a()
	h.Write([]byte(key))

	index := h.Sum32() % uint32(len(r.Nodes))

	return r.Nodes[index]
}

func (r *Router) Handle(client net.Conn) {
	defer client.Close()

	//scanner := bufio.NewScanner(client)
	reader := bufio.NewReader(client)

	for {
		args, err := resp.Parse(reader)
		if err != nil {
			return
		}

		fmt.Println(args)
		cmd := strings.ToUpper(args[0])

		fmt.Printf("%#v\n", args)
		switch cmd {
		case "KEYS":
			err = r.ForwardAll(args, client)
		default:
			key, err := GetKey(args)
			if err != nil {
				return
			}

			node := r.GetNode(key)
			err = r.Forward(node, args, client)
		}

		if err != nil {
			client.Write([]byte("ERR node unavailable\n"))
			continue
		}
	}

	//for scanner.Scan() {
	//	line := scanner.Text()
	//
	//	cmd, err := parse(line)
	//	if err != nil {
	//		client.Write([]byte("ERR invalid command\n"))
	//		continue
	//	}
	//
	//	key := cmd.Args[0]
	//
	//	node := r.GetNode(key)
	//
	//	response, err := r.Forward(node, line)
	//	if err != nil {
	//		client.Write([]byte("ERR node unavailable\n"))
	//		continue
	//	}
	//
	//	client.Write([]byte(response + "\n"))
	//}
}

func (r *Router) KeysFromNode(node Node) ([]string, error) {
	conn, err := net.Dial("tcp", node.Address)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	// Send KEYS command
	resp.WriteArray(conn, []string{"KEYS"})

	reader := bufio.NewReader(conn)

	b, err := reader.Peek(1024)
	fmt.Printf("%q\n", b)

	// Parse RESP array returned by the node
	keys, err := resp.Parse(reader)
	if err != nil {
		return nil, err
	}

	return keys, nil
}

func (r *Router) ForwardAll(args []string, client net.Conn) error {
	var allKeys []string

	for _, node := range r.Nodes {

		keys, err := r.KeysFromNode(node)
		if err != nil {
			return err
		}

		allKeys = append(allKeys, keys...)
	}

	resp.WriteArray(client, allKeys)
	return nil
}

func (r *Router) Forward(node Node, args []string, client net.Conn) error {
	conn, err := net.Dial("tcp", node.Address)
	if err != nil {
		return err
	}
	defer conn.Close()

	resp.WriteArray(conn, args)

	reader := bufio.NewReader(conn)

	return resp.ForwardValue(reader, client)
}

func main() {
	router := Router{
		Nodes: []Node{
			{ID: 1, Address: "localhost:7001"},
			{ID: 2, Address: "localhost:7002"},
			{ID: 3, Address: "localhost:7003"},
		},
	}

	listener, _ := net.Listen("tcp", ":6380")

	for {

		conn, _ := listener.Accept()

		go router.Handle(conn)
	}
}
