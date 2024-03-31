package main

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
)

type Addr struct {
	Ip   net.IP
	Port uint16
}

func (a *Addr) ReadFromBytes(b []byte) error {
	if len(b) != 6 {
		return fmt.Errorf("incorrect address size")
	}

	ipBuff := make([]byte, 4)
	portBuff := make([]byte, 2)

	reader := bytes.NewReader(b)

	reader.Read(ipBuff)
	reader.Read(portBuff)

	a.Ip = net.IP{ipBuff[0], ipBuff[1], ipBuff[2], ipBuff[3]}
	a.Port = binary.BigEndian.Uint16(portBuff)

	return nil
}

func (a *Addr) ReadFromString(str string) error {
	ip, port, ok := strings.Cut(str, ":")
	if !ok {
		return fmt.Errorf("unexpected address format")
	}

	portStr, err := strconv.Atoi(port)
	if err != nil {
		return err
	}

	a.Ip = net.ParseIP(ip)
	a.Port = uint16(portStr)

	return nil
}

func (a *Addr) ToString() string {
	return fmt.Sprintf("%s:%d", a.Ip, a.Port)
}

type GetPeersResponse struct {
	Interval int
	Peers    []Addr
}

func decodeGetPeersRespone(r *io.ReadCloser) (GetPeersResponse, error) {
	resp := GetPeersResponse{}

	decodedResp, err := decodeBencode(bufio.NewReader(*r))
	if err != nil {
		return resp, err
	}

	respMap := decodedResp.(map[string]any)

	resp.Interval = respMap["interval"].(int)
	endcodedPeers := respMap["peers"].(string)

	reader := strings.NewReader(endcodedPeers)

	buff := make([]byte, 6)

	for {
		_, err := reader.Read(buff)
		if err == io.EOF {
			break
		}

		addr := Addr{}
		err = addr.ReadFromBytes(buff)
		if err != nil {
			return resp, err
		}

		resp.Peers = append(resp.Peers, addr)
	}

	return resp, err
}

func getPeers(metafile TorrentMetaInfo) (GetPeersResponse, error) {

	client := http.Client{}
	req, err := createGetPeersRequest(metafile)
	if err != nil {
		return GetPeersResponse{}, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return GetPeersResponse{}, err
	}

	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return GetPeersResponse{}, fmt.Errorf("get peers - erros response")
	}

	responseStruct, err := decodeGetPeersRespone(&resp.Body)

	return responseStruct, err
}

func createGetPeersRequest(metafile TorrentMetaInfo) (*http.Request, error) {
	req, err := http.NewRequest("GET", metafile.Announce, nil)
	if err != nil {
		return nil, err
	}

	query := req.URL.Query()
	query.Add("info_hash", metafile.InfoHash.String())
	query.Add("peer_id", "00112233445566778899")
	query.Add("port", "6881")
	query.Add("uploaded", "0")
	query.Add("downloaded", "0")
	query.Add("left", strconv.Itoa(metafile.Info.Length))
	query.Add("compact", "1")
	req.URL.RawQuery = query.Encode()

	return req, nil
}

func makePeerHandshake(metafile TorrentMetaInfo, addr Addr) (Handshake, net.Conn, error) {
	conn, err := net.Dial("tcp", addr.ToString())
	if err != nil {
		return Handshake{}, conn, err
	}

	handshakeReq := Handshake{
		InfoHash: metafile.InfoHash,
		PeerId:   "00112233445566778899",
	}

	_, err = conn.Write(handshakeReq.toBytes())
	if err != nil {
		return Handshake{}, conn, err
	}

	resp := make([]byte, 68)
	n, err := conn.Read(resp)
	if err != nil {
		return Handshake{}, conn, err
	}

	if n != 68 {
		return Handshake{}, conn, fmt.Errorf("unxepected handshake response length %d", n)
	}

	respHandshake, err := NewHandshakeFromBytes(resp)
	if err != nil {
		return Handshake{}, conn, err
	}

	return respHandshake, conn, nil
}

type Handshake struct {
	InfoHash Hash
	PeerId   string
}

func (h *Handshake) toBytes() []byte {
	buf := make([]byte, 1)
	buf[0] = 19 // length of the protocol
	buf = append(buf, []byte("BitTorrent protocol")...)
	buf = append(buf, make([]byte, 8)...) // eight reserved bytes
	buf = append(buf, h.InfoHash.Hash...)
	buf = append(buf, []byte(h.PeerId)...) // peer id

	return buf
}

