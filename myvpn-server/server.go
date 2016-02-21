package main

import (
	"io"
	"net"
	"sync"
	"time"

	"protocol"
	"tun"

	"github.com/golang/glog"
)

type Server struct {
	rwm     sync.RWMutex
	clients map[string]*Client
	tun     *tun.Tun
	quit    chan struct{}
}

func NewServer(t *tun.Tun, n *net.IPNet) (s *Server) {
	s = &Server{
		clients: make(map[string]*Client),
		tun:     t,
		quit:    make(chan struct{}),
	}

	go func() {
		b := make([]byte, 65535)
		var dst net.IP

		for {
			select {
			case <-s.quit:
				return
			default:
			}
			n, err := t.Read(b)
			if err != nil {
				glog.Errorln("Read from tun", err)
			}

			dst = b[16:20]
			if c := s.clients[dst.String()]; c != nil {
				glog.Infoln("Write to", dst)
				_, err := c.Write(b[:n])
				if err != nil {
					glog.Errorln("Write to", dst, err)
				}
			} else {
				glog.Infoln("Unknown distination", dst)
			}
		}
	}()

	return
}

func (s *Server) Handle(conn *protocol.Conn) {
	c := &Client{
		Conn:      conn,
		timestamp: time.Now(),
		quit:      make(chan struct{}),
	}
	defer c.Close()
	defer glog.Infoln("Client quit", c.IPNet.IP)

	s.rwm.Lock()
	s.clients[c.IPNet.IP.String()] = c
	s.rwm.Unlock()
	defer func() {
		s.rwm.Lock()
		delete(s.clients, c.IPNet.IP.String())
		s.rwm.Unlock()
	}()

	b := make([]byte, 65535)
	var dst net.IP

	for {
		select {
		case <-c.quit:
			return
		default:
		}

		n, err := c.Read(b)
		if err != nil {
			if err == io.EOF {
				return
			}

			glog.Errorln("Read from client", c.IPNet.IP, err)
			continue
		}
		c.timestamp = time.Now()

		dst = b[16:20]
		glog.Infoln("Write to", dst)
		if c := s.clients[dst.String()]; c != nil {
			_, err = c.Write(b[:n])
		} else {
			_, err = s.tun.Write(b[:n])
		}
		if err != nil {
			glog.Errorln("Write to", dst, err)
		}
	}
}

func (s *Server) RecycleClient() {
	var oldest *Client

	s.rwm.RLock()
	for _, c := range s.clients {
		if oldest == nil {
			oldest = c
			continue
		}

		if c.timestamp.Before(oldest.timestamp) {
			oldest = c
		}
	}
	s.rwm.RUnlock()

	if oldest != nil {
		glog.Infoln("Recycle client", oldest.IPNet.IP)
		close(oldest.quit)
	} else {
		glog.Infoln("Can't recycle: empty clients")
	}

	return
}

func (s *Server) Close() {
	close(s.quit)

	s.rwm.RLock()
	for _, c := range s.clients {
		close(c.quit)
	}
	s.rwm.RUnlock()
}
