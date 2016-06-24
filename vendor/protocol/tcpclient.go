package protocol

import (
	"errors"
	"net"
	"sync"
)

type state int

const (
	stateConnecting = iota
	stateEstablished
	stateReconnecting
	stateDisconnecting
)

type tcpClient struct {
	net.Conn
	stateCh   chan state
	writeCh   chan []byte
	readCh    chan []byte
	stopCh    chan struct{}
	stoppedCh chan struct{}
}

func (c *tcpClient) handle() {
	c.stoppedCh = make(chan struct{})
	defer close(c.stoppedCh)

	for state := range c.stateCh {
		switch state {
		case stateConnecting:
			c.connect()
		case stateEstablished:
			c.readWrite()
		case stateReconnecting:
			c.reconnect()
		case stateDisconnecting:
			c.disconnect()
		default:
			panic(errors.New("unknown state"))
		}
	}
}

func (c *tcpClient) Close() {
	// stopping client
	close(c.stopCh)

	<-c.stoppedCh
	close(c.stateCh)
	close(c.writeCh)
	close(c.readCh)
}

func (c *tcpClient) Read(b []byte) (int, error) {
	return 0, nil
}

func (c *tcpClient) Write(b []byte) (int, error) {
	return 0, nil
}

func (c *tcpClient) connect() {
}

func (c *tcpClient) readWrite() {
	errorCh := make(chan error)
	defer close(errorCh)
	stopReadCh := make(chan struct{})
	stopWriteCh := make(chan struct{})
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()

		var b []byte
		for {
			select {
			case <-c.stopCh:
				return
			case b = <-c.writeCh:
			}

			if _, err := c.Conn.Write(b); err != nil {
				errorCh <- err
				// TODO
			}
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		var b []byte
		for {
			select {
			case <-c.stopCh:
				return
			default:
			}

			n, err := c.Conn.Read(b)
			if err != nil {
				errorCh <- err
				// TODO
			}
			c.readCh <- b[:n]
		}
	}()

	go func() {
		select {
		case <-c.stopCh:
			go func() { c.stateCh <- stateDisconnecting }()
		case <-errorCh:
			go func() { c.stateCh <- stateReconnecting }()
		}

		close(stopReadCh)
		close(stopWriteCh)
	}()

	// wait until read and write goroutine return
	wg.Wait()
}

func (c *tcpClient) reconnect() {

}

func (c *tcpClient) disconnect() {

}
