package resp

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
)

const maxLineLength = 1 << 20
const maxBulkLength = 64 << 20
const maxArrayLength = 1 << 20

func readLine(r *bufio.Reader) (string, error) {
	line, err := r.ReadString('\n')
	if err != nil {
		return "", err
	}
	if len(line) > maxLineLength {
		return "", fmt.Errorf("RESP line too large")
	}
	if !strings.HasSuffix(line, "\r\n") {
		return "", fmt.Errorf("malformed RESP line")
	}
	return strings.TrimSuffix(line, "\r\n"), nil
}

func Parse(reader *bufio.Reader) ([]string, error) {
	v, err := readValue(reader, 0)
	if err != nil {
		return nil, err
	}
	if v.Kind != '*' {
		return nil, fmt.Errorf("expected RESP array command")
	}
	args := make([]string, len(v.Array))
	for i, item := range v.Array {
		if item.Kind != '$' && item.Kind != '+' {
			return nil, fmt.Errorf("command argument must be a string")
		}
		args[i] = item.Text
	}
	return args, nil
}

type Value struct {
	Kind   byte
	Text   string
	Number int64
	Array  []Value
	Raw    []byte
}

func readValue(r *bufio.Reader, depth int) (Value, error) {
	if depth > 64 {
		return Value{}, fmt.Errorf("RESP nesting too deep")
	}
	prefix, err := r.ReadByte()
	if err != nil {
		return Value{}, err
	}
	v := Value{Kind: prefix}
	line, err := readLine(r)
	if err != nil {
		return Value{}, err
	}
	switch prefix {
	case '+', '-', ':', ',', '#', '_', '(', '=':
		v.Text = line
		if prefix == ':' {
			v.Number, err = strconv.ParseInt(line, 10, 64)
			if err != nil {
				return Value{}, err
			}
		}
	case '$', '!':
		n, e := strconv.Atoi(line)
		if e != nil {
			return Value{}, e
		}
		if n == -1 && prefix == '$' {
			return v, nil
		}
		if n < 0 || n > maxBulkLength {
			return Value{}, fmt.Errorf("invalid RESP bulk length")
		}
		b := make([]byte, n+2)
		if _, err = io.ReadFull(r, b); err != nil {
			return Value{}, err
		}
		if b[n] != '\r' || b[n+1] != '\n' {
			return Value{}, fmt.Errorf("malformed RESP bulk string")
		}
		v.Text = string(b[:n])
	case '*', '~', '>':
		n, e := strconv.Atoi(line)
		if e != nil {
			return Value{}, e
		}
		if n == -1 && prefix == '*' {
			return v, nil
		}
		if n < 0 || n > maxArrayLength {
			return Value{}, fmt.Errorf("invalid RESP array length")
		}
		v.Array = make([]Value, n)
		for i := range v.Array {
			v.Array[i], err = readValue(r, depth+1)
			if err != nil {
				return Value{}, err
			}
		}
	case '%':
		n, e := strconv.Atoi(line)
		if e != nil {
			return Value{}, e
		}
		if n < 0 || n > maxArrayLength {
			return Value{}, fmt.Errorf("invalid RESP map length")
		}
		v.Array = make([]Value, n*2)
		for i := range v.Array {
			v.Array[i], err = readValue(r, depth+1)
			if err != nil {
				return Value{}, err
			}
		}
	default:
		return Value{}, fmt.Errorf("unknown RESP type %q", prefix)
	}
	v.Raw = append([]byte{prefix}, []byte(line+"\r\n")...)
	return v, nil
}

func ParseValue(data []byte) (Value, error) {
	return readValue(bufio.NewReader(strings.NewReader(string(data))), 0)
}
