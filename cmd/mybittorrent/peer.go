package main

import (
	"encoding/binary"
	"fmt"
	"net"
)

type peerMsgId int

const (
	MsgIdChoke         peerMsgId = 0
	MsgIdUnchoke       peerMsgId = 1
	MsgIdInterested    peerMsgId = 2
	MsgIdNotInterested peerMsgId = 3
	MsgIdHave          peerMsgId = 4
	MsgIdBitfield      peerMsgId = 5
	MsgIdRequest       peerMsgId = 6
	MsgIdPiece         peerMsgId = 7
	MsgIdCancel        peerMsgId = 8
)

type Peer struct {
	Addr   Addr
	Conn   net.Conn
	PeerId string
}

type PeerMsg struct {
	MsgId   int
	Payload []byte
}

type PieceBlock struct {
	Index int
	Begin int
	Block []byte
}

func (p *Peer) Connect() error {
	conn, err := net.Dial("tcp", p.Addr.ToString())
	if err != nil {
		return err
	}

	p.Conn = conn

	return nil
}

func (p *Peer) Disconnect() error {
	if p.Conn != nil {
		return p.Conn.Close()
	}

	return nil
}

func (p *Peer) SendHandshake(infoHash Hash, peerId string) error {
	handshakeReq := Handshake{
		InfoHash: infoHash,
		PeerId:   peerId,
	}

	_, err := p.Conn.Write(handshakeReq.toBytes())
	if err != nil {
		return err
	}

	resp, err := readBytesFromConnection(p.Conn, 68)
	if err != nil {
		return err
	}

	respHandshake, err := NewHandshakeFromBytes(resp)
	if err != nil {
		return err
	}

	p.PeerId = respHandshake.PeerId

	return nil
}

func (p *Peer) SendIntrested() error {
	return p.WriteMessage(PeerMsg{MsgId: int(MsgIdInterested)})
}

func (p *Peer) CalculatePieceLength(fileLength int, pieceLength int, pieceIndex int) int {
	pieceOffset := pieceIndex * pieceLength
	left := fileLength - pieceOffset

	return min(left, pieceLength)
}

func (p *Peer) SendPieceBlocksRequests(pieceIndex int, pieceLength int) (int, error) {
	blockSize := 16 * 1024
	blocksRequested := 0

	for bytesToRequest := pieceLength; bytesToRequest > 0; bytesToRequest -= blockSize {

		msgPayload := make([]byte, 4*3)
		binary.BigEndian.PutUint32(msgPayload, uint32(pieceIndex))                         // piece index
		binary.BigEndian.PutUint32(msgPayload[4:], uint32(blockSize*blocksRequested))      // block offset
		binary.BigEndian.PutUint32(msgPayload[8:], uint32(min(blockSize, bytesToRequest))) // block length
		blocksRequested += 1

		msg := PeerMsg{MsgId: int(MsgIdRequest), Payload: msgPayload}

		err := p.WriteMessage(msg)
		if err != nil {
			return blocksRequested, err
		}
	}

	return blocksRequested, nil
}

func (p *Peer) ReceivePieceBlocks(blocksExpected int) ([]PieceBlock, error) {
	pieceBlocks := make([]PieceBlock, 0)

	for i := 0; i < blocksExpected; i++ {
		msg, err := p.ReadMessage()
		if err != nil {
			return nil, err
		}

		if msg.MsgId == int(MsgIdPiece) {
			block, err := msg.PieceBlock()
			if err != nil {
				return nil, err
			}

			pieceBlocks = append(pieceBlocks, block)
		} else {
			return nil, fmt.Errorf("undexpected message id %d", msg.MsgId)
		}
	}

	return pieceBlocks, nil
}

func (p *Peer) ReadMessage() (PeerMsg, error) {
	msgLengthBuff, err := readBytesFromConnection(p.Conn, 4)
	if err != nil {
		return PeerMsg{}, err
	}

	msgLength := int(binary.BigEndian.Uint32(msgLengthBuff))

	msgIdBuff, err := readBytesFromConnection(p.Conn, 1)
	if err != nil {
		return PeerMsg{}, err
	}

	payloadBuff := make([]byte, 0)
	if msgLength > 1 {
		payloadBuff, err = readBytesFromConnection(p.Conn, msgLength-1)
		if err != nil {
			return PeerMsg{}, err
		}
	}

	msg := PeerMsg{
		MsgId:   int(msgIdBuff[0]),
		Payload: payloadBuff,
	}

	return msg, nil
}

func (p *Peer) WriteMessage(msc PeerMsg) error {
	msgLen := len(msc.Payload) + 1
	buf := make([]byte, 5)

	binary.BigEndian.PutUint32(buf, uint32(msgLen))
	buf[4] = byte(msc.MsgId)

	if msgLen > 1 {
		buf = append(buf, msc.Payload...)
	}

	_, err := p.Conn.Write(buf)

	return err

}

func (msg PeerMsg) PieceBlock() (PieceBlock, error) {
	block := PieceBlock{}
	if msg.MsgId != 7 {
		return block, fmt.Errorf("wrong msg-id for piece block - %d", msg.MsgId)
	}

	block.Index = int(binary.BigEndian.Uint32(msg.Payload[:4]))
	block.Begin = int(binary.BigEndian.Uint32(msg.Payload[4:8]))
	block.Block = msg.Payload[8:]

	return block, nil
}
