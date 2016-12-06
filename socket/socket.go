package socket

import (
	"errors"
	"io"
	"sync"
	"time"

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
			glog.Errorln("error occured during socket run", err)
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
				break
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
				break
			}
			s.readCh <- b[:n]
		}
	}()

	select {
	case err = <-errorCh:
		glog.Errorln("socket run error ", err)
	case <-s.stopCh:
	}

	close(stopReadCh)
	close(stopWriteCh)

	// wait until read and write goroutine return
	wg.Wait()

	conn.Close()
	return
}

func (s *Socket) Write(b []byte) (int, error) {
	glog.V(2).Infoln("write", b)
	t := time.NewTimer(time.Microsecond)
	select {
	case s.writeCh <- b:
		return len(b), nil
	case <-t.C:
		return 0, errors.New("socket stopped")
	}
}

func (s *Socket) Read(b []byte) (n int, err error) {
	p := <-s.readCh
	copy(b, p)
	n = len(p)
	glog.V(2).Infoln("read", b[:n])
	return
}

func (s *Socket) Close() error {
	close(s.stopCh)
	return nil
}
