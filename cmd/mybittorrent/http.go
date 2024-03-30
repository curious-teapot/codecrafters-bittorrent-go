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

func downloadPiece(metafile TorrentMetaInfo, pieceIndex int, outputFile string) error {

	peers, err := getPeers(metafile)
	if err != nil {
		fmt.Println(err)
		return nil
	}

	peer := peers.Peers[1]

	_, peerConn, err := makePeerHandshake(metafile, peer)
	if err != nil {
		return err
	}

	defer peerConn.Close()

	// read bitfield
	msg, err := readPeerMessage(peerConn)
	if err != nil {
		return err
	}

	if msg.MsgId == 5 {
		fmt.Println("bitfield")
	}

	return nil
}

type PeerMsg struct {
	MsgId   int
	Payload []byte
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

	payloadBuff, err := readBytesFromConnection(conn, msgLength)
	if err != nil {
		return PeerMsg{}, err
	}

	msg := PeerMsg{
		MsgId:   int(binary.BigEndian.Uint32(msgIdBuff)),
		Payload: payloadBuff,
	}

	return msg, nil
}

func readBytesFromConnection(conn net.Conn, n int) ([]byte, error) {
	buff := make([]byte, n)
	_, err := conn.Read(buff)
	if err != nil {
		return nil, err
	}

	return buff, nil
}