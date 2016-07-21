package encrypted

import (
	"github.com/cirias/myvpn/cipher"
	"github.com/cirias/myvpn/tcp"
)

type Conn struct {
	*tcp.Conn
	cipher *cipher.Cipher

	InCh  chan []byte
	OutCh chan []byte
}

func NewConn(cph *cipher.Cipher, c *tcp.Conn) *Conn {
	conn := &Conn{
		cipher: cph,
		Conn:   c,

		InCh:  make(chan []byte),
		OutCh: make(chan []byte),
	}

	go conn.reading()
	go conn.writing()

	return conn
}

func (c *Conn) In() chan<- []byte {
	return c.InCh
}

func (c *Conn) Out() <-chan []byte {
	return c.OutCh
}

func (c *Conn) reading() {
	for b := range c.Conn.OutCh {
		c.OutCh <- c.decrypt(b)
	}
}

func (c *Conn) writing() {
	for b := range c.InCh {
		c.Conn.InCh <- c.encrypt(b)
	}
}

func (c *Conn) encrypt(b []byte) []byte {
	// TODO
	return b
}

func (c *Conn) decrypt(b []byte) []byte {
	// TODO
	return b
}
