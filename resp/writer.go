package resp

import (
	"fmt"
	"net"
)

func WriteCommand(conn net.Conn, args []string) error {
	if _, err := fmt.Fprintf(conn, "*%d\r\n", len(args)); err != nil {
		return err
	}
	for _, arg := range args {
		if err := WriteBulkBytes(conn, []byte(arg)); err != nil {
			return err
		}
	}
	return nil
}

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

func WriteBulkBytes(conn net.Conn, b []byte) error {
	if _, err := fmt.Fprintf(conn, "$%d\r\n", len(b)); err != nil {
		return err
	}
	if _, err := conn.Write(b); err != nil {
		return err
	}
	_, err := conn.Write([]byte("\r\n"))
	return err
}

func WriteArray(conn net.Conn, arr []string) {
	fmt.Fprintf(conn, "*%d\r\n", len(arr))

	for _, s := range arr {
		WriteBulkString(conn, s)
	}
}

func WriteNestedArray(conn net.Conn, values [][]string) {

	fmt.Fprintf(conn, "*%d\r\n", len(values))

	for _, arr := range values {
		WriteArray(conn, arr)
	}
}

func WriteScan(conn net.Conn, cursor string, keys []string) {

	// outer array size = 2
	fmt.Fprintf(conn, "*2\r\n")

	// cursor
	fmt.Fprintf(conn, "$%d\r\n%s\r\n",
		len(cursor),
		cursor,
	)

	// keys array
	WriteArray(conn, keys)
}
