package main

import (
	"errors"
	"flag"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"protocol"
	"tun"

	"github.com/golang/glog"
)

func dial(network, secret, serverAddr string) (conn protocol.Conn, err error) {
	switch network {
	case "udp":
		err = errors.New("UDP has not been implement")
	case "tcp":
		conn, err = protocol.DialTCP(secret, serverAddr)
	default:
		err = errors.New("unknown protocol")
	}

	return
}

func main() {
	var network, secret, serverAddr, upScript, downScript, addrScript string

	flag.StringVar(&network, "network", "udp", "network of transport layer")
	flag.StringVar(&secret, "secret", "", "secret")
	flag.StringVar(&serverAddr, "server-addr", "127.0.0.1:9525", "server address")
	flag.StringVar(&upScript, "up-script", "./if-up.sh", "up shell script file path")
	flag.StringVar(&downScript, "down-script", "./if-down.sh", "down shell script file path")
	flag.StringVar(&addrScript, "addr-script", "./if-addr.sh", "script file path for set ip address")
	flag.Parse()

	conn, err := dial(network, secret, serverAddr)
	if err != nil {
		glog.Fatalln(err)
	}
	defer conn.Close()
	glog.Infoln("connected to server", conn.LocalIPAddr())

	tun, err := tun.NewTUN("")
	if err != nil {
		glog.Fatalln(err)
	}
	defer tun.Close()

	err = tun.Run(addrScript, (&net.IPNet{conn.LocalIPAddr(), conn.IPNetMask()}).String())
	if err != nil {
		glog.Fatalln(err)
	}
	err = tun.Run(upScript, conn.ExternalRemoteIPAddr().String())
	if err != nil {
		glog.Fatalln(err)
	}
	defer tun.Run(downScript, (&net.IPNet{conn.LocalIPAddr(), conn.IPNetMask()}).String(), conn.ExternalRemoteIPAddr().String())
	glog.Infoln(tun.Name(), "is ready")

	errc := make(chan error)

	go func() {
		b := make([]byte, 65535)
		for {
			var n int

			for {
				n, err = conn.ReadIPPacket(b)
				if err == nil {
					break
				}
				conn.Close()

				for {
					conn, err = dial(network, secret, serverAddr)
					if err != nil {
						time.Sleep(10 * time.Second)
						continue
					}
					err = tun.Run(addrScript, (&net.IPNet{conn.LocalIPAddr(), conn.IPNetMask()}).String())
					if err != nil {
						glog.Warningln(err)
					}
					break
				}
			}

			/*
			 * n, err := conn.ReadIPPacket(b)
			 * if err != nil {
			 *   glog.Errorln("fail to read from connection", err)
			 *   errc <- err
			 *   continue
			 * }
			 */
			glog.V(3).Infoln("recieve IP packet from connection", b[:n])

			_, err = tun.Write(b[:n])
			if err != nil {
				glog.Errorln("fail to write to tun", err)
				errc <- err
			}
			glog.V(3).Infoln("send IP packet to tun", b[:n])
		}
	}()
	glog.Infoln("start processing data from connection")

	go func() {
		b := make([]byte, 65535)
		for {
			n, err := tun.ReadIPPacket(b)
			if err != nil {
				glog.Errorln("fail to read from tun", err)
				errc <- err
				continue
			}
			glog.V(3).Infoln("recieve IP packet from tun", b[:n])

			for {
				_, err = conn.Write(b[:n])
				if err == nil {
					break
				}
				time.Sleep(10 * time.Second)
			}
			/*
			 * if err != nil {
			 *   glog.Errorln("fail to write to connection", err)
			 *   errc <- err
			 * }
			 */
			glog.V(3).Infoln("send IP packet to connection", b[:n])
		}
	}()
	glog.Infoln("start processing data from tun")

	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt, os.Kill, syscall.SIGTERM)

		s := <-c
		errc <- errors.New(s.String())
	}()

	err = <-errc
	if err != nil {
		glog.Infoln("process quit", err)
	}

	return
}
