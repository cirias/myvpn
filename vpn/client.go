package vpn

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"time"

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

type InOuter interface {
	In() chan<- []byte
	Out() <-chan []byte
	InErr() <-chan error
	OutErr() <-chan error
}

func NewClient(ifce, conn InOuter) (c *Client, err error) {
	glog.Infoln("00000000000")
	r := bytes.NewBuffer(<-conn.Out())
	res := &response{}
	if err = binary.Read(r, binary.BigEndian, res); err != nil {
		glog.Errorln("fail to read response", err)
		return
	}
	glog.V(1).Infoln("res", res)

	switch res.Status {
	case statusOk:
	case statusNoIPAvaliable:
		return nil, ErrNoIPAvaliable
	default:
		return nil, ErrUnknownStatus
	}

	go func() {
		var b []byte
		t := time.NewTimer(0)
		for {
			t.Reset(10 * time.Second)
		CONN_OUT_LOOP:
			for {
				select {
				case <-c.stop:
					return
				case b = <-conn.Out():
					glog.V(1).Infoln("b = <-conn.Out()")
					break CONN_OUT_LOOP
				case <-t.C:
					glog.V(1).Infoln("conn.Out() timeout")
					t.Reset(time.Second)
				}
			}

			select {
			case <-c.stop:
				return
			case ifce.In() <- b:
				glog.V(1).Infoln("b -> ifce.In()")
			}
		}
	}()

	go func() {
		var b []byte
		t := time.NewTimer(0)
		for {
			select {
			case <-c.stop:
				return
			case b = <-ifce.Out():
				glog.V(1).Infoln("b = <-ifce.Out()")
			}

			t.Reset(10 * time.Second)
		CONN_IN_LOOP:
			for {
				select {
				case <-c.stop:
					return
				case conn.In() <- b:
					glog.V(1).Infoln("conn.In() <- b")
					break CONN_IN_LOOP
				case <-t.C:
					glog.V(1).Infoln("conn.In() is not avaliable")
					t.Reset(time.Second)
				}
			}
		}
	}()

	go func() {
		var err error
		for {
			select {
			case <-c.stop:
				break
			case err = <-ifce.InErr():
			case err = <-ifce.OutErr():
			}
			glog.Errorln("vpn client error", err)
		}
	}()

	c = &Client{
		stop:   make(chan struct{}),
		IP:     res.IP[:],
		IPMask: res.IPMask[:],
	}

	return
}

func (c *Client) Close() error {
	glog.Infoln("client closing")
	c.stop <- struct{}{}
	c.stop <- struct{}{}
	c.stop <- struct{}{}
	return nil
}
