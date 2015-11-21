package common

import (
	"encoding/binary"
	"io"
	"log"
)

const (
	MAXPACKETSIZE = 65535
)

type Interface struct {
	rw      io.ReadWriter
	buffers chan *QueuedBuffer
	Input   chan *QueuedBuffer
	Output  chan *QueuedBuffer
	error   chan error
}

func NewInterface(rw io.ReadWriter) *Interface {
	return &Interface{
		rw:      rw,
		buffers: NewBufferQueue(MAXPACKETSIZE),
		Input:   make(chan *QueuedBuffer, BUFFERQUEUESIZE),
		Output:  make(chan *QueuedBuffer, BUFFERQUEUESIZE),
		error:   make(chan error, 1),
	}
}

func (i *Interface) readPacket() {

}

func (i *Interface) readRoutine() {
	//defer func() {
	//if err := recover(); err != nil {
	//i.error <- err.(error)
	//}
	//}()
	var qb *QueuedBuffer
	var totalLen int
	for qb = range i.buffers {
		n, err := io.ReadAtLeast(i.rw, qb.Buffer, 4)
		if err != nil {
			//panic(err)
			i.error <- err
			qb.Return()
			return
		}

		totalLen = int(binary.BigEndian.Uint16(qb.Buffer[2:4]))

		for qb.N = n; qb.N < totalLen; qb.N += n {
			n, err = i.rw.Read(qb.Buffer[qb.N:])
			if err != nil {
				i.error <- err
				qb.Return()
				return
			}
		}
		qb.N = totalLen

		log.Printf("read n: %v data: %x\n", qb.N, qb.Buffer[:qb.N])
		i.Output <- qb
	}
}

func (i *Interface) writeRoutine() {
	var qb *QueuedBuffer
	for qb = range i.Input {
		n, err := i.rw.Write(qb.Buffer[:qb.N])
		log.Printf("write n: %v data: %x\n", n, qb.Buffer[:qb.N])
		qb.Return()
		if err != nil {
			i.error <- err
		}
	}
}

func (i *Interface) Run() (err error) {
	defer func() {
		//close(i.buffers)
		close(i.Input)
		close(i.Output)
	}()

	go i.readRoutine()
	go i.writeRoutine()

	err = <-i.error

	return
}
