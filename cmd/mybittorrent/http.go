package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"net"
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
