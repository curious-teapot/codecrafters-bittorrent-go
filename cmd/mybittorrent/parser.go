package main

import (
	"bufio"
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

type TorrentFileInfoFile struct {
	Length int    `json:"length"`
	Path   string `json:"path"`
}

type TorrentFileInfo struct {
	Files       []TorrentFileInfoFile `json:"files"`
	Length      int                   `json:"length"`
	Name        string                `json:"name"`
	PieceLength int                   `json:"piece length"`
	Pieces      []string              `json:"omitempty"`
}

type TorrentMetaInfo struct {
	Announce  string          `json:"announce"`
	Info      TorrentFileInfo `json:"info"`
	Comment   string          `json:"comment"`
	CreatedBy string          `json:"created by"`
	Encoding  string          `json:"encoding"`
	InfoHash  string
}

func calculateInfoHash(d []byte) string {
	sum := sha1.Sum(d)

	return hex.EncodeToString(sum[:])
}

func decodePiecesHash(str string) []string {
	hashes := make([]string, 0)

	reader := strings.NewReader(str)
	buff := make([]byte, 20)

	for {
		fmt.Print(1)
		_, err := reader.Read(buff)
		if err == io.EOF {
			break
		}

		hashes = append(hashes, hex.EncodeToString(buff))
	}

	return hashes
}

func decodeMetaInfoFile(path string) (*TorrentMetaInfo, error) {
	fileData, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	reader := bufio.NewReader(bytes.NewReader(fileData))
	decodedData, err := decodeBencode(reader)
	if err != nil {
		return nil, err
	}

	mapAsJson, err := json.Marshal(decodedData)
	if err != nil {
		return nil, err
	}

	torrentFile := TorrentMetaInfo{}
	err = json.Unmarshal(mapAsJson, &torrentFile)
	if err != nil {
		return nil, err
	}

	dataAsMap := decodedData.(map[string]interface{})

	encodedInfo, err := encodeBencode(dataAsMap["info"])
	if err != nil {
		return nil, err
	}

	info := dataAsMap["info"].(map[string]interface{})

	torrentFile.InfoHash = calculateInfoHash([]byte(encodedInfo))
	torrentFile.Info.Pieces = decodePiecesHash(info["pieces"].(string))

	if len(torrentFile.Info.Files) == 0 {
		file := TorrentFileInfoFile{Path: torrentFile.Info.Name, Length: torrentFile.Info.Length}
		torrentFile.Info.Files = []TorrentFileInfoFile{file}
	}

	return &torrentFile, nil
}
