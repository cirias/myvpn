package main

import (
	"bytes"
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

var ErrInvalidPassword = errors.New("Invalid password")

type Server struct {
	ip        net.IP
	Interface *common.Interface
	ipPool    *common.IPPool
	portPool  *common.PortPool
	clients   map[uint32]*Client
	Cipher    *common.Cipher
}

func NewServer(addr, password string) (server *Server, err error) {
	// Open tun
	tun, err := water.NewTUN("")
	if err != nil {
		return
	}

	if err = common.IfUp(tun.Name(), addr); err != nil {
		return
	}
	defer common.IfDown(tun.Name(), addr)
	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt, os.Kill)

		s := <-c
		glog.Infoln("Got signal:", s)
		if err = common.IfDown(tun.Name(), addr); err != nil {
			glog.Fatalln("IfDown", err)
		}
		os.Exit(0)
	}()

	// IPPool
	ip, ipPool, err := common.NewIPPoolWithCIDR(addr)
	if err != nil {
		return
	}

	// Cipher
	cipher, err := common.NewCipher(password)
	if err != nil {
		return
	}

	server = &Server{
		ip:        ip,
		Interface: common.NewInterface("tun", tun),
		ipPool:    ipPool,
		portPool:  common.NewPortPool(61000, 1000),
		Cipher:    cipher,
	}

	// Initialize clients
	server.clients = make(map[uint32]*Client)
	return
}

func (server *Server) handshake(conn net.Conn) (client *Client, err error) {
	defer func() {
		conn.Close()
	}()

	// Step 1: Read encrypted IV and random key
	buffer := make([]byte, common.IVSize*2+common.KeySize)
	if _, err = io.ReadFull(conn, buffer); err != nil {
		return
	}

	plaintext := make([]byte, common.IVSize+common.KeySize)
	if err = server.Cipher.Decrypt(buffer[:common.IVSize], plaintext, buffer[common.IVSize:]); err != nil {
		return
	}
	if !bytes.Equal(plaintext[:common.IVSize], buffer[:common.IVSize]) {
		err = ErrInvalidPassword
		return
	}

	// New client cipher from server cipher
	cipher, err := common.NewCipherWithKey(plaintext[common.IVSize:])
	if err != nil {
		return
	}
	glog.Infoln("handshake step 1 pass")

	// Step 2: Send REP, IP, IPMask, Port
	ip, err := server.ipPool.GetIP()
	if err != nil {
		// set REP
		plaintext = []byte{0x01}
	}

	port, err := server.portPool.GetPort()
	if err != nil {
		// set REP
		plaintext = []byte{0x02}
	}

	if len(plaintext) != 1 {
		client, err = NewClient(cipher, port, &net.IPNet{
			IP:   ip,
			Mask: server.ipPool.Network.Mask,
		})
		if err != nil {
			return
		}

		plaintext = make([]byte, 1+common.IPSize+common.IPMaskSize+common.PortSize)
		copy(plaintext[1:], client.IPNet.IP)
		copy(plaintext[1+common.IPSize:], client.IPNet.Mask)

		buf := new(bytes.Buffer)
		if err = binary.Write(buf, binary.BigEndian, uint16(port)); err != nil {
			return
		}
		copy(plaintext[1+common.IPSize+common.IPMaskSize:], buf.Bytes())
	}

	ciphertext := make([]byte, common.IVSize+1+common.IPSize+common.IPMaskSize+common.PortSize)
	if err = server.Cipher.Encrypt(ciphertext[:common.IVSize], ciphertext[common.IVSize:], plaintext); err != nil {
		return
	}

	_, err = conn.Write(ciphertext)
	glog.Infoln("handshake step 2 done")
	return
}

func (server *Server) handle(conn net.Conn) {
	client, err := server.handshake(conn)
	if err != nil {
		glog.Errorln("during handshake", err)
		return
	}

	glog.Infoln("client join", client.IPNet.IP)
	server.clients[binary.BigEndian.Uint32(client.IPNet.IP)] = client
	defer func() {
		glog.Infoln("client leave", client.IPNet.IP)
		delete(server.clients, binary.BigEndian.Uint32(client.IPNet.IP))
		server.ipPool.ReturnIP(client.IPNet.IP)
		server.portPool.ReturnPort(client.Port)
	}()

	done := make(chan struct{})
	defer close(done)

	errc := client.Run(done)
	go func(client *Client) {
		var plainQb *common.QueuedBuffer
		var dst net.IP
		var target *Client
		for plainQb = range client.Output {
			// Get distination
			dst = plainQb.Buffer[16:20]
			glog.V(2).Infof("-> [%v]\n", dst)

			switch true {
			case dst.Equal(server.ip):
				server.Interface.Input <- plainQb
			case server.ipPool.Network.Contains(dst):
				target = server.clients[binary.BigEndian.Uint32(dst)]
				if target != nil {
					target.Input <- plainQb
				} else {
					glog.Warningln("drop packet")
				}
			default:
				server.Interface.Input <- plainQb
			}
		}
	}(client)

	err = <-errc
	glog.Errorln("handle conn", err)

	return
}

func (server *Server) rawPacket(rawQb *common.QueuedBuffer) (err error) {
	var qb *common.QueuedBuffer
	var ok bool
	// IP packet header
	for rawQb.N = 0; rawQb.N < 20; {
		qb, ok = <-server.Interface.Output
		if !ok {
			return
		}
		copy(rawQb.Buffer[rawQb.N:], qb.Buffer[:qb.N])
		rawQb.N += qb.N
		qb.Return()
	}

	if rawQb.N == 0 {
		return errors.New("server.Interface.Output closed")
	}

	totalLen := int(binary.BigEndian.Uint16(rawQb.Buffer[2:4]))

	// the rest of the packet
	for rawQb.N < totalLen {
		qb, ok = <-server.Interface.Output
		if !ok {
			return
		}
		copy(rawQb.Buffer[rawQb.N:], qb.Buffer[:qb.N])
		rawQb.N += qb.N
		qb.Return()
	}
	return
}

func (server *Server) routeRoutine() {
	var rawQb *common.QueuedBuffer
	var dst net.IP
	var client *Client
	for {
		rawQb = common.LB.Get()
		err := server.rawPacket(rawQb)
		if err != nil {
			return
		}

		dst = rawQb.Buffer[16:20]
		glog.V(2).Infof("-> [%v]\n", dst)

		client = server.clients[binary.BigEndian.Uint32(dst)]
		if client != nil {
			client.Input <- rawQb
		} else {
			glog.Warningln("drop packet")
		}
	}
}

func (server *Server) Run(laddr string) (err error) {
	// TCP
	listener, err := net.Listen("tcp", laddr)
	if err != nil {
		return
	}
	defer listener.Close()
	glog.Infoln("listening on", laddr)

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				glog.Fatalln("listener", err)
				return
			}

			go server.handle(conn)
		}
	}()

	// UDP
	//udpAddr, err := net.ResolveUDPAddr("udp", laddr)
	//if err != nil {
	//return
	//}
	//conn, err := net.ListenUDP("udp", udpAddr)
	//if err != nil {
	//return
	//}
	//server.conn = conn

	done := make(chan struct{})
	defer close(done)

	errc := server.Interface.Run(done)
	go server.routeRoutine()

	err = <-errc

	return
}
