package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"os"
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
}

type TorrentMetaInfo struct {
	Announce  string          `json:"announce"`
	Info      TorrentFileInfo `json:"info"`
	Comment   string          `json:"comment"`
	CreatedBy string          `json:"created by"`
	Encoding  string          `json:"encoding"`
	InfoHash  string
}

// func calculateInfoHash(d []byte) string {
// 	sum := sha1.Sum(d)

// 	return hex.EncodeToString(sum[:])
// }

func decodeMetaInfoFile(path string) (*TorrentMetaInfo, error) {
	fileData, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	reader := bufio.NewReader(bytes.NewReader(fileData))
	dataAsMap, err := decodeBencode(reader)
	if err != nil {
		return nil, err
	}

	mapAsJson, err := json.Marshal(dataAsMap)
	if err != nil {
		return nil, err
	}

	torrentFile := TorrentMetaInfo{}
	err = json.Unmarshal(mapAsJson, &torrentFile)
	if err != nil {
		return nil, err
	}

	// encodedInfo, err := bencode.EncodeBytes(dataAsMap["info"])
	// if err != nil {
	// 	return nil, err
	// }

	// torrentFile.InfoHash = calculateInfoHash(encodedInfo)

	if len(torrentFile.Info.Files) == 0 {
		file := TorrentFileInfoFile{Path: torrentFile.Info.Name, Length: torrentFile.Info.Length}
		torrentFile.Info.Files = []TorrentFileInfoFile{file}
	}

	return &torrentFile, nil
}
