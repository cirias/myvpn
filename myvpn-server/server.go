package main

import (
	"bytes"
	"github.com/golang/glog"
	//"fmt"
	"github.com/cirias/myvpn/common"
	"github.com/songgao/water"
	//"github.com/songgao/water/waterutil"
	"io"
	//"log"
	"encoding/binary"
	"errors"
	"net"
	//"os"
	//"os/exec"
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

	// New client cipher from server cipher
	cipher := server.Cipher.Copy()

	// Step 1: Read decrypt IV and check password
	buffer := make([]byte, common.IVLEN*2)
	_, err = io.ReadFull(conn, buffer)
	if err != nil {
		return
	}

	err = cipher.InitDecrypt(buffer[:common.IVLEN])
	if err != nil {
		return
	}

	plaintext := make([]byte, common.IVLEN)
	cipher.Decrypt(plaintext, buffer[common.IVLEN:])
	if !bytes.Equal(plaintext, buffer[:common.IVLEN]) {
		err = ErrInvalidPassword
		return
	}
	glog.Infoln("handshake step 1 pass")

	// Step 2: Send encrypt IV, REP, IP
	iv := make([]byte, common.IVLEN)
	if err = cipher.InitEncrypt(iv); err != nil {
		return
	}

	buffer = make([]byte, len(iv)+1+4+4+2)
	copy(buffer, iv)

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

		plaintext = make([]byte, 1+4+4+2)
		copy(plaintext[1:], client.IPNet.IP)
		copy(plaintext[1+4:], client.IPNet.Mask)

		buf := new(bytes.Buffer)
		if err = binary.Write(buf, binary.BigEndian, uint16(port)); err != nil {
			return
		}
		copy(plaintext[1+4+4:], buf.Bytes())
	}

	cipher.Encrypt(buffer[len(iv):], plaintext)

	_, err = conn.Write(buffer)
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
			glog.V(2).Infoln("dst:", dst)

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
		glog.V(2).Infoln("dst:", dst)

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
