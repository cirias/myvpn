package main

import (
	"io"
	"net"
	"sync"

	"protocol"
	"tun"

	"github.com/golang/glog"
)

type Server struct {
	rwm     sync.RWMutex
	clients map[string]*Client
	tun     *tun.Interface
	quit    chan struct{}
}

func NewServer(t *tun.Interface, n *net.IPNet) (s *Server) {
	s = &Server{
		clients: make(map[string]*Client),
		tun:     t,
		quit:    make(chan struct{}),
	}

	go func() {
		b := make([]byte, 65535)
		dst := &net.IPAddr{}

		for {
			select {
			case <-s.quit:
				return
			default:
			}

			n, err := t.ReadIPPacket(b)
			if err != nil {
				if err == io.EOF {
					return
				}
				glog.Errorln("fail to read from tun", err)
				continue
			}
			glog.V(3).Infoln("recieve IP packet from tun", b[:n])

			dst.IP = b[16:20]
			if c := s.clients[dst.String()]; c != nil {
				glog.V(1).Infoln("write to", dst)
				_, err := c.Write(b[:n])
				if err != nil {
					glog.Errorln("fail to write to", dst, err)
				}
				glog.V(3).Infoln("send IP packet to connection", b[:n])
			} else {
				glog.Warningln("Unknown distination", dst)
			}
		}
	}()

	return
}

func (s *Server) Handle(conn protocol.Conn) {
	c := &Client{
		Conn: conn,
		quit: make(chan struct{}),
	}
	defer c.Close()
	defer glog.Infoln("client quit", c.RemoteIPAddr())

	s.rwm.Lock()
	s.clients[c.RemoteIPAddr().String()] = c
	s.rwm.Unlock()
	defer func() {
		s.rwm.Lock()
		delete(s.clients, c.RemoteIPAddr().String())
		s.rwm.Unlock()
	}()

	b := make([]byte, 65535)

	for {
		select {
		case <-c.quit:
			return
		default:
		}

		n, err := c.ReadIPPacket(b)
		if err != nil {
			if err == io.EOF {
				return
			}
			glog.Errorln("fail to read from client", c.RemoteIPAddr(), err)
			continue
		}
		glog.V(3).Infoln("recieve IP packet from connection", b[:n])

		if _, err = s.tun.Write(b[:n]); err != nil {
			glog.Errorln("fail to write to tun", err)
		}
		glog.V(3).Infoln("send IP packet to tun", b[:n])
	}
}

func (s *Server) Close() {
	glog.Infoln("server closing")
	close(s.quit)

	s.rwm.RLock()
	for _, c := range s.clients {
		close(c.quit)
	}
	s.rwm.RUnlock()
}
