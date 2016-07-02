package socket

import (
	"crypto/rand"
	"encoding/binary"
	"io"
	"time"

	"protocol"

	"github.com/golang/glog"
)

type Client struct {
	*Socket
	id idType
}

func NewClient(psk, remoteAddr string) (c *Client, err error) {
	c = &Client{
		Socket: NewSocket(),
	}

	if _, err = io.ReadFull(rand.Reader, c.id[:]); err != nil {
		return
	}

	go func() {
		for {
			conn, err := protocol.DialTCP(psk, remoteAddr)
			if err != nil {
				glog.Warningf("fail to dail %s, retry...\n", err)
				time.Sleep(10 * time.Second)
				continue
			}
			glog.V(2).Info("dial connection success")

			req := &request{
				Id: c.id,
			}
			glog.V(2).Info("send request ", req)
			if err := binary.Write(conn, binary.BigEndian, req); err != nil {
				return
			}

			res := &response{}
			if err := binary.Read(conn, binary.BigEndian, res); err != nil {
				return
			}
			glog.V(2).Info("recieve response ", res)

			if err := c.Socket.run(conn); err != nil {
				glog.Error("error occured during socket run", err)
			}

			break
		}
	}()

	return
}
