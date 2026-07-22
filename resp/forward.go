package resp

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"strconv"
)

func ForwardValue(reader *bufio.Reader, conn net.Conn) error {
	prefix, err := reader.ReadByte()
	if err != nil {
		return err
	}

	conn.Write([]byte{prefix})

	switch prefix {

	case '+', '-', ':':
		line, err := reader.ReadString('\n')
		if err != nil {
			return err
		}
		_, err = conn.Write([]byte(line))
		return err

	case '$':
		line, err := reader.ReadString('\n')
		if err != nil {
			return err
		}
		conn.Write([]byte(line))

		n, err := strconv.Atoi(line[:len(line)-2])
		if err != nil {
			return err
		}

		if n == -1 {
			return nil
		}

		_, err = io.CopyN(conn, reader, int64(n+2)) // data + CRLF
		return err

	case '*':
		line, err := reader.ReadString('\n')
		if err != nil {
			return err
		}
		conn.Write([]byte(line))

		count, err := strconv.Atoi(line[:len(line)-2])
		if err != nil {
			return err
		}

		for i := 0; i < count; i++ {
			if err := ForwardValue(reader, conn); err != nil {
				return err
			}
		}

		return nil
	}

	return fmt.Errorf("unknown RESP type %q", prefix)
}
