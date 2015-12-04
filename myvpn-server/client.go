package main

import (
	"errors"
	"github.com/cirias/myvpn/common"
	"github.com/golang/glog"
	"net"
	"strconv"
	"time"
)

var ErrClientTimeout = errors.New("client timeout")

type Client struct {
	Cipher         *common.Cipher
	Port           int
	UDPAddr        *net.UDPAddr
	IPNet          *net.IPNet
	UDPConn        *net.UDPConn
	Input          chan *common.QueuedBuffer
	Output         chan *common.QueuedBuffer
	LastActiveTime time.Time
	Quit           chan error
}

func NewClient(cipher *common.Cipher, port int, ipNet *net.IPNet) (client *Client, err error) {
	udpAddr, err := net.ResolveUDPAddr("udp", ":"+strconv.Itoa(port))
	if err != nil {
		return
	}
	udpconn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		return
	}

	client = &Client{
		Cipher:  cipher,
		Port:    port,
		IPNet:   ipNet,
		UDPConn: udpconn,
	}
	return
}

func (client *Client) read(done <-chan struct{}) <-chan error {
	client.Output = make(chan *common.QueuedBuffer)
	errc := make(chan error)
	var cipherQb *common.QueuedBuffer
	var plainQb *common.QueuedBuffer
	go func() {
		defer close(errc)
		defer close(client.Output)

		for {
			cipherQb = common.LB.Get()
			select {
			case <-done:
				return
			default:
				n, raddr, err := client.UDPConn.ReadFromUDP(cipherQb.Buffer)
				cipherQb.N = n
				if err != nil {
					cipherQb.Return()
					errc <- err
					continue
				}

				plainQb = common.LB.Get()
				if err := client.Cipher.Decrypt(cipherQb.Buffer[:common.IVSize], plainQb.Buffer, cipherQb.Buffer[common.IVSize:cipherQb.N]); err != nil {
					cipherQb.Return()
					plainQb.Return()
					errc <- err
					continue
				}
				plainQb.N = cipherQb.N - common.IVSize
				cipherQb.Return()

				glog.V(3).Infof("<%v> read n: %v data: %x\n", "client", plainQb.N, plainQb.Buffer[:plainQb.N])
				client.UDPAddr = raddr
				client.LastActiveTime = time.Now()
				client.Output <- plainQb
			}
		}
	}()
	return errc
}

func (client *Client) write(done <-chan struct{}) <-chan error {
	client.Input = make(chan *common.QueuedBuffer)
	errc := make(chan error)
	var plainQb *common.QueuedBuffer
	var cipherQb *common.QueuedBuffer
	go func() {
		defer close(errc)
		defer close(client.Input)

		for plainQb = range client.Input {
			cipherQb = common.LB.Get()
			if err := client.Cipher.Encrypt(cipherQb.Buffer[:common.IVSize], cipherQb.Buffer[common.IVSize:], plainQb.Buffer[:plainQb.N]); err != nil {
				cipherQb.Return()
				plainQb.Return()
				glog.Fatalln("client encrypt", err)
				continue
			}
			cipherQb.N = common.IVSize + plainQb.N
			plainQb.Return()

			n, err := client.UDPConn.WriteToUDP(cipherQb.Buffer[:cipherQb.N], client.UDPAddr)
			glog.V(3).Infof("<%v> write n: %v data: %x\n", "client", n, cipherQb.Buffer[:cipherQb.N])
			cipherQb.Return()
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

func (client *Client) Run(done <-chan struct{}) <-chan error {
	wec := client.write(done)
	rec := client.read(done)

	client.Quit = make(chan error)
	go func(done <-chan struct{}) {
		defer close(client.Quit)
		<-done
	}(done)

	errc := common.Merge(done, wec, rec, client.Quit)

	return errc
}
