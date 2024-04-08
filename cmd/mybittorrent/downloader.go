package main

import (
	"errors"
	"fmt"
	"math"
	"os"
	"sync"
	"syscall"
)

type Downloader struct {
	PeerId string
}

var (
	ErrPeerConnection = errors.New("peer connection error")
)

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

	fmt.Printf("Start donwload frorm %d peers\n", len(peers))

	for _, peer := range peers {
		peer := peer
		go func() {
			for pieceToDownload := range piecesQueue {
				pieceToDownload.Blocks, err = d.downloadPiece(&peer, metafile, pieceToDownload.Index)

				if err != nil {
					piecesQueue <- pieceToDownload

					if errors.Is(err, ErrPeerConnection) {
						fmt.Printf("YEET the peer - %s \n", peer.Addr.Ip)
						return // YEET the peer
					} else if errors.Is(err, syscall.EPIPE) {
						peer.Disconnect()
					}
					continue
				}

				pieceToDownload.sortBlocks()

				isValid, _ := pieceToDownload.checkHash()
				if !isValid {
					fmt.Printf("Invalid piece %d hash\n", pieceToDownload.Index)
					piecesQueue <- pieceToDownload
					continue
				}

				fileSaveQueue <- pieceToDownload

				wg.Done()
			}
		}()
	}

	defer func() {
		for _, peer := range peers {
			peer.Disconnect()
		}
	}()

	fileSaveIsDone := make(chan struct{})

	go func() {
		dowloadedPieces := 0
		for pieceToSave := range fileSaveQueue {
			err = savePieceToFile(pieceToSave, path, metafile.Info.PieceLength)
			dowloadedPieces += 1
			fmt.Printf("[%d/%d] Piece saved %d \n", dowloadedPieces, len(pieces), pieceToSave.Index)
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

func (d *Downloader) downloadPiece(peer *Peer, metafile TorrentMetaInfo, pieceIndex int) ([]PieceBlock, error) {
	if peer.Conn == nil {
		conn, err := peer.Connect()
		if err != nil {
			return nil, fmt.Errorf("%w: %s", ErrPeerConnection, err)
		}

		peer.Conn = conn

		err = peer.SendHandshake(metafile.InfoHash, d.PeerId)
		if err != nil {
			peer.Disconnect()
			return nil, fmt.Errorf("%w: handshake error %s", ErrPeerConnection, err)
		}

		msg, err := peer.ReadMessage()
		if err != nil || msg.MsgId != int(MsgIdBitfield) {
			peer.Disconnect()
			return nil, fmt.Errorf("%w: bitfields message error %s", ErrPeerConnection, err)
		}

		peer.HavePieces.updateFromBitfield(msg.Payload)
	}

	if !peer.HavePieces.hasPiece(pieceIndex) {
		return nil, fmt.Errorf("%s peer dont have piece #%d", peer.Addr.Ip, pieceIndex)
	}

	err := peer.SendIntrested()
	if err != nil {
		return nil, err
	}

	pieceLength := peer.CalculatePieceLength(metafile.Info.Length, metafile.Info.PieceLength, pieceIndex)
	pieceBloksCount := calculateBlocksCount(pieceLength)

	pieceBlocks := make([]PieceBlock, 0)

	pieceRequested := false

	for {
		msg, err := peer.ReadMessage()
		if err != nil {
			return nil, fmt.Errorf("read message error: %s", err)
		}

		switch msg.MsgId {
		case int(MsgIdKeepAlive):
			fmt.Println("Keep alive received")

		case int(MsgIdChoke):
			peer.Disconnect()
			return nil, fmt.Errorf("choke")

		case int(MsgIdHave):
			peerHavePieceIndex := int(msg.Payload[0])
			peer.HavePieces.setPieceStatus(peerHavePieceIndex, true)

		case int(MsgIdUnchoke):
			if !pieceRequested {
				_, err := peer.SendPieceBlocksRequests(pieceIndex, pieceLength)
				if err != nil {
					return nil, fmt.Errorf("send piece blocks request error: %s", err)
				}
				pieceRequested = true
			}

		case int(MsgIdPiece):
			block, err := msg.PieceBlock()
			if err != nil {
				return nil, fmt.Errorf("piece block decode error: %s", err)
			}

			pieceBlocks = append(pieceBlocks, block)

		default:
			return nil, fmt.Errorf("undexpected message id %d", msg.MsgId)
		}

		if len(pieceBlocks) == pieceBloksCount {
			break
		}
	}

	return pieceBlocks, nil
}

func calculateBlocksCount(pieceLength int) int {
	blockSize := 16 * 1024
	return int(math.Ceil(float64(pieceLength) / float64(blockSize)))
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
