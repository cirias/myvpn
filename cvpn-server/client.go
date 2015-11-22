package main

import (
	"encoding/binary"
	"errors"
	"github.com/cirias/cvpn/common"
	"log"
	"net"
)

type Client struct {
	Cipher    *common.Cipher
	Interface *common.Interface
	IPNet     *net.IPNet
}

func NewClient(conn net.Conn, cipher *common.Cipher, ipNet *net.IPNet) (client *Client) {
	client = &Client{
		Cipher:    cipher,
		Interface: common.NewInterface(conn),
		IPNet:     ipNet,
	}
	return
}

// Decrypt and resamble
func (client *Client) DecryptPacket(plainQb *common.QueuedBuffer) (err error) {
	var qb *common.QueuedBuffer
	// Decrypt IP packet header
	for qb = range client.Interface.Output {
		client.Cipher.Decrypt(plainQb.Buffer[plainQb.N:], qb.Buffer[:qb.N])
		plainQb.N += qb.N
		qb.Return()

		if plainQb.N >= 20 {
			break
		}
	}

	if plainQb.N == 0 {
		return errors.New("client.Interface.Output closed")
	}

	totalLen := int(binary.BigEndian.Uint16(plainQb.Buffer[2:4]))
	log.Printf("totalLen: %v, plainQb.N: %v\n", totalLen, plainQb.N)
	if plainQb.N >= totalLen {
		return
	}

	// Decrypt the rest of the packet
	for qb = range client.Interface.Output {
		client.Cipher.Decrypt(plainQb.Buffer[plainQb.N:], qb.Buffer[:qb.N])
		plainQb.N += qb.N
		qb.Return()

		if plainQb.N >= totalLen {
			break
		}
	}
	return
}
