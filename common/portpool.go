package common

import (
	"errors"
)

var ErrPortPoolEmpty = errors.New("No avaliable port")
var ErrPortPoolFull = errors.New("PortPool is full")

type PortPool struct {
	//base   int
	//size   int
	//exists []bool
	ports chan int
}

func NewPortPool(base, size int) (pool *PortPool) {
	pool = &PortPool{
		ports: make(chan int, size),
	}

	for port := base + 1; port <= base+size; port++ {
		pool.ports <- port
	}
	return
}

func (pool *PortPool) GetPort() (port int, err error) {
	select {
	case port = <-pool.ports:
	default:
		err = ErrPortPoolEmpty
	}
	return
}

func (pool *PortPool) ReturnPort(port int) (err error) {
	select {
	case pool.ports <- port:
	default:
		err = ErrPortPoolFull
	}
	return
}