func NewHandshakeFromBytes(bytes []byte) (Handshake, error) {
	h := Handshake{}

	if len(bytes) != 68 {
		return h, fmt.Errorf("unxepected handhake length %d", len(bytes))
	}

	h.InfoHash = Hash{Hash: bytes[28:48]}
	h.PeerId = hex.EncodeToString(bytes[48:])

	return h, nil
}

func downloadPiece(metafile TorrentMetaInfo, pieceIndex int) ([]PieceBlock, error) {

	peers, err := getPeers(metafile)
	if err != nil {
		return nil, err
	}

	peer := peers.Peers[0]

	fmt.Println("send Handshake")
	_, peerConn, err := makePeerHandshake(metafile, peer)
	if err != nil {
		return nil, err
	}

	defer peerConn.Close()

	// read bitfield
	msg, err := readPeerMessage(peerConn)
	if err != nil {
		return nil, err
	}

	if msg.MsgId == 5 {
		fmt.Println("bitfield received")
	} else {
		fmt.Printf("undexpected message id %d", msg.MsgId)
	}

	fmt.Println("send interested msg")
	err = writePeerMessage(peerConn, PeerMsg{MsgId: 2})
	if err != nil {
		return nil, err
	}

	msg, err = readPeerMessage(peerConn)
	if err != nil {
		return nil, err
	}

	if msg.MsgId == 1 {
		fmt.Println("unchoke received")
	} else {
		fmt.Printf("undexpected message id %d", msg.MsgId)
	}

	blockSize := 16 * 1024
	blockIndex := 0
	for bytesToRequest := metafile.Info.PieceLength; bytesToRequest > 0; bytesToRequest -= blockSize {

		msgPayload := make([]byte, 4*3)
		binary.BigEndian.PutUint32(msgPayload, uint32(pieceIndex))                         // piece index
		binary.BigEndian.PutUint32(msgPayload[4:], uint32(blockSize*blockIndex))           // block offset
		binary.BigEndian.PutUint32(msgPayload[8:], uint32(min(blockSize, bytesToRequest))) // block length
		blockIndex += 1

		msg := PeerMsg{MsgId: 6, Payload: msgPayload}
		fmt.Printf("send request msg %d \n", blockIndex)

		err = writePeerMessage(peerConn, msg)
		if err != nil {
			return nil, err
		}
	}

	pieceBlocks := make([]PieceBlock, 0)

	for i := 0; i < blockIndex; i++ {
		msg, err = readPeerMessage(peerConn)
		if err != nil {
			return nil, err
		}

		if msg.MsgId == 7 {
			fmt.Println("piece received")
			block, err := msg.PieceBlock()
			if err != nil {
				return nil, err
			}

			pieceBlocks = append(pieceBlocks, block)
		} else {
			fmt.Printf("undexpected message id %d", msg.MsgId)
		}
	}

	return pieceBlocks, nil
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

func readPeerMessage(conn net.Conn) (PeerMsg, error) {
	msgLengthBuff, err := readBytesFromConnection(conn, 4)
	if err != nil {
		return PeerMsg{}, err
	}

	msgLength := int(binary.BigEndian.Uint32(msgLengthBuff))

	msgIdBuff, err := readBytesFromConnection(conn, 1)
	if err != nil {
		return PeerMsg{}, err
	}

	payloadBuff := make([]byte, 0)
	if msgLength > 1 {
		payloadBuff, err = readBytesFromConnection(conn, msgLength-1)
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

func writePeerMessage(conn net.Conn, msc PeerMsg) error {
	msgLen := len(msc.Payload) + 1
	buf := make([]byte, 5)

	binary.BigEndian.PutUint32(buf, uint32(msgLen))
	buf[4] = byte(msc.MsgId)

	if msgLen > 1 {
		buf = append(buf, msc.Payload...)
	}

	_, err := conn.Write(buf)

	return err
}

func readBytesFromConnection(conn net.Conn, n int) ([]byte, error) {
	result := make([]byte, 0)
	needToRead := n
	for {
		buff := make([]byte, needToRead)
		readed, err := conn.Read(buff)
		if err != nil {
			return nil, err
		}

		result = append(result, buff[:readed]...)

		if readed < needToRead {
			needToRead -= readed
		} else {
			break
		}
	}

	return result, nil
}
