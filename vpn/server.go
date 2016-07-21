package vpn

import (
	"bytes"
	"encoding/binary"
	"net"
	"sync"
	"time"

	"github.com/golang/glog"
)

type Server struct {
	rwm     sync.RWMutex
	clients map[string]*clientHandle
	ipPool  IPAddrPool
	ifce    InOuter
	quit    chan struct{}
}

func NewServer(ifce InOuter, ipnet string) (s *Server, err error) {
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

	go func() {
		dst := &net.IPAddr{}

		for {
			select {
			case <-s.quit:
				return
			default:
			}

			b := <-ifce.Out()
			glog.V(1).Infoln("recieve IP packet from tun", b)

			dst.IP = b[16:20]
			if c := s.clients[dst.String()]; c != nil {
				glog.V(1).Infoln("write to", dst)
				select {
				case c.In() <- b:
				default:
				}
			} else {
				glog.Warningln("Unknown distination", dst)
			}
		}
	}()

	go func() {
		var err error
		for {
			select {
			case <-s.quit:
				break
			case err = <-ifce.InErr():
			case err = <-ifce.OutErr():
			}
			glog.Errorln("interface error", err)
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

func (s *Server) Handle(conn InOuter) error {
	c := &clientHandle{
		InOuter: conn,
		quit:    make(chan struct{}),
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

	w := &bytes.Buffer{}
	glog.V(1).Infoln("res", res)
	if err = binary.Write(w, binary.BigEndian, res); err != nil {
		return err
	}
	conn.In() <- w.Bytes()

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

	for {
		var b []byte
		t := time.NewTimer(0)
		for {
			t.Reset(10 * time.Second)
		CONN_OUT_LOOP:
			for {
				select {
				case <-c.quit:
					return nil
				case b = <-c.Out():
					glog.V(1).Infoln("b = <-c.Out()")
					break CONN_OUT_LOOP
				case <-t.C:
					glog.V(1).Infoln("c.Out() timeout")
					t.Reset(time.Second)
				}
			}

			select {
			case <-c.quit:
				return nil
			case s.ifce.In() <- b:
				glog.V(1).Infoln("b -> ifce.In()")
				glog.V(1).Infoln("send to tun")
			}
		}
	}
}

func (s *Server) Close() {
	glog.Infoln("server closing")
	s.quit <- struct{}{}
	s.quit <- struct{}{}

	s.rwm.RLock()
	for _, c := range s.clients {
		close(c.quit)
	}
	s.rwm.RUnlock()
	glog.Infoln("server closed")
}

type clientHandle struct {
	InOuter
	ipAddr *net.IPAddr
	quit   chan struct{}
}
