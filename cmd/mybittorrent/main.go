package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
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

		peer := Peer{Addr: addr}
		err = peer.Connect()
		if err != nil {
			fmt.Println(err)
			return
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

		peer := Peer{Addr: peers[0]}

		piece := Piece{
			Index: pieceIndex,
			Hash:  metaInfo.Info.Pieces[pieceIndex],
		}

		d := Downloader{PeerId: "00112233445566778899"}

		piece.Blocks, err = d.downloadPiece(peer, metaInfo, pieceIndex)
		if err != nil {
			fmt.Println(err)
			return
		}

		piece.sortBlocks()

		isValid, _ := piece.checkHash()
		if !isValid {
			fmt.Printf("Piece %d is invalid", piece.Index)
			return
		}

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

		t := Tracker{
			AnnounceUrl: metaInfo.Announce,
			PeerId:      "00112233445566778899",
		}

		peers, err := t.getPeers(metaInfo)
		if err != nil {
			fmt.Println(err)
			return
		}

		pieces := make([]Piece, len(metaInfo.Info.Pieces))
		for pieceIndex := range pieces {
			pieces[pieceIndex].Index = pieceIndex
			pieces[pieceIndex].Hash = metaInfo.Info.Pieces[pieceIndex]
		}

		d := Downloader{PeerId: "00112233445566778899"}

		piecesQueue := make(chan Piece, len(pieces))
		fileSaveQueue := make(chan Piece, len(pieces))

		for _, piece := range pieces {
			piecesQueue <- piece
		}

		var wg sync.WaitGroup
		wg.Add(len(pieces))

		for _, peerAddr := range peers {
			go func(peer Peer, piecesQueue chan Piece, fileSaveQueue chan Piece) {
				for pieceToDownload := range piecesQueue {
					pieceToDownload.Blocks, err = d.downloadPiece(peer, metaInfo, pieceToDownload.Index)
					if err != nil {
						fmt.Println(err)
						piecesQueue <- pieceToDownload
						return
					}

					pieceToDownload.sortBlocks()

					isValid, _ := pieceToDownload.checkHash()
					if !isValid {
						piecesQueue <- pieceToDownload
						return
					}

					fileSaveQueue <- pieceToDownload

					wg.Done()
				}
			}(Peer{Addr: peerAddr}, piecesQueue, fileSaveQueue)
		}

		go func() {
			for pieceToSave := range fileSaveQueue {
				err = savePieceToFile(pieceToSave, outputFile, metaInfo.Info.PieceLength)
				if err != nil {
					fmt.Println(err)
					return
				}
			}
		}()

		wg.Wait()

		close(fileSaveQueue)

		fmt.Printf("Downloaded %s to %s.\n", filePath, outputFile)

	default:
		fmt.Println("Unknown command: " + command)
		os.Exit(1)

	}
}
