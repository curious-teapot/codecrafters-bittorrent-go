package main

import (
	"fmt"
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

	for _, peerAddr := range peers {
		go func(peer Peer, piecesQueue chan Piece, fileSaveQueue chan Piece) {
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

				wg.Done()
			}
		}(Peer{Addr: peerAddr}, piecesQueue, fileSaveQueue)
	}

	go func() {
		for pieceToSave := range fileSaveQueue {
			err = savePieceToFile(pieceToSave, path, metafile.Info.PieceLength)
			if err != nil {
				fmt.Println(err)
				return
			}
		}
	}()

	wg.Wait()

	close(fileSaveQueue)

	return nil
}

func (d *Downloader) downloadPiece(peer Peer, metafile TorrentMetaInfo, pieceIndex int) ([]PieceBlock, error) {

	err := peer.Connect()
	if err != nil {
		return nil, err
	}

	defer peer.Disconnect()

	err = peer.SendHandshake(metafile.InfoHash, d.PeerId)
	if err != nil {
		return nil, err
	}

	// read bitfield
	msg, err := peer.ReadMessage()
	if err != nil {
		return nil, err
	}

	if msg.MsgId != int(MsgIdBitfield) {
		return nil, fmt.Errorf("undexpected message id %d", msg.MsgId)
	}

	err = peer.SendIntrested()
	if err != nil {
		return nil, err
	}

	// read unchoke
	msg, err = peer.ReadMessage()
	if err != nil {
		return nil, err
	}

	if msg.MsgId != int(MsgIdUnchoke) {
		return nil, fmt.Errorf("undexpected message id %d", msg.MsgId)
	}

	pieceLength := peer.CalculatePieceLength(metafile.Info.Length, metafile.Info.PieceLength, pieceIndex)
	blocksRequested, err := peer.SendPieceBlocksRequests(pieceIndex, pieceLength)
	if err != nil {
		return nil, err
	}

	return peer.ReceivePieceBlocks(blocksRequested)
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
