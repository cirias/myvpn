package shadowsocks

import (
	//"encoding/binary"
	//"fmt"
	"github.com/cirias/cvpn/common"
	"io"
	"net"
	//"strconv"
)

type Conn struct {
	net.Conn
	*Cipher
	buffers chan *common.QueuedBuffer
}

func NewConn(c net.Conn, cipher *Cipher) *Conn {
	return &Conn{
		Conn:    c,
		Cipher:  cipher,
		buffers: common.NewBufferQueue(common.MAXPACKETSIZE),
	}
}

func (c *Conn) Read(b []byte) (n int, err error) {
	if c.dec == nil {
		iv := make([]byte, c.info.ivLen)
		if _, err = io.ReadFull(c.Conn, iv); err != nil {
			return
		}
		if err = c.initDecrypt(iv); err != nil {
			return
		}
	}

	qb := <-c.buffers
	defer qb.Return()

	n, err = c.Conn.Read(qb.Buffer)
	if err != nil {
		return
	}

	c.decrypt(b[0:n], qb.Buffer[0:n])
	return
}

func (c *Conn) Write(b []byte) (n int, err error) {
	qb := <-c.buffers
	defer qb.Return()

	if c.enc == nil {
		iv, err := c.initEncrypt()
		if err != nil {
			return 0, err
		}

		// Put initialization vector in buffer, do a single write to send both
		// iv and data.
		copy(qb.Buffer, iv)
		c.encrypt(qb.Buffer[len(iv):], b)
	} else {
		c.encrypt(qb.Buffer, b)
	}

	n, err = c.Conn.Write(qb.Buffer)
	return
}
