package vpn

import (
	"encoding/binary"
	"io"
	"net"
	"sync"
	"time"

	"github.com/golang/glog"
)

type Server struct {
	rwm     sync.RWMutex
	clients map[string]*clientHandle
	ipPool  IPAddrPool
	ifce    io.ReadWriter
	quit    chan struct{}
}

func NewServer(ifce io.ReadWriter, ipnet string) (s *Server, err error) {
	ip, ipNet, err := net.ParseCIDR(ipnet)
	if err != nil {
		return
	}

	s = &Server{
		clients: make(map[string]*clientHandle),
		ifce:    ifce,
		ipPool:  NewIPAddrPool(ip, ipNet),
		quit:    make(chan struct{}),
	}

	/*
	 *   readCh := make(chan []byte)
	 *   go func() {
	 *     b := make([]byte, 65535)
	 *     for {
	 *       n, err := ifce.Read(b)
	 *       if err != nil {
	 *
	 *       }
	 *     }
	 *   }()
	 */

	go func() {
		b := make([]byte, 65535)
		dst := &net.IPAddr{}

		for {
			select {
			case <-s.quit:
				return
			default:
			}

			n, err := ifce.Read(b)
			if err != nil {
				if err == io.EOF {
					return
				}
				glog.Errorln("fail to read from tun", err)
				continue
			}
			glog.V(1).Infoln("recieve IP packet from tun", b[:n])

			dst.IP = b[16:20]
			if c := s.clients[dst.String()]; c != nil {
				glog.V(1).Infoln("write to", dst)
				_, err := c.Write(b[:n])
				if err != nil {
					glog.Errorln("fail to write to", dst, err)
					continue
				}
				glog.V(1).Infoln("send IP packet to connection", b[:n])
			} else {
				glog.Warningln("Unknown distination", dst)
			}
		}
	}()

	return
}

const (
	statusOk byte = iota
	statusNoIPAvaliable
)

type response struct {
	Status byte
	IP     [4]byte
	IPMask [4]byte
}

func (s *Server) Handle(conn io.ReadWriter) error {
	c := &clientHandle{
		ReadWriter: conn,
		quit:       make(chan struct{}),
	}

	res := &response{}
	// allocate ip address
	ip, err := s.ipPool.Get()
	defer func() {
		if ip != nil {
			s.ipPool.Put(ip)
		}
	}()
	switch err {
	case ErrNoIPAvaliable:
		res.Status = statusNoIPAvaliable
	case nil:
		res.Status = statusOk
		copy(res.IP[:], ip.IP.To4())
		copy(res.IPMask[:], ip.Mask)
	default:
	}

	// wait until socket is ready
	for i := 0; i < 3; i++ {
		if err = binary.Write(c, binary.BigEndian, res); err != nil {
			time.Sleep(time.Millisecond)
			continue
		}
		break
	}
	if err != nil {
		glog.Errorln("fail to write response to client", err)
		return err
	}

	if res.Status != statusOk {
		return ErrUnknownStatus
	}

	c.ipAddr = &net.IPAddr{}
	c.ipAddr.IP = make(net.IP, len(ip.IP))
	copy(c.ipAddr.IP, ip.IP)
	defer glog.Infoln("client quit", c.ipAddr)

	glog.Infoln("client join", c.ipAddr.IP)
	s.rwm.Lock()
	s.clients[c.ipAddr.String()] = c
	s.rwm.Unlock()
	defer func() {
		s.rwm.Lock()
		delete(s.clients, c.ipAddr.String())
		s.rwm.Unlock()
	}()

	b := make([]byte, 65535)

	for {
		select {
		case <-c.quit:
			return nil
		default:
		}

		n, err := c.Read(b)
		if err != nil {
			if err == io.EOF {
				return nil
			}
			glog.Errorln("fail to read from client", c.ipAddr, err)
			continue
		}
		glog.V(1).Infoln("recieve IP packet from connection", b[:n])

		if _, err = s.ifce.Write(b[:n]); err != nil {
			glog.Errorln("fail to write to tun", err)
		}
		glog.V(1).Infoln("send IP packet to tun", b[:n])
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
	glog.Infoln("server closed")
}

type clientHandle struct {
	io.ReadWriter
	ipAddr *net.IPAddr
	quit   chan struct{}
}
