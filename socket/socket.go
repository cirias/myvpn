package socket

import (
	"io"
	"sync"

	"github.com/golang/glog"
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

func (s *Socket) start(conn io.ReadWriteCloser) {
	go func() {
		if err := s.run(conn); err != nil {
			glog.Error("socket run error", err)
		} else {
			s.stoppedCh <- struct{}{}
		}
	}()
}

func (s *Socket) run(conn io.ReadWriteCloser) (err error) {
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
		b := make([]byte, 65535)
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

	close(s.readCh)
	close(s.writeCh)
	conn.Close()
	return
}

func (s *Socket) Write(b []byte) (int, error) {
	glog.V(2).Info("write ", b)
	s.writeCh <- b
	return len(b), nil
}

func (s *Socket) Read(b []byte) (n int, err error) {
	p := <-s.readCh
	copy(b, p)
	n = len(p)
	glog.V(2).Info("read ", b[:n])
	return
}

func (s *Socket) Close() error {
	close(s.stopCh)
	return nil
}
