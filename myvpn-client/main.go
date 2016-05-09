package main

import (
	"errors"
	"flag"
	"os"
	"os/signal"
	"syscall"

	"protocol"
	"tun"

	"github.com/golang/glog"
)

func main() {
	var network, secret, serverAddr, upScript, downScript string

	flag.StringVar(&network, "network", "udp", "network of transport layer")
	flag.StringVar(&secret, "secret", "", "secret")
	flag.StringVar(&serverAddr, "server-addr", "127.0.0.1:9525", "server address")
	flag.StringVar(&upScript, "up-script", "./if-up.sh", "up shell script file path")
	flag.StringVar(&downScript, "down-script", "./if-down.sh", "down shell script file path")
	flag.Parse()

	conn, err := protocol.Dial(network, secret, serverAddr)
	if err != nil {
		glog.Fatalln(err)
	}
	defer conn.Close()
	glog.Infoln("connected to server", conn.IP())

	tun, err := tun.NewTUN("", &conn.IPNet.IP, &conn.IPNet.Mask)
	if err != nil {
		glog.Fatalln(err)
	}
	defer tun.Close()

	err = tun.Up(upScript, conn.IP().String())
	if err != nil {
		glog.Fatalln(err)
	}
	defer tun.Down(downScript, conn.IP().String())
	glog.Infoln(tun.Name(), "is ready")

	errc := make(chan error)

	go func() {
		b := make([]byte, 65535)
		for {
			n, err := conn.Read(b)
			if err != nil {
				glog.Errorln("fail to read from connection", err)
				errc <- err
				continue
			}
			glog.V(3).Infoln("read from connection", b[:n])

			_, err = tun.Write(b[:n])
			if err != nil {
				glog.Errorln("fail to write to tun", err)
				errc <- err
			}
		}
	}()
	glog.Infoln("start processing data from connection")

	go func() {
		b := make([]byte, 65535)
		for {
			n, err := tun.Read(b)
			if err != nil {
				glog.Errorln("fail to read from tun", err)
				errc <- err
				continue
			}
			glog.V(3).Infoln("read from tun", b[:n])

			_, err = conn.Write(b[:n])
			if err != nil {
				glog.Errorln("fail to write to connection", err)
				errc <- err
			}
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
