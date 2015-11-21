package main

import (
	//"fmt"
	"github.com/cirias/cvpn/common"
	"github.com/songgao/water"
	//"github.com/songgao/water/waterutil"
	"log"
	//"io"
	"net"
	//"os"
	//"errors"
	"encoding/binary"
)

type Server struct {
	ip        net.IP
	Interface *common.Interface
	ipPool    *common.IPPool
	clients   map[uint32]*Client
}

func NewServer(addr string) (server *Server, err error) {
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

	server = &Server{
		ip:        ip,
		Interface: common.NewInterface(tun),
		ipPool:    ipPool,
	}

	// Initialize clients
	server.clients = make(map[uint32]*Client)
	return
}

func (server *Server) handle(conn net.Conn) {
	ip, err := server.ipPool.GetIP()
	if err != nil {
		log.Println(err)
		return
	}

	client := NewClient(conn, &net.IPNet{
		IP:   ip,
		Mask: server.ipPool.Network.Mask,
	})

	log.Printf("client join %v", client.IPNet.IP)
	server.clients[binary.BigEndian.Uint32(client.IPNet.IP)] = client
	defer func() {
		log.Printf("client leave %v", client.IPNet.IP)
		delete(server.clients, binary.BigEndian.Uint32(client.IPNet.IP))
	}()

	go func() {
		var qb *common.QueuedBuffer
		var dst net.IP
		for qb = range client.Interface.Output {
			dst = qb.Buffer[16:20]
			log.Printf("routing dst: %s", dst)
			switch true {
			case dst.Equal(server.ip):
				server.Interface.Input <- qb
			case server.ipPool.Network.Contains(dst):
				server.clients[binary.BigEndian.Uint32(dst)].Interface.Input <- qb
			default:
				server.Interface.Input <- qb
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

func (server *Server) routeRoutine() {
	var qb *common.QueuedBuffer
	var dst net.IP
	var client *Client
	for qb = range server.Interface.Output {
		dst = qb.Buffer[16:20]
		log.Printf("server n: %v, dst: %s", qb.N, dst)
		client = server.clients[binary.BigEndian.Uint32(dst)]
		if client != nil {
			client.Interface.Input <- qb
		} else {
			log.Printf("Drop packet, dst: [%v]\n", dst)
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
			defer conn.Close()

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
