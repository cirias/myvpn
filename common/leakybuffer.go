package common

type LeakyBuffer struct {
	bufferSize int
	freeList   chan *QueuedBuffer
}

const leakyBufferSize = 4096
const maxBufferCount = 2048

var LB = NewLeakyBuffer(maxBufferCount, leakyBufferSize)

func NewLeakyBuffer(n, bufferSize int) *LeakyBuffer {
	return &LeakyBuffer{
		bufferSize: bufferSize,
		freeList:   make(chan *QueuedBuffer, n),
	}
}

func (lb *LeakyBuffer) Get() *QueuedBuffer {
	var qb *QueuedBuffer
	select {
	case qb = <-lb.freeList:
	default:
		qb = NewQueuedBuffer(lb.freeList, lb.bufferSize)
	}
	return qb
}
