package udp

import (
	"github.com/golang/glog"
	"net"
	"sync"
)

type clientConn struct {
	*net.UDPConn
}

func Dial(network, address string) (conn net.Conn, err error) {
	raddr, err := net.ResolveUDPAddr(network, address)
	if err != nil {
		return
	}
	c, err := net.DialUDP(network, nil, raddr)
	conn = &clientConn{c}
	return
}

type serverConn struct {
	*net.UDPConn
	raddr *net.UDPAddr
	input chan []byte
	error chan error
	ln    *Listener
}

func (c *serverConn) RemoteAddr() net.Addr {
	return c.raddr
}

func (c *serverConn) Read(b []byte) (n int, err error) {
	select {
	case err = <-c.error:
		glog.V(1).Infoln("udp serverConn read", err)
		return
	case s := <-c.input:
		glog.V(3).Infoln("udp serverConn read", s)
		copy(b, s)
		return len(s), nil
	}
}

func (c *serverConn) Write(b []byte) (n int, err error) {
	glog.V(3).Infoln("udp serverConn write", c.raddr, b)
	return c.WriteToUDP(b, c.raddr)
}

func (c *serverConn) Close() error {
	glog.V(2).Infoln("udp serverConn close")
	c.ln.mutex.Lock()
	delete(c.ln.conns, c.RemoteAddr().String())
	c.ln.mutex.Unlock()
	return nil
}

type Listener struct {
	udpConn *net.UDPConn
	conns   map[string]*serverConn
	mutex   sync.Mutex
	done    chan struct{}
	new     chan *serverConn
	newErr  chan error
}

func Listen(network, address string) (ln *Listener, err error) {
	ln = &Listener{
		conns:  make(map[string]*serverConn),
		done:   make(chan struct{}),
		new:    make(chan *serverConn),
		newErr: make(chan error),
	}

	laddr, err := net.ResolveUDPAddr(network, address)
	if err != nil {
		return
	}
	ln.udpConn, err = net.ListenUDP(network, laddr)
	if err != nil {
		return
	}

	var conn *serverConn
	go func() {
		for {
			select {
			case <-ln.done:
				return
			default:
			}

			// allocate memory every time may be a bad ideal
			b := make([]byte, 65535)
			n, raddr, err := ln.udpConn.ReadFromUDP(b)

			if raddr == nil {
				ln.newErr <- err
				continue
			}

			ln.mutex.Lock()
			conn = ln.conns[raddr.String()]
			ln.mutex.Unlock()

			if conn == nil {
				conn = &serverConn{ln.udpConn, raddr, make(chan []byte), make(chan error), ln}
				ln.new <- conn
			}

			if err != nil {
				conn.error <- err
			} else {
				conn.input <- b[:n]
			}
		}
	}()
	return
}

func (ln *Listener) Accept() (conn net.Conn, err error) {
	glog.V(2).Infoln("udp listener waiting to accept")
	select {
	case conn = <-ln.new:
	case err = <-ln.newErr:
	}
	if err != nil {
		glog.V(1).Infoln("udp listener fail to accept", err)
		return
	}

	ln.mutex.Lock()
	ln.conns[conn.RemoteAddr().String()] = conn.(*serverConn)
	ln.mutex.Unlock()
	glog.V(2).Infoln("udp listener accepted", conn.RemoteAddr())
	return
}

func (ln *Listener) Close() (err error) {
	glog.V(2).Infoln("udp listener closing")
	ln.done <- struct{}{}

	return ln.udpConn.Close()
}

func (ln *Listener) Addr() net.Addr {
	return ln.udpConn.LocalAddr()
}
