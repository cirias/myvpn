package vpn

import (
	"encoding/binary"
	"errors"
	"io"
	"net"
	"sync"

	"github.com/golang/glog"
)

var ErrUnknownStatus = errors.New("unknown status")

type Client struct {
	ifce   io.ReadWriter
	conn   io.ReadWriter
	stop   chan struct{}
	IP     net.IP
	IPMask net.IPMask
}

func NewClient(ifce, conn io.ReadWriter) (c *Client, err error) {

	res := &response{}
	if err = binary.Read(conn, binary.BigEndian, res); err != nil {
		glog.Errorln("fail to read response", err)
		return
	}

	switch res.Status {
	case statusOk:
	case statusNoIPAvaliable:
		return nil, ErrNoIPAvaliable
	default:
		return nil, ErrUnknownStatus
	}

	c = &Client{
		ifce:   ifce,
		conn:   conn,
		stop:   make(chan struct{}),
		IP:     res.IP[:],
		IPMask: res.IPMask[:],
	}

	go func() {
		if err := c.ReadWrite(); err != nil {
			glog.Errorln("vpn client error", err)
		}
	}()

	return
}

func (c *Client) Close() error {
	glog.Infoln("client closing")
	close(c.stop)
	return nil
}

func (c *Client) ReadWrite() (err error) {
	var wg sync.WaitGroup
	errCh := make(chan error)
	defer close(errCh)

	wg.Add(1)
	go func() {
		defer wg.Done()
		b := make([]byte, 65535)
		for {
			select {
			case <-c.stop:
				return
			default:
			}

			n, err := c.conn.Read(b)
			if err != nil {
				glog.Errorln("fail to read from connection", err)
				errCh <- err
				continue
			}
			glog.V(1).Infoln("recieve IP packet from connection", b[:n])

			_, err = c.ifce.Write(b[:n])
			if err != nil {
				glog.Errorln("fail to write to ifce", err)
				errCh <- err
			}
			glog.V(1).Infoln("send IP packet to ifce", b[:n])
		}
	}()
	glog.Infoln("start processing data from connection")

	wg.Add(1)
	go func() {
		wg.Done()
		b := make([]byte, 65535)
		for {
			select {
			case <-c.stop:
				return
			default:
			}

			n, err := c.ifce.Read(b)
			if err != nil {
				glog.Errorln("fail to read from ifce", err)
				errCh <- err
				continue
			}
			glog.V(1).Infoln("recieve IP packet from ifce", b[:n])

			_, err = c.conn.Write(b[:n])
			if err != nil {
				glog.Errorln("fail to write to connection", err)
				errCh <- err
			}
			glog.V(1).Infoln("send IP packet to connection", b[:n])
		}
	}()
	glog.Infoln("start processing data from ifce")

	// TODO error handle
	go func() {
		for err := range errCh {
			glog.Errorln(err)
		}
	}()

	wg.Wait()

	glog.Infoln("stopped")

	return
}
