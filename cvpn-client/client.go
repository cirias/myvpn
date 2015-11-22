package main

import (
	"encoding/binary"
	"errors"
	"github.com/cirias/cvpn/common"
	"github.com/songgao/water"
	"io"
	"log"
	"net"
	//"os"
)

type Client struct {
	serverAddr string
	Conn       *common.Interface
	Tun        *common.Interface
	cipher     *common.Cipher
	buffers    chan *common.QueuedBuffer
	error      chan error
}

func NewClient(addr, password string) (client *Client, err error) {
	cipher, err := common.NewCipher(password)
	if err != nil {
		return
	}

	client = &Client{
		serverAddr: addr,
		cipher:     cipher,
		buffers:    common.NewBufferQueue(common.MAXPACKETSIZE),
		error:      make(chan error),
	}
	return
}

func (client *Client) tun2conn() {
	var qb *common.QueuedBuffer
	var cipherQb *common.QueuedBuffer
	for qb = range client.Tun.Output {
		// Encrypt
		cipherQb = <-client.buffers
		client.cipher.Encrypt(cipherQb.Buffer, qb.Buffer[:qb.N])
		cipherQb.N = qb.N
		qb.Return()

		client.Conn.Input <- cipherQb
	}
}

func (client *Client) conn2tun() {
	var qb *common.QueuedBuffer
	var plainQb *common.QueuedBuffer
	for qb = range client.Conn.Output {
		// Decrypt
		plainQb = <-client.buffers
		client.cipher.Decrypt(plainQb.Buffer, qb.Buffer[:qb.N])
		plainQb.N = qb.N
		qb.Return()

		client.Tun.Input <- plainQb
	}
}

func (client *Client) Run() (err error) {
	// Create tun
	tun, err := water.NewTUN("")
	if err != nil {
		return
	}

	// Create connnection to server
	conn, err := net.Dial("tcp", client.serverAddr)
	if err != nil {
		return
	}
	defer conn.Close()

	// Step 1: Send encrypt IV and encrypted IV
	iv, err := client.cipher.InitEncrypt()
	if err != nil {
		return
	}
	buffer := make([]byte, len(iv)*2)
	copy(buffer, iv)
	client.cipher.Encrypt(buffer[len(iv):], iv)
	n, err := conn.Write(buffer)
	if err != nil {
		return
	}
	log.Printf("step 1: %vbytes writen %v", n, buffer)

	// Step 2: Read decrypt IV, REP, IP from server
	buffer = make([]byte, common.IVLEN+2+4+4)
	_, err = io.ReadFull(conn, buffer)
	if err != nil {
		return
	}

	err = client.cipher.InitDecrypt(buffer[:common.IVLEN])
	if err != nil {
		return
	}

	plaintext := make([]byte, 2+4+4)
	client.cipher.Decrypt(plaintext, buffer[common.IVLEN:])
	log.Printf("Step 2: plaintext:\t%v\n", plaintext)

	rep := binary.BigEndian.Uint16(plaintext[:2])
	switch rep {
	case 0:
	default:
		err = errors.New("Error occured")
		return
	}

	addr := net.IPNet{
		IP:   plaintext[2:6],
		Mask: plaintext[6:10],
	}

	//
	err = common.ConfigureTun(tun, addr.String())
	if err != nil {
		return
	}

	client.Conn = common.NewInterface(conn)
	client.Tun = common.NewInterface(tun)

	go client.tun2conn()
	go client.conn2tun()

	go func() {
		client.error <- client.Tun.Run()
	}()
	go func() {
		client.error <- client.Conn.Run()
	}()

	err = <-client.error
	return
}
