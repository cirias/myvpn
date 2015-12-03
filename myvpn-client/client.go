package main

import (
	"crypto/rand"
	"encoding/binary"
	"errors"
	"github.com/cirias/myvpn/common"
	"github.com/golang/glog"
	"github.com/songgao/water"
	"io"
	"net"
	"os"
	"os/signal"
)

type Client struct {
	serverAddr   string
	internalAddr string
	password     string
	UDPHandle    *UDPHandle
	Tun          *common.Interface
	cipher       *common.Cipher
}

func NewClient(addr, password string) (client *Client, err error) {
	client = &Client{
		serverAddr: addr,
		password:   password,
	}
	return
}

func (client *Client) handshake(conn net.Conn) (err error) {
	defer conn.Close()

	cipher, err := common.NewCipher(client.password)
	if err != nil {
		return
	}

	// Step 1: Send encrypted IV and random key
	plaintext := make([]byte, common.IVSize+common.KeySize)
	if _, err = io.ReadFull(rand.Reader, plaintext[common.IVSize:]); err != nil {
		return
	}

	client.cipher, err = common.NewCipherWithKey(plaintext[common.IVSize:])
	if err != nil {
		return
	}

	ciphertext := make([]byte, common.IVSize*2+common.KeySize)
	if err = cipher.Encrypt(plaintext[:common.IVSize], ciphertext[common.IVSize:], plaintext); err != nil {
		return
	}
	copy(ciphertext[:common.IVSize], plaintext[:common.IVSize])

	if _, err = conn.Write(ciphertext); err != nil {
		return
	}
	glog.Infoln("handshake step 1 done")

	// Step 2: Read REP, IP, IPMask, UDPPort from server
	ciphertext = make([]byte, common.IVSize+1+common.IPSize+common.IPMaskSize+common.PortSize)
	if _, err = io.ReadFull(conn, ciphertext); err != nil {
		return
	}

	plaintext = make([]byte, 1+common.IPSize+common.IPMaskSize+common.PortSize)
	if err = cipher.Decrypt(ciphertext[:common.IVSize], plaintext, ciphertext[common.IVSize:]); err != nil {
		return
	}

	// REP
	rep := plaintext[0]
	switch rep {
	case 0x00:
	case 0x01:
		err = common.ErrIPPoolEmpty
		return
	case 0x02:
		err = common.ErrPortPoolEmpty
		return
	default:
		err = errors.New("Error occured")
		return
	}

	// IP and IPMask
	addr := &net.IPNet{
		IP:   plaintext[1:5],
		Mask: plaintext[5:9],
	}
	client.internalAddr = addr.String()

	// UDPPort
	port := int(binary.BigEndian.Uint16(plaintext[9:11]))
	udpAddr, err := net.ResolveUDPAddr("udp", client.serverAddr)
	if err != nil {
		return
	}
	udpAddr.Port = port
	glog.V(2).Infoln("server udp addr", udpAddr)
	client.UDPHandle, err = NewUDPHandle(udpAddr)
	if err != nil {
		return
	}

	glog.Infoln("handshake step 2 done")
	return
}

func (client *Client) tun2conn() {
	var qb *common.QueuedBuffer
	var cipherQb *common.QueuedBuffer
	for qb = range client.Tun.Output {
		// Encrypt
		cipherQb = common.LB.Get()

		if err := client.cipher.Encrypt(cipherQb.Buffer[:common.IVSize], cipherQb.Buffer[common.IVSize:], qb.Buffer[:qb.N]); err != nil {
			qb.Return()
			cipherQb.Return()
			glog.Fatalln("client encrypt", err)
			continue
		}

		cipherQb.N = common.IVSize + qb.N
		qb.Return()

		client.UDPHandle.Input <- cipherQb
	}
}

func (client *Client) conn2tun() {
	var cipherQb *common.QueuedBuffer
	var plainQb *common.QueuedBuffer
	for cipherQb = range client.UDPHandle.Output {
		// Decrypt
		plainQb = common.LB.Get()
		if err := client.cipher.Decrypt(cipherQb.Buffer[:common.IVSize], plainQb.Buffer, cipherQb.Buffer[common.IVSize:cipherQb.N]); err != nil {
			cipherQb.Return()
			plainQb.Return()
			glog.Fatalln("client decrypt", err)
			continue
		}

		plainQb.N = cipherQb.N - common.IVSize
		cipherQb.Return()

		client.Tun.Input <- plainQb
	}
}

func (client *Client) Run() (err error) {
	// Create connnection
	conn, err := net.Dial("tcp", client.serverAddr)
	if err != nil {
		return
	}

	if err := client.handshake(conn); err != nil {
		return err
	}

	// Create tun
	tun, err := water.NewTUN("")
	if err != nil {
		return
	}

	// Get server IP
	tcpAddr, err := net.ResolveTCPAddr("tcp", client.serverAddr)
	if err != nil {
		return
	}

	if err = common.IfUp(tun.Name(), client.internalAddr, tcpAddr.IP.String()); err != nil {
		return
	}
	defer common.IfDown(tun.Name(), client.internalAddr, tcpAddr.IP.String())
	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt, os.Kill)

		s := <-c
		glog.Infoln("Got signal:", s)
		if err = common.IfDown(tun.Name(), client.internalAddr, tcpAddr.IP.String()); err != nil {
			glog.Fatalln("IfDown", err)
		}
		os.Exit(0)
	}()

	//client.Conn = common.NewInterface("conn", conn)
	client.Tun = common.NewInterface("tun", tun)
	glog.Infoln("Tun up as", tun.Name())

	done := make(chan struct{})
	defer close(done)

	tec := client.Tun.Run(done)
	cec := client.UDPHandle.Run(done)
	errc := common.Merge(done, tec, cec)

	go client.tun2conn()
	go client.conn2tun()

	err = <-errc
	return
}
