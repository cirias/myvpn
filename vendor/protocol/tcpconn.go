package protocol

import (
	"net"
	"sync"

	"cipher"

	"github.com/golang/glog"
)

type state int

const (
	stateEstablished = iota
	stateReconnecting
)

func newTCPConn(c *net.TCPConn) (conn *TCPConn, err error) {
	// in case idle connection session being cleaned by NAT server
	if err = c.SetKeepAlive(true); err != nil {
		glog.Errorln("fail to set keep alive", err)
		return
	}

	conn = &TCPConn{
		writeCh: make(chan []byte),
		readCh:  make(chan []byte),
		stopCh:  make(chan struct{}),
	}
	conn.Conn = c
	return
}

type TCPConn struct {
	net.Conn
	cipher  *cipher.Cipher
	writeCh chan []byte
	readCh  chan []byte
	stopCh  chan struct{}
}

func (c *TCPConn) Close() error {
	// stopping client
	close(c.stopCh)

	close(c.writeCh)
	close(c.readCh)

	return c.Conn.Close()
}

func (c *TCPConn) Read(b []byte) (int, error) {
	p := <-c.readCh
	copy(b, p)
	return len(p), nil
}

func (c *TCPConn) Write(b []byte) (int, error) {
	c.writeCh <- b
	return len(b), nil
}

func (c *TCPConn) readWrite() (err error) {
	errorCh := make(chan error)
	stopReadCh := make(chan struct{})
	stopWriteCh := make(chan struct{})
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()

		var b []byte
		for {
			select {
			case <-stopWriteCh:
				return
			case b = <-c.writeCh:
			}

			if err := send(c.cipher, c.Conn, &Packet{&Header{TypeTraffic, uint16(len(b))}, b}); err != nil {
				glog.Errorln("fail to write to connection", err)
				errorCh <- err
			}
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		packet := &Packet{Header: &Header{}}
		for {
			select {
			case <-stopReadCh:
				return
			default:
			}

			// TODO check header type
			if err := recieve(c.cipher, c.Conn, packet); err != nil {
				glog.Errorln("fail to read from connection", err)
				errorCh <- err
			}
			c.readCh <- packet.Body.([]byte)
		}
	}()

	select {
	case err = <-errorCh:
	case <-c.stopCh:
	}

	close(stopReadCh)
	close(stopWriteCh)

	// wait until read and write goroutine return
	wg.Wait()
	return
}

func (c *TCPConn) reconnect() error {
	return nil
}

func (c *TCPConn) disconnect() {

}
