package main

import "fmt"

type Downloader struct {
	PeerId string
}

func (d *Downloader) download(metafile TorrentMetaInfo, path string) error {
	// t := Tracker{
	// 	AnnounceUrl: metafile.Announce,
	// 	PeerId:      "00112233445566778899",
	// }

	// peers, err := t.getPeers(metafile)
	// if err != nil {
	// 	return err
	// }

	// // peer := Peer{Addr: peers[0]}

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
