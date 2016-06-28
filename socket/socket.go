package socket

import (
	"sync"

	"protocol"
)

type typeState int

const (
	stateRunning = iota
	stateStopped
)

type Socket struct {
	writeCh   chan []byte
	readCh    chan []byte
	stopCh    chan struct{}
	stoppedCh chan struct{}
}

func NewSocket() *Socket {
	return &Socket{
		writeCh:   make(chan []byte),
		readCh:    make(chan []byte),
		stopCh:    make(chan struct{}),
		stoppedCh: make(chan struct{}),
	}
}

func (s *Socket) stop() {
	select {
	case s.stopCh <- struct{}{}:
		<-s.stoppedCh
	default:
	}
}

func (s *Socket) start(conn protocol.Conn) {
	go func() {
		if err := s.run(conn); err != nil {

		} else {
			s.stoppedCh <- struct{}{}
		}
	}()
}

func (s *Socket) run(conn protocol.Conn) (err error) {
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
			case b = <-s.writeCh:
			}

			if _, err := conn.Write(b); err != nil {
				errorCh <- err
			}
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		var b []byte
		for {
			select {
			case <-stopReadCh:
				return
			default:
			}

			n, err := conn.Read(b)
			if err != nil {
				errorCh <- err
			}
			s.readCh <- b[:n]
		}
	}()

	select {
	case err = <-errorCh:
	case <-s.stopCh:
	}

	close(stopReadCh)
	close(stopWriteCh)

	// wait until read and write goroutine return
	wg.Wait()
	return
}

func (s *Socket) Write(b []byte) (int, error) {
	s.writeCh <- b
	return len(b), nil
}

func (s *Socket) Read(b []byte) (int, error) {
	p := <-s.readCh
	copy(b, p)
	return len(p), nil
}

func (s *Socket) Close() error {
	close(s.stopCh)
	return nil
}
