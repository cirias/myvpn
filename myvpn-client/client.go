package main

import (
	"crypto/rand"
	"encoding/binary"
	"github.com/cirias/myvpn/common"
	"github.com/golang/glog"
	"github.com/songgao/water"
	"io"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
)

type Client struct {
	serverAddr   string
	internalAddr string
	password     string
	UDPHandle    *UDPHandle
	Tun          *common.Interface
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

	udpcipher, err := common.NewCipherWithKey(plaintext[common.IVSize:])
	if err != nil {
		return
	}

	ciphertext := make([]byte, common.IVSize*2+common.KeySize)
	if err = cipher.Encrypt(plaintext[:common.IVSize], ciphertext[common.IVSize:], plaintext); err != nil {
		return
	}
	copy(ciphertext[:common.IVSize], plaintext[:common.IVSize])
	glog.V(3).Infof("write %vbytes to [%v]: %x\n", len(plaintext), conn.RemoteAddr(), plaintext)

	if _, err = conn.Write(ciphertext); err != nil {
		return
	}
	glog.Infoln("handshake step 1 done")

	// Step 2: Read REP, IP, IPMask, UDPPort from server
	ciphertext = make([]byte, common.IVSize+common.IPSize+common.IPMaskSize+common.PortSize)
	if _, err = io.ReadFull(conn, ciphertext); err != nil {
		return
	}

	plaintext = make([]byte, common.IPSize+common.IPMaskSize+common.PortSize)
	if err = cipher.Decrypt(ciphertext[:common.IVSize], plaintext, ciphertext[common.IVSize:]); err != nil {
		return
	}
	glog.V(3).Infof("read %vbytes from [%v]: %x\n", len(plaintext), conn.RemoteAddr(), plaintext)

	// IP and IPMask
	addr := &net.IPNet{
		IP:   plaintext[:common.IPSize],
		Mask: plaintext[common.IPSize : common.IPSize+common.IPMaskSize],
	}
	client.internalAddr = addr.String()

	// UDPPort
	port := int(binary.BigEndian.Uint16(plaintext[common.IPSize+common.IPMaskSize : common.IPSize+common.IPMaskSize+common.PortSize]))
	udpAddr, err := net.ResolveUDPAddr("udp", client.serverAddr)
	if err != nil {
		return
	}
	udpAddr.Port = port
	glog.V(2).Infoln("server udp addr", udpAddr)
	client.UDPHandle, err = NewUDPHandle(udpAddr, udpcipher)
	if err != nil {
		return
	}

	glog.Infoln("handshake step 2 done")
	return
}

func (client *Client) tun2conn() {
	var qb *common.QueuedBuffer
	for qb = range client.Tun.Output {
		glog.V(3).Infof("tun2conn %vbytes: %x\n", qb.N, qb.Buffer[:qb.N])
		client.UDPHandle.Input <- qb
	}
}

func (client *Client) conn2tun() {
	var qb *common.QueuedBuffer
	for qb = range client.UDPHandle.Output {
		glog.V(3).Infof("conn2tun %vbytes: %x\n", qb.N, qb.Buffer[:qb.N])
		client.Tun.Input <- qb
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
		signal.Notify(c, os.Interrupt, os.Kill, syscall.SIGTERM)

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

HandleError:
	for err = range errc {
		switch {
		case strings.Contains(err.Error(), "tun: invalid argument"):
			glog.Errorln(err)
		case err == ErrHeartbeatTimeout:
			break HandleError
		default:
			break HandleError
		}
	}

	return
}
