package main

import (
	//"encoding/binary"
	//"errors"
	"github.com/cirias/myvpn/common"
	"github.com/golang/glog"
	//"log"
	"net"
)

type UDPHandle struct {
	UDPAddr *net.UDPAddr
	UDPConn *net.UDPConn
	Input   chan *common.QueuedBuffer
	Output  chan *common.QueuedBuffer
}

func NewUDPHandle(udpAddr *net.UDPAddr) (handle *UDPHandle, err error) {
	udpconn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4zero, Port: 0})
	if err != nil {
		return
	}

	handle = &UDPHandle{
		UDPAddr: udpAddr,
		UDPConn: udpconn,
	}
	return
}

func (handle *UDPHandle) read(done <-chan struct{}) chan error {
	handle.Output = make(chan *common.QueuedBuffer)
	errc := make(chan error)
	var qb *common.QueuedBuffer
	go func() {
		defer close(errc)
		defer close(handle.Output)

		for {
			qb = common.LB.Get()
			select {
			case <-done:
				return
			default:
				n, _, err := handle.UDPConn.ReadFromUDP(qb.Buffer)
				qb.N = n
				if err != nil {
					qb.Return()
					errc <- err
				} else {
					glog.V(3).Infof("<%v> read n: %v data: %x\n", "handle", qb.N, qb.Buffer[:qb.N])
					handle.Output <- qb
				}
			}
		}
	}()
	return errc
}

func (handle *UDPHandle) write(done <-chan struct{}) chan error {
	handle.Input = make(chan *common.QueuedBuffer)
	errc := make(chan error)
	var qb *common.QueuedBuffer
	go func() {
		defer close(errc)
		defer close(handle.Input)

		for qb = range handle.Input {
			n, err := handle.UDPConn.WriteToUDP(qb.Buffer[:qb.N], handle.UDPAddr)
			glog.V(3).Infof("<%v> write n: %v data: %x\n", "handle", n, qb.Buffer[:qb.N])
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

func (handle *UDPHandle) Run(done chan struct{}) <-chan error {
	wec := handle.write(done)
	rec := handle.read(done)

	errc := common.Merge(done, wec, rec)

	return errc
}
