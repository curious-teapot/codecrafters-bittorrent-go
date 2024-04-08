package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
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

		t := Tracker{
			AnnounceUrl: metaInfo.Announce,
			PeerId:      "00112233445566778899",
		}

		peers, err := t.getPeers(metaInfo)
		if err != nil {
			fmt.Println(err)
			return
		}

		for _, peer := range peers {
			fmt.Printf("%s:%d\n", peer.Addr.Ip, peer.Addr.Port)
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

		peer := Peer{Addr: addr}
		conn, err := peer.Connect()
		if err != nil {
			fmt.Println(err)
			return
		} else {
			peer.Conn = conn
		}

		err = peer.SendHandshake(metaInfo.InfoHash, "00112233445566778899")
		if err != nil {
			fmt.Println(err)
		}

		defer peer.Disconnect()

		fmt.Printf("Peer ID: %s\n", peer.PeerId)

	case "download_piece":
		outputFile := os.Args[3]
		filePath := os.Args[4]
		pieceIndex, _ := strconv.Atoi(os.Args[5])

		metaInfo, err := decodeMetaInfoFile(filePath)
		if err != nil {
			fmt.Println(err)
			return
		}

		t := Tracker{
			AnnounceUrl: metaInfo.Announce,
			PeerId:      "00112233445566778899",
		}

		peers, err := t.getPeers(metaInfo)
		if err != nil {
			fmt.Println(err)
			return
		}

		piece := Piece{
			Index: pieceIndex,
			Hash:  metaInfo.Info.Pieces[pieceIndex],
		}

		d := Downloader{PeerId: "00112233445566778899"}

		for _, peer := range peers {
			piece.Blocks, err = d.downloadPiece(&peer, metaInfo, pieceIndex)
			if err != nil {
				fmt.Println(err)
			} else {
				peer.Conn.Close()
				break
			}
		}

		piece.sortBlocks()

		isValid, _ := piece.checkHash()
		if !isValid {
			fmt.Printf("Piece %d is invalid", piece.Index)
			return
		}

		piece.Index = 0

		err = savePieceToFile(piece, outputFile, metaInfo.Info.PieceLength)
		if err != nil {
			fmt.Println(err)
			return
		}

		fmt.Printf("Piece %d downloaded to %s.\n", pieceIndex, outputFile)

	case "download":
		outputFile := os.Args[3]
		filePath := os.Args[4]

		metaInfo, err := decodeMetaInfoFile(filePath)
		if err != nil {
			fmt.Println(err)
			return
		}

		d := Downloader{PeerId: "00112233445566778899"}

		err = d.Download(metaInfo, outputFile)
		if err != nil {
			fmt.Println(err)
			return
		}

		fmt.Printf("Downloaded %s to %s.\n", filePath, outputFile)

	default:
		fmt.Println("Unknown command: " + command)
		os.Exit(1)

	}
}
