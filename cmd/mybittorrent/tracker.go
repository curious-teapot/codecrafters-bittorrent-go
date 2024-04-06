package main

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

type Tracker struct {
	AnnounceUrl string
	PeerId      string
	Interval    int
}

type PeersResponse struct {
	Interval int
	Peers    []Addr
}

func (t *Tracker) getPeers(metafile TorrentMetaInfo) ([]Peer, error) {
	switch {
	case strings.HasPrefix(t.AnnounceUrl, "http"):
		return t.getPeersHttp(metafile)
	case strings.HasPrefix(t.AnnounceUrl, "udp"):
		return t.getPeersUdp(metafile)
	default:
		return nil, fmt.Errorf("undexpected tracker proticol %s", metafile.Announce)
	}
}

func (t *Tracker) getPeersHttp(metafile TorrentMetaInfo) ([]Peer, error) {
	client := http.Client{}
	req, err := createHttpPeersRequest(t.PeerId, metafile)
	if err != nil {
		return nil, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("get peers - erros response")
	}

	responseStruct, err := decodeHttpPeersRespone(&resp.Body)
	if err != nil {
		return nil, err
	}

	t.Interval = responseStruct.Interval

	peers := make([]Peer, len(responseStruct.Peers))
	for i, peerAddr := range responseStruct.Peers {
		peers[i] = Peer{
			Addr:       peerAddr,
			HavePieces: NewPiecesMap(len(metafile.Info.Pieces)),
		}
	}

	return peers, nil
}

func decodeHttpPeersRespone(r *io.ReadCloser) (PeersResponse, error) {
	resp := PeersResponse{}

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

func createHttpPeersRequest(peerId string, metafile TorrentMetaInfo) (*http.Request, error) {
	req, err := http.NewRequest("GET", metafile.Announce, nil)
	if err != nil {
		return nil, err
	}

	query := req.URL.Query()
	query.Add("info_hash", metafile.InfoHash.String())
	query.Add("peer_id", peerId)
	query.Add("port", "6881")
	query.Add("uploaded", "0")
	query.Add("downloaded", "0")
	query.Add("left", strconv.Itoa(metafile.Info.Length))
	query.Add("compact", "1")
	req.URL.RawQuery = query.Encode()

	return req, nil
}

func (t *Tracker) getPeersUdp(metafile TorrentMetaInfo) ([]Peer, error) {
	u, err := url.Parse(t.AnnounceUrl)
	if err != nil {
		return nil, err
	}

	peerPort, err := strconv.Atoi(u.Port())
	if err != nil {
		return nil, err
	}

	a, err := net.LookupIP(u.Hostname())
	if err != nil {
		return nil, err
	}

	raddr := net.UDPAddr{
		IP:   a[0],
		Port: peerPort,
	}

	conn, err := net.DialUDP("udp", nil, &raddr)
	if err != nil {
		return nil, err
	}

	defer conn.Close()

	buf := make([]byte, 16)

	transactionId := 4609668
	binary.BigEndian.PutUint64(buf[0:], 0x41727101980)          // connection_id
	binary.BigEndian.PutUint32(buf[8:], 0)                      // action
	binary.BigEndian.PutUint32(buf[12:], uint32(transactionId)) // transaction_id

	_, err = conn.Write(buf)
	if err != nil {
		return nil, err
	}

	resp, err := readBytes(conn, 16)
	if err != nil {
		return nil, err
	}

	action := binary.BigEndian.Uint32(resp[:4])
	transaction_id := binary.BigEndian.Uint32(resp[4:8])
	connection_id := binary.BigEndian.Uint64(resp[8:])

	fmt.Printf("Action: %d, Transaction: %d, connection: %d \n", action, transaction_id, connection_id)

	buf = make([]byte, 100)

	binary.BigEndian.PutUint64(buf[0:8], connection_id)                  // int64_t 	connection_id 	The connection id acquired from establishing the connection.
	binary.BigEndian.PutUint32(buf[8:12], 1)                             // int32_t 	action 	Action. in this case, 1 for announce. See actions.
	binary.BigEndian.PutUint32(buf[12:16], uint32(transactionId))        // int32_t 	transaction_id 	Randomized by client.
	copyToSlice(buf, metafile.InfoHash.Hash, 16)                         // int8_t[20] 	info_hash 	The info-hash of the torrent you want announce yourself in.
	copyToSlice(buf, []byte(t.PeerId), 36)                               // int8_t[20] 	peer_id 	Your peer id.
	binary.BigEndian.PutUint64(buf[56:64], 0)                            // int64_t 	downloaded 	The number of byte you've downloaded in this session.
	binary.BigEndian.PutUint64(buf[64:72], uint64(metafile.Info.Length)) // int64_t 	left 	The number of bytes you have left to download until you're finished.
	binary.BigEndian.PutUint64(buf[72:80], 0)                            // int64_t 	uploaded 	The number of bytes you have uploaded in this session.
	binary.BigEndian.PutUint32(buf[80:84], 0)                            // int32_t 	event
	binary.BigEndian.PutUint32(buf[84:88], 0)                            // uint32_t 	ip 	Your ip address. Set to 0 if you want the tracker to use the sender of this UDP packet.
	binary.BigEndian.PutUint32(buf[88:92], uint32(transactionId))        // uint32_t 	key 	A unique key that is randomized by the client.
	binary.BigEndian.PutUint32(buf[92:96], 10)                           // int32_t 	num_want 	The maximum number of peers you want in the reply. Use -1 for default.
	binary.BigEndian.PutUint16(buf[96:98], 9999)                         // uint16_t 	port 	The port you're listening on.
	binary.BigEndian.PutUint16(buf[98:100], 0)                           // uint16_t 	extensions

	n, err := conn.Write(buf)
	if err != nil {
		fmt.Printf("N: %d \n", n)
		return nil, err
	}

	buf = make([]byte, 100)
	readed, err := conn.Read(buf)
	if err != nil {
		return nil, err
	}

	action = binary.BigEndian.Uint32(buf[:4])
	transaction_id = binary.BigEndian.Uint32(buf[4:8])
	interval := binary.BigEndian.Uint32(buf[8:12])
	leechers := binary.BigEndian.Uint32(buf[12:16])
	seeders := binary.BigEndian.Uint32(buf[16:20])

	fmt.Printf("Action: %d, Transaction: %d, interval: %d, leechers: %d, seeders: %d \n", action, transaction_id, interval, leechers, seeders)

	peerData := buf[20:readed]

	peers := make([]Peer, 0)

	t.Interval = int(interval)

	for {
		addr := Addr{}
		err = addr.ReadFromBytes(peerData[:6])
		if err != nil {
			return nil, err
		}

		peerData = peerData[6:]

		peer := Peer{
			Addr:       addr,
			HavePieces: NewPiecesMap(len(metafile.Info.Pieces)),
		}

		peers = append(peers, peer)
		if len(peerData) == 0 {
			break
		}
	}

	return peers, nil
}

func copyToSlice(target []byte, source []byte, targetOffset int) {
	for i, b := range source {
		target[i+targetOffset] = b
	}
}
