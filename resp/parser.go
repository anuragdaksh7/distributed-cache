package resp

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
)

func readLine(r *bufio.Reader) (string, error) {
	line, err := r.ReadString('\n')
	if err != nil {
		return "", err
	}

	return strings.TrimSuffix(line, "\r\n"), nil
}

func Parse(reader *bufio.Reader) ([]string, error) {
	header, err := readLine(reader)
	if err != nil {
		return nil, err
	}

	if len(header) == 0 {
		return nil, fmt.Errorf("empty request")
	}

	if header[0] != '*' {
		return nil, fmt.Errorf("expected array")
	}

	count, err := strconv.Atoi(header[1:])
	if err != nil {
		return nil, err
	}

	args := make([]string, 0, count)

	for i := 0; i < count; i++ {
		lengthLine, err := readLine(reader)
		if err != nil {
			return nil, err
		}

		if lengthLine[0] != '$' {
			return nil, fmt.Errorf("expected bulk string")
		}

		length, err := strconv.Atoi(lengthLine[1:])
		if err != nil {
			return nil, err
		}

		buf := make([]byte, length)

		_, err = io.ReadFull(reader, buf)
		if err != nil {
			return nil, err
		}

		if _, err := reader.ReadByte(); err != nil {
			return nil, err
		}
		if _, err := reader.ReadByte(); err != nil {
			return nil, err
		}

		args = append(args, string(buf))
	}

	return args, nil
}
