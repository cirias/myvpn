package main

import (
	"github.com/cirias/cvpn/common"
	"net"
)

type Client struct {
	Interface *common.Interface
	IPNet     *net.IPNet
}

func NewClient(conn net.Conn, ipNet *net.IPNet) (client *Client) {
	client = &Client{
		Interface: common.NewInterface(conn),
		IPNet:     ipNet,
	}

	// send IP address to client
	qb := &common.QueuedBuffer{
		Buffer: []byte(client.IPNet.String()),
		N:      len([]byte(client.IPNet.String())),
		//Buffer: append([]byte{1, 2, 3, 4}, make([]byte, 65536)...),
		//N:      65540,
	}
	client.Interface.Input <- qb
	return
}
