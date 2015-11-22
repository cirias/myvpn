package main

import (
	"bytes"
	//"fmt"
	"github.com/cirias/cvpn/common"
	"github.com/songgao/water"
	//"github.com/songgao/water/waterutil"
	"io"
	"log"
	"net"
	//"os"
	"encoding/binary"
	"errors"
)

var ErrInvalidPassword = errors.New("Invalid password")

type Server struct {
	ip        net.IP
	Interface *common.Interface
	ipPool    *common.IPPool
	clients   map[uint32]*Client
	Cipher    *common.Cipher
	buffers   chan *common.QueuedBuffer
}

func NewServer(addr, password string) (server *Server, err error) {
	// Open tun
	tun, err := water.NewTUN("")
	if err != nil {
		return
	}
	common.ConfigureTun(tun, addr)

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
		Interface: common.NewInterface(tun),
		ipPool:    ipPool,
		Cipher:    cipher,
		buffers:   common.NewBufferQueue(common.MAXPACKETSIZE),
	}

	// Initialize clients
	server.clients = make(map[uint32]*Client)
	return
}

func (server *Server) handle(conn net.Conn) {
	defer func() {
		conn.Close()
		// TODO error handle
		//if err := recover(); err != nil {
		//switch err {
		//case ErrInvalidPassword:
		//case common.ErrIPPoolFull:
		//default:
		//}
		//}
	}()
	// New client cipher from server cipher
	cipher := server.Cipher.Copy()

	// Step 1: Read decrypt IV and check password
	buffer := make([]byte, common.IVLEN*2)
	_, err := io.ReadFull(conn, buffer)
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
	log.Printf("Step 1: pass %x\n", buffer)

	// Step 2: Send encrypt IV, REP, IP
	iv, err := server.Cipher.InitEncrypt()
	if err != nil {
		return
	}
	buffer = make([]byte, len(iv)+2+4+4)
	copy(buffer, iv)

	ip, err := server.ipPool.GetIP()
	if err != nil {
		return
	}

	client := NewClient(conn, cipher, &net.IPNet{
		IP:   ip,
		Mask: server.ipPool.Network.Mask,
	})

	plaintext = make([]byte, len(iv)+2+4+4)
	copy(plaintext[len(iv)+2:], client.IPNet.IP)
	copy(plaintext[len(iv)+2+4:], client.IPNet.Mask)
	log.Printf("Step 2: plaintext:\t%x\n", plaintext)
	server.Cipher.Encrypt(buffer[len(iv):], plaintext[len(iv):])

	log.Printf("Step 2: buffer:\t%x\n", buffer)
	_, err = conn.Write(buffer)
	if err != nil {
		return
	}

	log.Printf("client join %v", client.IPNet.IP)
	server.clients[binary.BigEndian.Uint32(client.IPNet.IP)] = client
	defer func() {
		log.Printf("client leave %v", client.IPNet.IP)
		delete(server.clients, binary.BigEndian.Uint32(client.IPNet.IP))
	}()

	go func() {
		var plainQb *common.QueuedBuffer
		var cipherQb *common.QueuedBuffer
		var dst net.IP
		for {
			plainQb = <-server.buffers
			err := client.DecryptPacket(plainQb)
			if err != nil {
				return
			}

			// Get distination
			dst = plainQb.Buffer[16:20]
			log.Printf("dst: %s", dst)

			switch true {
			case dst.Equal(server.ip):
				server.Interface.Input <- plainQb
			case server.ipPool.Network.Contains(dst):
				cipherQb = <-server.buffers
				server.Cipher.Encrypt(cipherQb.Buffer, plainQb.Buffer[:plainQb.N])
				cipherQb.N = plainQb.N
				plainQb.Return()

				server.clients[binary.BigEndian.Uint32(dst)].Interface.Input <- cipherQb
			default:
				server.Interface.Input <- plainQb
			}
		}
	}()

	err = client.Interface.Run()
	if err != nil {
		log.Println(err)
		return
	}

	return
}

func (server *Server) rawPacket(rawQb *common.QueuedBuffer) (err error) {
	var qb *common.QueuedBuffer
	var ok bool
	// Decrypt IP packet header
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
	log.Printf("totalLen: %v, rawQb.N: %v\n", totalLen, rawQb.N)

	// Decrypt the rest of the packet
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
	var cipherQb *common.QueuedBuffer
	var dst net.IP
	var client *Client
	for {
		rawQb = <-server.buffers
		err := server.rawPacket(rawQb)
		if err != nil {
			return
		}

		dst = rawQb.Buffer[16:20]
		log.Printf("dst: %s\n", dst)

		client = server.clients[binary.BigEndian.Uint32(dst)]
		if client != nil {
			cipherQb = <-server.buffers
			server.Cipher.Encrypt(cipherQb.Buffer, rawQb.Buffer[:rawQb.N])
			cipherQb.N = rawQb.N
			rawQb.Return()

			client.Interface.Input <- cipherQb
		} else {
			log.Printf("Drop packet, dst: [%s]\n", dst)
		}
	}
}

func (server *Server) Run(laddr string) (err error) {
	go server.routeRoutine()

	// TCP
	listener, err := net.Listen("tcp", laddr)
	if err != nil {
		return
	}
	defer listener.Close()

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				log.Fatalln(err)
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

	err = server.Interface.Run()

	return
}
