package main

import (
	//"fmt"
	"encoding/binary"
	"errors"
	"github.com/cirias/myvpn/common"
	"github.com/golang/glog"
	"github.com/songgao/water"
	"github.com/songgao/water/waterutil"
	"io"
	//"log"
	"net"
	"os"
	"os/signal"
	//"os/exec"
)

type Client struct {
	serverAddr   string
	internalAddr string
	UDPHandle    *UDPHandle
	Tun          *common.Interface
	cipher       *common.Cipher
}

func NewClient(addr, password string) (client *Client, err error) {
	cipher, err := common.NewCipher(password)
	if err != nil {
		return
	}

	client = &Client{
		serverAddr: addr,
		cipher:     cipher,
	}
	return
}

func (client *Client) handshake(conn net.Conn) (err error) {
	defer conn.Close()

	// Step 1: Send encrypt IV and encrypted IV
	iv := make([]byte, common.IVLEN)
	if err = client.cipher.InitEncrypt(iv); err != nil {
		return
	}

	buffer := make([]byte, len(iv)*2)
	copy(buffer, iv)
	client.cipher.Encrypt(buffer[len(iv):], iv)

	_, err = conn.Write(buffer)
	if err != nil {
		return
	}
	glog.Infoln("handshake step 1 done")

	// Step 2: Read decrypt IV, REP, IP, IPMask, UDPPort from server
	buffer = make([]byte, common.IVLEN+1+4+4+2)
	_, err = io.ReadFull(conn, buffer)
	if err != nil {
		return
	}

	err = client.cipher.InitDecrypt(buffer[:common.IVLEN])
	if err != nil {
		return
	}

	plaintext := make([]byte, 1+4+4+2)
	client.cipher.Decrypt(plaintext, buffer[common.IVLEN:])

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
		// Check is IPv4
		if !waterutil.IsIPv4(qb.Buffer[:qb.N]) {
			qb.Return()
			continue
		}

		// Encrypt
		cipherQb = common.LB.Get()

		if err := client.cipher.InitEncrypt(cipherQb.Buffer[:common.IVLEN]); err != nil {
			qb.Return()
			cipherQb.Return()
			glog.Errorln("InitEncrypt", err)
			continue
		}

		client.cipher.Encrypt(cipherQb.Buffer[common.IVLEN:], qb.Buffer[:qb.N])
		cipherQb.N = common.IVLEN + qb.N
		qb.Return()

		client.UDPHandle.Input <- cipherQb
	}
}

func (client *Client) conn2tun() {
	var qb *common.QueuedBuffer
	var plainQb *common.QueuedBuffer
	for qb = range client.UDPHandle.Output {
		if err := client.cipher.InitDecrypt(qb.Buffer[:common.IVLEN]); err != nil {
			qb.Return()
			glog.Errorln("InitDecrypt", err)
			//errc <- err
			continue
		}

		// Decrypt
		plainQb = common.LB.Get()
		client.cipher.Decrypt(plainQb.Buffer, qb.Buffer[common.IVLEN:qb.N])
		plainQb.N = qb.N - common.IVLEN
		qb.Return()

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
