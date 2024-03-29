package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

func main() {
	command := os.Args[1]

	switch command {
	case "decode":
		bencodedValue := os.Args[2]

		decoded, err := decodeBencode(bufio.NewReader(strings.NewReader(bencodedValue)))
		if err != nil {
			fmt.Println(err)
			return
		}

		jsonOutput, _ := json.Marshal(decoded)
		fmt.Println(string(jsonOutput))

	case "info":
		filePath := os.Args[2]

		metaInfo, err := decodeMetaInfoFile(filePath)
		if err != nil {
			fmt.Println(err)
			return
		}

		fmt.Printf("Tracker URL: %s\n", metaInfo.Announce)
		fmt.Printf("Length: %d\n", metaInfo.Info.Length)
		fmt.Printf("Info Hash: %s\n", metaInfo.InfoHash.Hex())
		fmt.Printf("Piece Length: %d\n", metaInfo.Info.PieceLength)
		fmt.Println("Piece Hashes:")
		for _, pieceHash := range metaInfo.Info.Pieces {
			fmt.Println(pieceHash.Hex())
		}

	case "peers":
		filePath := os.Args[2]

		metaInfo, err := decodeMetaInfoFile(filePath)
		if err != nil {
			fmt.Println(err)
			return
		}

		peers, err := getPeers(metaInfo)
		if err != nil {
			fmt.Println(err)
			return
		}

		for _, peer := range peers.Peers {
			fmt.Printf("%s:%d\n", peer.Ip, peer.Port)
		}

	case "handshake":
		if len(os.Args) < 4 {
			fmt.Println("Please provide file and peer id")
			return
		}

		filePath := os.Args[2]
		peerAddr := os.Args[3]

		metaInfo, err := decodeMetaInfoFile(filePath)
		if err != nil {
			fmt.Println(err)
			return
		}

		addr := Addr{}
		err = addr.ReadFromString(peerAddr)
		if err != nil {
			fmt.Println(err)
			return
		}

		handshake, err := makePeerHandshake(*metaInfo, addr)
		if err != nil {
			fmt.Println(err)
		}

		fmt.Printf("Peer ID: %s\n", handshake.PeerId)

	default:
		fmt.Println("Unknown command: " + command)
		os.Exit(1)

	}
}
