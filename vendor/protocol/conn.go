package protocol

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"io"
	"net"

	"udp"

	"github.com/golang/glog"
)

const (
	TCP byte = iota
	UDP
)

const (
	StatusOK byte = iota
	StatusUnknowErr
	StatusInvalidSecret
	StatusInvalidProto
	StatusNoConnectionAvaliable
)

var ErrUnknowErr = errors.New("Unknow error")
var ErrInvalidSecret = errors.New("Invalid secret")
var ErrInvalidProto = errors.New("Invalid proto")

type request struct {
	Secret [KeySize]byte
	Proto  byte
	Key    [KeySize]byte
}

type response struct {
	Status byte
	IP     [4]byte
	IPMask [4]byte
}

type Conn struct {
	*CipherConn
	IPNet *net.IPNet
	pool  *ConnectionPool
}

func (c *Conn) IP() (ip net.IP) {
	switch c.RemoteAddr().(type) {
	case *net.TCPAddr:
		ip = c.RemoteAddr().(*net.TCPAddr).IP
	case *net.UDPAddr:
		ip = c.RemoteAddr().(*net.UDPAddr).IP
	}
	return
}

func (c *Conn) Close() error {
	if c.pool != nil {
		c.pool.Put(c)
	}
	return c.CipherConn.Close()
}

func Dial(network, secret, remoteAddr string) (conn *Conn, err error) {
	var proto byte
	var c net.Conn
	switch network {
	case "tcp":
		proto = TCP
		c, err = net.Dial(network, remoteAddr)
	case "udp":
		proto = UDP
		c, err = udp.Dial(network, remoteAddr)
	default:
		err = ErrInvalidProto
	}
	if err != nil {
		glog.V(1).Infoln("fail to dial to server", err, network, remoteAddr)
		return
	}

	cipher, err := NewCipher([]byte(secret))
	if err != nil {
		return
	}

	key := make([]byte, KeySize)
	if _, err = io.ReadFull(rand.Reader, key); err != nil {
		return
	}

	req := request{
		Secret: sha256.Sum256([]byte(secret)),
		Proto:  proto,
	}
	copy(req.Key[:], key)

	cc := &CipherConn{
		Conn:   c,
		Cipher: cipher,
	}
	if err = binary.Write(cc, binary.BigEndian, req); err != nil {
		glog.V(1).Infoln("fail to write to cipherconn", err)
		return
	}
	glog.V(3).Infoln("write to cipherconn", req)

	// Recieve
	res := response{}
	if err = binary.Read(cc, binary.BigEndian, &res); err != nil {
		glog.V(1).Infoln("fail to read from cipherconn", err)
		return
	}
	glog.V(3).Infoln("read from cipherconn", res)

	switch res.Status {
	case StatusOK:
	case StatusUnknowErr:
		fallthrough
	case StatusInvalidSecret:
		err = ErrInvalidSecret
		return
	case StatusInvalidProto:
		err = ErrInvalidProto
		return
	case StatusNoConnectionAvaliable:
		err = ErrNoConnectionAvaliable
		return
	default:
		err = ErrUnknowErr
		return
	}

	// Initialize conn
	conn = &Conn{
		CipherConn: &CipherConn{},
		IPNet:      &net.IPNet{res.IP[:], res.IPMask[:]},
	}
	conn.Conn = c
	conn.Cipher, err = NewCipher(key)
	return
}

type Listener struct {
	Listener net.Listener
	Cipher   *Cipher
	CP       *ConnectionPool
	Secret   [KeySize]byte
	Proto    byte
}

func Listen(network, secret, localAddr string, internalIP net.IP, ipNet *net.IPNet) (listener *Listener, err error) {
	listener = &Listener{}
	var proto byte
	switch network {
	case "tcp":
		proto = TCP
		listener.Listener, err = net.Listen(network, localAddr)
	case "udp":
		proto = UDP
		listener.Listener, err = udp.Listen(network, localAddr)
	default:
		err = ErrInvalidProto
	}
	if err != nil {
		return
	}

	cipher, err := NewCipher([]byte(secret))
	if err != nil {
		return
	}
	listener.Cipher = cipher

	listener.CP = NewConnectionPool(internalIP, ipNet)
	listener.Secret = sha256.Sum256([]byte(secret))
	listener.Proto = proto

	return
}

type exception struct {
	err error
	cc  *CipherConn
}

func (ln Listener) Accept() (conn *Conn, err error) {
	defer func() {
		if r := recover(); r != nil {
			switch r.(type) {
			case exception:
				e := r.(exception)
				glog.V(1).Infoln("fail to accept", err)
				res := response{}
				switch e.err {
				case ErrInvalidSecret:
					res.Status = StatusInvalidSecret
				case ErrInvalidProto:
					res.Status = StatusInvalidProto
				case ErrNoConnectionAvaliable:
					res.Status = StatusNoConnectionAvaliable
				default:
					res.Status = StatusUnknowErr
				}
				if err := binary.Write(e.cc, binary.BigEndian, res); err != nil {
					glog.V(1).Infoln("fail to write to cipherconn")
				}
			default:
				panic(r)
			}
		}
	}()

	var c net.Conn
	req := request{}
	for {
		c, err = ln.Listener.Accept()
		if err != nil {
			glog.V(1).Infoln("fail to accept from listener", err)
			return conn, err
		}

		cc := &CipherConn{
			Conn:   c,
			Cipher: ln.Cipher,
		}

		// Recieve
		if err := binary.Read(cc, binary.BigEndian, &req); err != nil {
			glog.V(1).Infoln("fail to read from cipherconn")
			continue
		}
		glog.V(3).Infoln("read from cipherconn", req)

		if req.Secret != ln.Secret {
			panic(exception{ErrInvalidSecret, cc})
			continue
		}

		if req.Proto != ln.Proto {
			panic(exception{ErrInvalidProto, cc})
			continue
		}

		conn, err := ln.CP.Get()
		if err != nil {
			panic(exception{err, cc})
			continue
		}

		conn.Cipher, err = NewCipher(req.Key[:])
		if err != nil {
			return conn, err
		}

		// send response
		res := response{Status: StatusOK}
		copy(res.IP[:], conn.IPNet.IP.To4())
		copy(res.IPMask[:], conn.IPNet.Mask)
		if err := binary.Write(cc, binary.BigEndian, res); err != nil {
			glog.V(1).Infoln("fail to write to cipherconn")
			continue
		}
		glog.V(3).Infoln("write to cipherconn", res)

		conn.Conn = c

		return conn, err
	}
}
