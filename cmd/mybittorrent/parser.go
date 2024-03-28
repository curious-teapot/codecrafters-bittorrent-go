package main

import (
	"bufio"
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
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
	Pieces      []Hash                `json:"omitempty"`
}

type TorrentMetaInfo struct {
	Announce  string          `json:"announce"`
	Info      TorrentFileInfo `json:"info"`
	Comment   string          `json:"comment"`
	CreatedBy string          `json:"created by"`
	Encoding  string          `json:"encoding"`
	InfoHash  Hash
}

type Hash struct {
	Hash []byte
}

func (h *Hash) Hex() string {
	return hex.EncodeToString(h.Hash)
}

func (h *Hash) String() string {
	return string(h.Hash)
}

func calculateInfoHash(d []byte) Hash {
	sum := sha1.Sum(d)

	return Hash{sum[:]}
}

func decodePiecesHash(str string) []Hash {
	hashes := make([]Hash, 0)

	reader := strings.NewReader(str)
	buff := make([]byte, 20)

	for {
		_, err := reader.Read(buff)
		if err == io.EOF {
			break
		}

		hashes = append(hashes, Hash{buff})
	}

	return hashes
}

func unmarshalToStruct(obj any, targetStruct any) error {
	mapAsJson, err := json.Marshal(obj)
	if err != nil {
		return err
	}

	err = json.Unmarshal(mapAsJson, targetStruct)
	if err != nil {
		return err
	}

	return nil
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

	torrentFile := TorrentMetaInfo{}
	err = unmarshalToStruct(decodedData, &torrentFile)
	if err != nil {
		return nil, err
	}

	dataAsMap := decodedData.(map[string]any)

	encodedInfo, err := encodeBencode(dataAsMap["info"])
	if err != nil {
		return nil, err
	}

	info := dataAsMap["info"].(map[string]any)

	torrentFile.InfoHash = calculateInfoHash([]byte(encodedInfo))
	torrentFile.Info.Pieces = decodePiecesHash(info["pieces"].(string))

	if len(torrentFile.Info.Files) == 0 {
		file := TorrentFileInfoFile{Path: torrentFile.Info.Name, Length: torrentFile.Info.Length}
		torrentFile.Info.Files = []TorrentFileInfoFile{file}
	}

	return &torrentFile, nil
}
