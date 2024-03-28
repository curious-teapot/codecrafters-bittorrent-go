package main

import (
	"bufio"
	"bytes"
	"encoding/binary"
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

func getPeers(metafile *TorrentMetaInfo) (GetPeersResponse, error) {

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

func createGetPeersRequest(metafile *TorrentMetaInfo) (*http.Request, error) {
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
