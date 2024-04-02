package main

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
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

func (t *Tracker) getPeers(metafile TorrentMetaInfo) ([]Addr, error) {

	client := http.Client{}
	req, err := createGetPeersRequest(t.PeerId, metafile)
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

	responseStruct, err := decodePeersRespone(&resp.Body)
	if err != nil {
		return nil, err
	}

	t.Interval = responseStruct.Interval

	return responseStruct.Peers, nil
}

func decodePeersRespone(r *io.ReadCloser) (PeersResponse, error) {
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

func createGetPeersRequest(peerId string, metafile TorrentMetaInfo) (*http.Request, error) {
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
