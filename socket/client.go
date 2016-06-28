package socket

import (
	"crypto/rand"
	"encoding/binary"
	"io"
	"time"

	"protocol"
)

type Client struct {
	*Socket
	id idType
}

func NewSocketClient(psk, remoteAddr string) *Client {
	c := &Client{
		Socket: NewSocket(),
	}

	go func() {
		for {
			conn, err := protocol.DialTCP(psk, remoteAddr)
			if err != nil {
				time.Sleep(10 * time.Second)
				continue
			}

			req := &request{
				reqType: typeReqConnect,
			}
			if _, err = io.ReadFull(rand.Reader, req.id[:]); err != nil {
				return
			}
			if err := binary.Write(conn, binary.BigEndian, req); err != nil {
				return
			}

			res := &response{}
			if err := binary.Read(conn, binary.BigEndian, res); err != nil {
				return
			}

			// TODO

			c.Socket.start(conn)

			break
		}
	}()

	return c
}
