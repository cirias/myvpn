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
	*CipherReadWriter
	IPNet *net.IPNet
	pool  *ConnectionPool
}

func (c *Conn) Close() error {
	if c.pool != nil {
		c.pool.Put(c)
	}
	return c.CipherReadWriter.Close()
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

	crw := &CipherReadWriter{
		ReadWriteCloser: c,
		Cipher:          cipher,
	}
	if err = binary.Write(crw, binary.BigEndian, req); err != nil {
		glog.Infoln("Dial:", err)
		return
	}

	// Recieve
	res := response{}
	if err = binary.Read(crw, binary.BigEndian, &res); err != nil {
		return
	}

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
		CipherReadWriter: &CipherReadWriter{},
		IPNet:            &net.IPNet{res.IP[:], res.IPMask[:]},
	}
	conn.ReadWriteCloser = c
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
	Err error
	CRW *CipherReadWriter
}

func (ln Listener) Accept() (conn *Conn, err error) {
	defer func() {
		if r := recover(); r != nil {
			switch r.(type) {
			case exception:
				e := r.(exception)
				res := response{}
				switch e.Err {
				case ErrInvalidSecret:
					res.Status = StatusInvalidSecret
				case ErrInvalidProto:
					res.Status = StatusInvalidProto
				case ErrNoConnectionAvaliable:
					res.Status = StatusNoConnectionAvaliable
				default:
					res.Status = StatusUnknowErr
				}
				binary.Write(e.CRW, binary.BigEndian, res)
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
			return conn, err
		}

		crw := &CipherReadWriter{
			ReadWriteCloser: c,
			Cipher:          ln.Cipher,
		}

		// Recieve
		binary.Read(crw, binary.BigEndian, &req)

		if req.Secret != ln.Secret {
			panic(exception{ErrInvalidSecret, crw})
			continue
		}

		if req.Proto != ln.Proto {
			panic(exception{ErrInvalidProto, crw})
			continue
		}

		conn, err := ln.CP.Get()
		if err != nil {
			panic(exception{err, crw})
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
		binary.Write(crw, binary.BigEndian, res)

		conn.ReadWriteCloser = c

		return conn, err
	}
}
