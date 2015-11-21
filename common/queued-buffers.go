package common

const (
	BUFFERQUEUESIZE = 32
)

type QueuedBuffer struct {
	Buffer []byte
	N      int // valid length of the Buffer
	queue  chan *QueuedBuffer
}

func NewQueuedBuffer(queue chan *QueuedBuffer, size int) *QueuedBuffer {
	buffer := make([]byte, size)
	return &QueuedBuffer{Buffer: buffer, queue: queue}
}

func (qb *QueuedBuffer) Return() {
	qb.N = 0
	if qb.queue != nil {
		qb.queue <- qb
	}
}

func NewBufferQueue(bufferSize int) (bufferQueue chan *QueuedBuffer) {
	bufferQueue = make(chan *QueuedBuffer, BUFFERQUEUESIZE)
	for i := 0; i < BUFFERQUEUESIZE; i++ {
		bufferQueue <- NewQueuedBuffer(bufferQueue, bufferSize)
	}
	return
}
