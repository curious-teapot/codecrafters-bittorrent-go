package main

import (
	"bufio"
	"fmt"
	"sort"
	"strconv"
	"unicode"
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

func decodeList(reader *bufio.Reader) ([]any, error) {
	_, err := reader.Discard(1)
	if err != nil {
		return nil, err
	}

	list := make([]any, 0)
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

func decodeDictionary(reader *bufio.Reader) (map[string]any, error) {
	_, err := reader.Discard(1)
	if err != nil {
		return nil, err
	}

	m := make(map[string]any)

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

func decodeBencode(reader *bufio.Reader) (any, error) {
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
			return "", fmt.Errorf("only strings are supported at the moment - %v", c)

		}

	}

	return "", fmt.Errorf("only strings are supported at the moment")
}

func encodeBencode(val any) (string, error) {
	switch val := val.(type) {
	case int:
		return fmt.Sprintf("i%de", val), nil

	case string:
		return fmt.Sprintf("%d:%s", len(val), val), nil

	case []any:
		strBuff := ""
		for _, sliceItem := range val {
			encodedItem, err := encodeBencode(sliceItem)
			if err != nil {
				return "", err
			}
			strBuff += encodedItem
		}

		return fmt.Sprintf("l%se", strBuff), nil

	case map[string]any:
		mapKeys := make([]string, 0, len(val))
		for k := range val {
			mapKeys = append(mapKeys, k)
		}

		sort.Strings(mapKeys)

		strBuff := ""
		for _, mapItemKey := range mapKeys {
			encodedKey, err := encodeBencode(mapItemKey)
			if err != nil {
				return "", err
			}
			strBuff += encodedKey

			encodeItem, err := encodeBencode(val[mapItemKey])
			if err != nil {
				return "", err
			}
			strBuff += encodeItem
		}

		return fmt.Sprintf("d%se", strBuff), nil

	default:
		fmt.Printf("%+v type: %T", val, val)

		return "", fmt.Errorf("unsupported value for encode")
	}
}
