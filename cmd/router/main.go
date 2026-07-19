package main

import (
	"bufio"
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

func (r *Router) GetNode(key string) Node {
	h := fnv.New32a()
	h.Write([]byte(key))

	index := h.Sum32() % uint32(len(r.Nodes))

	return r.Nodes[index]
}

func (r *Router) Handle(client net.Conn) {
	defer client.Close()

	scanner := bufio.NewScanner(client)

	for scanner.Scan() {
		line := scanner.Text()

		cmd, err := parse(line)
		if err != nil {
			client.Write([]byte("ERR invalid command\n"))
			continue
		}

		key := cmd.Args[0]

		node := r.GetNode(key)

		response, err := r.Forward(node, line)
		if err != nil {
			client.Write([]byte("ERR node unavailable\n"))
			continue
		}

		client.Write([]byte(response + "\n"))
	}
}

func (r *Router) Forward(node Node, command string) (string, error) {

	conn, err := net.Dial("tcp", node.Address)
	if err != nil {
		return "", err
	}
	defer conn.Close()

	_, err = fmt.Fprintf(conn, "%s\n", command)
	if err != nil {
		return "", err
	}

	reply, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(reply), nil
}

func main() {
	router := Router{
		Nodes: []Node{
			{ID: 1, Address: "localhost:7001"},
			{ID: 2, Address: "localhost:7002"},
			{ID: 3, Address: "localhost:7003"},
		},
	}

	listener, _ := net.Listen("tcp", ":6379")

	for {

		conn, _ := listener.Accept()

		go router.Handle(conn)
	}
}
