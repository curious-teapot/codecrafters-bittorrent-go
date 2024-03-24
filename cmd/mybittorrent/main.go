package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"unicode"
	// bencode "github.com/jackpal/bencode-go" // Available if you need it!
)

func readUntilByte(reader *bufio.Reader, untilByte byte) (string, error) {
	buff, err := reader.ReadBytes(untilByte)
	if err != nil {
		return "", err
	}

	return string(buff[:len(buff)-1]), nil
}

func readByteSlice(reader *bufio.Reader, n int) ([]byte, error) {
	bytes, err := reader.Peek(n)
	if err != nil {
		return nil, err
	}

	_, err = reader.Discard(n)
	if err != nil {
		return nil, err
	}

	return bytes, nil
}

func decodeString(reader *bufio.Reader) (string, error) {

	sizeStr, err := readUntilByte(reader, ':')
	if err != nil {
		return "", err
	}

	length, err := strconv.Atoi(sizeStr)
	if err != nil {
		return "", err
	}

	bytes, err := readByteSlice(reader, length)
	if err != nil {
		return "", err
	}

	return string(bytes), nil
}

func decodeInt(reader *bufio.Reader) (int, error) {
	_, err := reader.Discard(1)
	if err != nil {
		return 0, err
	}

	intStr, err := readUntilByte(reader, 'e')
	if err != nil {
		return 0, err
	}

	val, err := strconv.Atoi(string(intStr))
	if err != nil {
		return 0, err
	}

	return val, nil
}

func decodeList(reader *bufio.Reader) ([]interface{}, error) {
	_, err := reader.Discard(1)
	if err != nil {
		return nil, err
	}

	list := make([]interface{}, 0)
	for {
		nextByte, err := reader.Peek(1)
		if err != nil {
			return nil, err
		}

		if nextByte[0] == 'e' {
			break
		}

		val, err := decodeBencode(reader)
		if err != nil {
			return nil, err
		}

		list = append(list, val)
	}

	_, err = reader.Discard(1)
	if err != nil {
		return nil, err
	}

	return list, nil
}

func decodeDictionary(reader *bufio.Reader) (map[string]interface{}, error) {
	_, err := reader.Discard(1)
	if err != nil {
		return nil, err
	}

	m := make(map[string]interface{})

	for {
		nextByte, err := reader.Peek(1)
		if err != nil {
			return nil, err
		}

		if nextByte[0] == 'e' {
			break
		}

		key, err := decodeString(reader)
		if err != nil {
			return nil, err
		}

		val, err := decodeBencode(reader)
		if err != nil {
			return nil, err
		}

		m[key] = val
	}

	return m, nil
}

func decodeBencode(reader *bufio.Reader) (interface{}, error) {
	for {
		b, err := reader.Peek(1)
		if err != nil {
			break
		}

		c := rune(b[0])

		switch {
		case unicode.IsDigit(c):
			return decodeString(reader)

		case c == 'i':
			return decodeInt(reader)

		case c == 'l':
			return decodeList(reader)

		case c == 'd':
			return decodeDictionary(reader)

		default:
			return "", fmt.Errorf("only strings are supported at the moment")

		}

	}

	return "", fmt.Errorf("only strings are supported at the moment")
}

func main() {
	command := os.Args[1]

	if command == "decode" {
		bencodedValue := os.Args[2]

		decoded, err := decodeBencode(bufio.NewReader(strings.NewReader(bencodedValue)))
		if err != nil {
			fmt.Println(err)
			return
		}

		jsonOutput, _ := json.Marshal(decoded)
		fmt.Println(string(jsonOutput))
	} else {
		fmt.Println("Unknown command: " + command)
		os.Exit(1)
	}
}
