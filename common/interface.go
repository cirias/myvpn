package common

import (
	"github.com/golang/glog"
	"io"
	//"log"
	"sync"
)

const (
	MAXPACKETSIZE = 1500
)

type Interface struct {
	name   string
	rw     io.ReadWriter
	Input  chan *QueuedBuffer
	Output chan *QueuedBuffer
}

func NewInterface(name string, rw io.ReadWriter) *Interface {
	return &Interface{
		name: name,
		rw:   rw,
	}
}

func (i *Interface) read(done <-chan struct{}) chan error {
	i.Output = make(chan *QueuedBuffer, BUFFERQUEUESIZE)
	errc := make(chan error)
	var qb *QueuedBuffer
	go func() {
		defer close(errc)
		defer close(i.Output)

		for {
			qb = LB.Get()
			select {
			case <-done:
				return
			default:
				n, err := i.rw.Read(qb.Buffer)
				qb.N = n
				if err != nil {
					qb.Return()
					errc <- err
				} else {
					glog.V(3).Infof("read %vbytes from [%v]: %x\n", qb.N, i.name, qb.Buffer[:qb.N])
					i.Output <- qb
				}
			}
		}
	}()
	return errc
}

func (i *Interface) write(done <-chan struct{}) chan error {
	i.Input = make(chan *QueuedBuffer, BUFFERQUEUESIZE)
	errc := make(chan error)
	var qb *QueuedBuffer
	go func() {
		defer close(errc)
		defer close(i.Input)

		for qb = range i.Input {
			n, err := i.rw.Write(qb.Buffer[:qb.N])
			glog.V(3).Infof("wrote %vbytes to [%v]: %x\n", n, i.name, qb.Buffer[:n])
			qb.Return()
			select {
			case <-done:
				return
			default:
				if err != nil {
					errc <- err
				}
			}
		}
	}()
	return errc
}

func Merge(done <-chan struct{}, cs ...<-chan error) <-chan error {
	var wg sync.WaitGroup
	out := make(chan error)

	output := func(c <-chan error) {
		for err := range c {
			select {
			case out <- err:
			case <-done:
				return
			}
		}
		wg.Done()
	}
	wg.Add(len(cs))
	for _, c := range cs {
		go output(c)
	}

	go func() {
		wg.Wait()
		close(out)
	}()
	return out
}

func (i *Interface) Run(done chan struct{}) <-chan error {
	wec := i.write(done)
	rec := i.read(done)

	errc := Merge(done, wec, rec)

	return errc
}
