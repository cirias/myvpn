package main

import (
	"github.com/cirias/cvpn/common"
	"github.com/songgao/water"
	//"io"
	"log"
	"net"
	//"os"
)

type Client struct {
	serverAddr string
	Conn       *common.Interface
	Tun        *common.Interface
	error      chan error
}

func NewClient(addr string) (client *Client, err error) {
	client = &Client{
		serverAddr: addr,
		error:      make(chan error),
	}
	return
}

func (client *Client) tun2conn() {
	for {
		client.Conn.Input <- (<-client.Tun.Output)
	}
}

func (client *Client) conn2tun() {
	for {
		client.Tun.Input <- (<-client.Conn.Output)
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

	// Get ip from server
	buffer := make([]byte, 1522)
	n, err := conn.Read(buffer)
	if err != nil {
		return
	}
	log.Printf("n: %v data: %x\n", n, buffer[:n])

	err = common.ConfigureTun(tun, string(buffer[:n]))
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
