package resp

import (
	"fmt"
	"net"
)

func WriteSimpleString(conn net.Conn, s string) {
	fmt.Fprintf(conn, "+%s\r\n", s)
}

func WriteError(conn net.Conn, s string) {
	fmt.Fprintf(conn, "-ERR %s\r\n", s)
}

func WriteInteger(conn net.Conn, n int) {
	fmt.Fprintf(conn, ":%d\r\n", n)
}

func WriteNull(conn net.Conn) {
	conn.Write([]byte("$-1\r\n"))
}

func WriteBulkString(conn net.Conn, s string) {
	fmt.Fprintf(conn, "$%d\r\n%s\r\n", len(s), s)
}

func WriteBulkBytes(conn net.Conn, b []byte) {
	fmt.Fprintf(conn, "$%d\r\n", len(b))
	conn.Write(b)
	conn.Write([]byte("\r\n"))
}

func WriteArray(conn net.Conn, arr []string) {
	fmt.Fprintf(conn, "*%d\r\n", len(arr))

	for _, s := range arr {
		WriteBulkString(conn, s)
	}
}
