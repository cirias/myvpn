package tcp

import (
	"encoding/binary"
	"io"
	"log"
	"net"

	"github.com/golang/glog"
)

type Conn struct {
	conn     net.Conn
	InCh     chan []byte
	OutCh    chan []byte
	InErrCh  chan error
	OutErrCh chan error
}

func NewConn(c net.Conn) *Conn {
	conn := &Conn{
		conn:     c,
		InCh:     make(chan []byte),
		OutCh:    make(chan []byte),
		InErrCh:  make(chan error),
		OutErrCh: make(chan error),
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

func (c *Conn) InErr() <-chan error {
	return c.InErrCh
}

func (c *Conn) OutErr() <-chan error {
	return c.OutErrCh
}

func (c *Conn) reading() {
	var len uint16
	b := make([]byte, 65535)
	for {
		glog.V(3).Infoln("reading")
		if err := binary.Read(c.conn, binary.BigEndian, &len); err != nil {
			select {
			case c.OutErrCh <- err:
			default:
			}
			break
		}

		n, err := io.ReadFull(c.conn, b[:len])
		// n, err := c.conn.Read(b[:len])
		if err != nil {
			log.Println(err)
			select {
			case c.OutErrCh <- err:
			default:
			}
			break
		}

		glog.V(3).Infoln("read", b[:n])
		c.OutCh <- b[:n]
	}
}

func (c *Conn) writing() {
	for b := range c.InCh {
		glog.V(3).Infoln("writing", b)
		if err := binary.Write(c.conn, binary.BigEndian, uint16(len(b))); err != nil {
			if err != nil {
				log.Println(err)
				select {
				case c.InErrCh <- err:
				default:
				}
				break
			}
		}

		n, err := c.conn.Write(b)
		if err != nil {
			log.Println(err)
			select {
			case c.InErrCh <- err:
			default:
			}
			break
		}
		glog.V(3).Infoln("wrote", b[:n])
	}
}

func (c *Conn) Close() error {
	return c.conn.Close()
}
