package resp

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
)

func ReadValue(reader *bufio.Reader) ([]byte, error) {
	var out bytes.Buffer
	if err := copyValue(reader, &out, 0); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func copyLine(reader *bufio.Reader, out io.Writer) (string, error) {
	line, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	if _, err = io.WriteString(out, line); err != nil {
		return "", err
	}
	if len(line) < 2 || line[len(line)-2:] != "\r\n" {
		return "", fmt.Errorf("malformed RESP line")
	}
	return line[:len(line)-2], nil
}

func copyValue(reader *bufio.Reader, out io.Writer, depth int) error {
	if depth > 64 {
		return fmt.Errorf("RESP nesting too deep")
	}
	prefix, err := reader.ReadByte()
	if err != nil {
		return err
	}
	if _, err = out.Write([]byte{prefix}); err != nil {
		return err
	}
	line, err := copyLine(reader, out)
	if err != nil {
		return err
	}
	switch prefix {
	case '+', '-', ':', ',', '#', '_', '(':
		return nil
	case '$', '!', '=':
		n, err := parseLength(line)
		if err != nil {
			return err
		}
		if n == -1 && prefix == '$' {
			return nil
		}
		if n < 0 || n > 64<<20 {
			return fmt.Errorf("invalid RESP bulk length")
		}
		_, err = io.CopyN(out, reader, int64(n+2))
		return err
	case '*', '~', '>':
		n, err := parseLength(line)
		if err != nil {
			return err
		}
		if n == -1 && prefix == '*' {
			return nil
		}
		if n < 0 || n > 1<<20 {
			return fmt.Errorf("invalid RESP array length")
		}
		for i := 0; i < n; i++ {
			if err := copyValue(reader, out, depth+1); err != nil {
				return err
			}
		}
		return nil
	case '%':
		n, err := parseLength(line)
		if err != nil {
			return err
		}
		if n < 0 || n > 1<<20 {
			return fmt.Errorf("invalid RESP map length")
		}
		for i := 0; i < n*2; i++ {
			if err := copyValue(reader, out, depth+1); err != nil {
				return err
			}
		}
		return nil
	default:
		return fmt.Errorf("unknown RESP type %q", prefix)
	}
}

func parseLength(s string) (int, error) { var n int; _, err := fmt.Sscanf(s, "%d", &n); return n, err }

func ForwardValue(reader *bufio.Reader, writer io.Writer) error { return copyValue(reader, writer, 0) }
