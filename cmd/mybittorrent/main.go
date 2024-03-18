package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"unicode"
	// bencode "github.com/jackpal/bencode-go" // Available if you need it!
)

func decodeString(str string) (string, int, error) {
	firstColonIndex := strings.IndexByte(str, ':')

	if firstColonIndex == -1 {
		return "", 0, errors.New("invalid string format - missing ':'")
	}

	lengthStr := str[:firstColonIndex]

	length, err := strconv.Atoi(lengthStr)
	if err != nil {
		return "", 0, err
	}

	strEnd := firstColonIndex + 1 + length

	return str[firstColonIndex+1 : strEnd], strEnd - 1, nil
}

func decodeInt(str string) (int, int, error) {
	firstEndIndex := strings.IndexByte(str, 'e')

	if firstEndIndex == -1 {
		return 0, 0, errors.New("invalid int format - missing 'e'")
	}

	val, err := strconv.Atoi(str[1:firstEndIndex])
	if err != nil {
		return 0, 0, err
	}

	return val, firstEndIndex, nil
}

func decodeList(str string) ([]interface{}, int, error) {
	list := make([]interface{}, 0)

	listContent := str[1 : len(str)-1]

	cursor := 0

	for cursor < len(listContent)-1 {
		val, end, err := decodeBencode(listContent[cursor:])

		if err != nil {
			return list, 0, err
		}

		cursor += end + 1
		list = append(list, val)
	}

	return list, cursor, nil
}

// Example:
// - 5:hello -> hello
// - 10:hello12345 -> hello12345
func decodeBencode(bencodedString string) (interface{}, int, error) {
	firstChar := rune(bencodedString[0])

	switch {
	case unicode.IsDigit(firstChar):
		return decodeString(bencodedString)

	case firstChar == 'i':
		return decodeInt(bencodedString)

	case firstChar == 'l':
		return decodeList(bencodedString)

	default:
		return "", 0, fmt.Errorf("only strings are supported at the moment")

	}
}

func main() {
	command := os.Args[1]

	if command == "decode" {
		bencodedValue := os.Args[2]

		decoded, _, err := decodeBencode(bencodedValue)
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
