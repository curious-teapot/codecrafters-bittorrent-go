package main

import (
	"fmt"
	"math"
	"os"
	"sync"
)

type Downloader struct {
	PeerId string
}

func (d *Downloader) Download(metafile TorrentMetaInfo, path string) error {

	t := Tracker{
		AnnounceUrl: metafile.Announce,
		PeerId:      d.PeerId,
	}

	peers, err := t.getPeers(metafile)
	if err != nil {
		return err
	}

	if len(peers) == 0 {
		return fmt.Errorf("no peers to start download")
	}

	pieces := make([]Piece, len(metafile.Info.Pieces))
	for pieceIndex := range pieces {
		pieces[pieceIndex].Index = pieceIndex
		pieces[pieceIndex].Hash = metafile.Info.Pieces[pieceIndex]
	}

	piecesQueue := make(chan Piece, len(pieces))
	fileSaveQueue := make(chan Piece, len(pieces))

	for _, piece := range pieces {
		piecesQueue <- piece
	}

	var wg sync.WaitGroup
	wg.Add(len(pieces))

	for _, peer := range peers {
		peer := peer
		go func() {
			for pieceToDownload := range piecesQueue {
				pieceToDownload.Blocks, err = d.downloadPiece(peer, metafile, pieceToDownload.Index)
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

				fmt.Printf("Piece downloaded %d \n", pieceToDownload.Index)

				wg.Done()
			}
		}()
	}

	fileSaveIsDone := make(chan struct{})

	go func() {
		for pieceToSave := range fileSaveQueue {
			err = savePieceToFile(pieceToSave, path, metafile.Info.PieceLength)
			fmt.Printf("Piece saved %d \n", pieceToSave.Index)
			if err != nil {
				fmt.Println(err)
				return
			}
		}
		close(fileSaveIsDone)
	}()

	wg.Wait()
	close(fileSaveQueue)
	<-fileSaveIsDone

	return nil
}

func (d *Downloader) downloadPiece(peer Peer, metafile TorrentMetaInfo, pieceIndex int) ([]PieceBlock, error) {

	err := peer.Connect()
	if err != nil {
		return nil, err
	}

	err = peer.SendHandshake(metafile.InfoHash, d.PeerId)
	if err != nil {
		return nil, err
	}

	defer peer.Disconnect()

	// var bitfield []byte = nil

	pieceBlocks := make([]PieceBlock, 0)

	bloksTotal := math.Ceil(float64(metafile.Info.PieceLength) / (16 * 1024))

	for {
		msg, err := peer.ReadMessage()
		if err != nil {
			return nil, err
		}

		switch msg.MsgId {
		case int(MsgIdBitfield):
			err = peer.SendIntrested()
			if err != nil {
				return nil, err
			}
		case int(MsgIdUnchoke):

			pieceLength := peer.CalculatePieceLength(metafile.Info.Length, metafile.Info.PieceLength, pieceIndex)
			_, err := peer.SendPieceBlocksRequests(pieceIndex, pieceLength)
			if err != nil {
				return nil, err
			}
		case int(MsgIdPiece):
			block, err := msg.PieceBlock()
			if err != nil {
				return nil, err
			}

			pieceBlocks = append(pieceBlocks, block)
		default:
			return nil, fmt.Errorf("undexpected message id %d", msg.MsgId)
		}

		if len(pieceBlocks) == bloksTotal {
			break
		}
	}

	return pieceBlocks, nil
}

func savePieceToFile(piece Piece, path string, pieceLength int) error {
	// If the file doesn't exist, create it
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}

	defer f.Close()

	for _, block := range piece.Blocks {
		pieceOffset := piece.Index*pieceLength + block.Begin

		f.WriteAt(block.Block, int64(pieceOffset))
	}

	return nil
}
