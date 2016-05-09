package protocol

import (
	"errors"
	"net"
)

var ErrNoConnectionAvaliable = errors.New("No connection avaliable")
var ErrConnectionPoolFull = errors.New("Connection pool is full")

type ConnectionPool struct {
	cc chan *Conn
}

func NewConnectionPool(reserveIP net.IP, ipNet *net.IPNet) (p *ConnectionPool) {
	// calculate pool size
	ones, bits := ipNet.Mask.Size()
	size := 1<<uint(bits-ones) - 2

	// Initialize pool
	p = &ConnectionPool{make(chan *Conn, size)}
	for identity := 1; identity <= size; identity++ {
		ip := make(net.IP, 4)
		copy(ip, ipNet.IP.To4())
		for i, index := 1, identity; index != 0; i++ {
			ip[len(ip)-i] = ip[len(ip)-i] | (byte)(0xFF&index)
			index = index >> 8
		}

		if ip.Equal(reserveIP) {
			continue
		}

		p.cc <- &Conn{
			CipherConn: &CipherConn{},
			IPNet:      &net.IPNet{ip, ipNet.Mask},
			pool:       p,
		}
	}
	return
}

func (p *ConnectionPool) Get() (c *Conn, err error) {
	select {
	case c = <-p.cc:
		return
	default:
		err = ErrNoConnectionAvaliable
		return
	}
}

func (p *ConnectionPool) Put(c *Conn) (err error) {
	select {
	case p.cc <- c:
		return
	default:
		err = ErrConnectionPoolFull
		return
	}
}
