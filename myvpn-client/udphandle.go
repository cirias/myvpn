package main

import (
	"github.com/cirias/myvpn/common"
	"github.com/golang/glog"
	"net"
	"time"
)

type UDPHandle struct {
	timer   *time.Timer
	cipher  *common.Cipher
	UDPAddr *net.UDPAddr
	UDPConn *net.UDPConn
	Input   chan *common.QueuedBuffer
	Output  chan *common.QueuedBuffer
}

func NewUDPHandle(udpAddr *net.UDPAddr, cipher *common.Cipher) (handle *UDPHandle, err error) {
	udpconn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4zero, Port: 0})
	if err != nil {
		return
	}

	handle = &UDPHandle{
		cipher:  cipher,
		UDPAddr: udpAddr,
		UDPConn: udpconn,
	}
	return
}

func (handle *UDPHandle) read(done <-chan struct{}) <-chan error {
	handle.Output = make(chan *common.QueuedBuffer)
	errc := make(chan error)
	var cipherQb *common.QueuedBuffer
	var plainQb *common.QueuedBuffer
	go func() {
		defer close(errc)
		defer close(handle.Output)

		for {
			cipherQb = common.LB.Get()
			n, raddr, err := handle.UDPConn.ReadFromUDP(cipherQb.Buffer)
			cipherQb.N = n
			select {
			case <-done:
				return
			default:
				if err != nil {
					cipherQb.Return()
					errc <- err
				}
			}

			// Decrypt
			plainQb = common.LB.Get()
			if err := handle.cipher.Decrypt(cipherQb.Buffer[:common.IVSize], plainQb.Buffer, cipherQb.Buffer[common.IVSize:cipherQb.N]); err != nil {
				cipherQb.Return()
				plainQb.Return()
				glog.Fatalln("client decrypt", err)
			}

			plainQb.N = cipherQb.N - common.IVSize
			cipherQb.Return()

			glog.V(3).Infof("read %vbytes to <%v>: %x\n", plainQb.N, raddr, plainQb.Buffer[:plainQb.N])
			handle.Output <- plainQb
		}
	}()
	return errc
}

func (handle *UDPHandle) write(done <-chan struct{}) <-chan error {
	handle.Input = make(chan *common.QueuedBuffer)
	errc := make(chan error)
	var plainQb *common.QueuedBuffer
	var cipherQb *common.QueuedBuffer
	go func() {
		defer close(errc)
		defer close(handle.Input)

		for plainQb = range handle.Input {
			// Encrypt
			cipherQb = common.LB.Get()

			if err := handle.cipher.Encrypt(cipherQb.Buffer[:common.IVSize], cipherQb.Buffer[common.IVSize:], plainQb.Buffer[:plainQb.N]); err != nil {
				plainQb.Return()
				cipherQb.Return()
				glog.Fatalln("client encrypt", err)
			}

			cipherQb.N = common.IVSize + plainQb.N
			plainQb.Return()

			n, err := handle.UDPConn.WriteToUDP(cipherQb.Buffer[:cipherQb.N], handle.UDPAddr)
			cipherQb.Return()
			glog.V(3).Infof("write %vbytes to <%v>: %x\n", n, handle.UDPAddr, cipherQb.Buffer[:n])
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

func (handle *UDPHandle) Run(done chan struct{}) (errc <-chan error) {
	wec := handle.write(done)
	rec := handle.read(done)

	errc = common.Merge(done, wec, rec)

	return
}
