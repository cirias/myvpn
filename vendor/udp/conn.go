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
		return
	case s := <-c.input:
		copy(b, s)
		return len(s), nil
	}
}

func (c *serverConn) Write(b []byte) (n int, err error) {
	glog.Infoln("Write:", c.raddr, b)
	return c.WriteToUDP(b, c.raddr)
}

func (c *serverConn) Close() error {
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
				break
			default:
			}

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
	glog.Infoln("Accept:", "waiting")
	select {
	case conn = <-ln.new:
	case err = <-ln.newErr:
	}
	if err != nil {
		return
	}

	glog.Infoln("Accept:", conn.RemoteAddr())
	ln.mutex.Lock()
	ln.conns[conn.RemoteAddr().String()] = conn.(*serverConn)
	ln.mutex.Unlock()
	return
}

func (ln *Listener) Close() (err error) {
	ln.done <- struct{}{}

	return ln.udpConn.Close()
}

func (ln *Listener) Addr() net.Addr {
	return ln.udpConn.LocalAddr()
}
