package main

import (
	"errors"
	"flag"
	"net"
	"os"
	"os/signal"
	"syscall"

	"protocol"
	"tun"

	"github.com/golang/glog"
)

func listen(network, secret, listenAddr string, ip net.IP, ipNet *net.IPNet) (ln protocol.Listener, err error) {
	switch network {
	case "udp":
		// ln, err = protocol.ListenUDP(secret, listenAddr, ip, ipNet)
		err = errors.New("UDP has not been implement")
	case "tcp":
		ln, err = protocol.ListenTCP(secret, listenAddr, ip, ipNet)
	default:
		err = errors.New("unknown protocol")
	}

	return
}

func main() {
	var network, secret, listenAddr, ipnet, upScript, downScript string

	flag.StringVar(&network, "network", "udp", "network of transport layer")
	flag.StringVar(&secret, "secret", "", "secret")
	flag.StringVar(&listenAddr, "listen-addr", "0.0.0.0:9525", "listening address")
	flag.StringVar(&ipnet, "ipnet", "10.0.200.1/24", "internal ip net")
	flag.StringVar(&upScript, "up-script", "./if-up.sh", "up shell script file path")
	flag.StringVar(&downScript, "down-script", "./if-down.sh", "down shell script file path")
	flag.Parse()

	ip, ipNet, err := net.ParseCIDR(ipnet)
	if err != nil {
		glog.Fatalln(err)
	}
	ln, err := listen(network, secret, listenAddr, ip, ipNet)
	if err != nil {
		glog.Fatalln(err)
	}
	defer ln.Close()
	glog.Infoln("start listening")

	tun, err := tun.NewTUN("")
	if err != nil {
		glog.Fatalln(err)
	}
	defer tun.Close()

	err = tun.Run(upScript, (&net.IPNet{ip, ipNet.Mask}).String(), listenAddr)
	if err != nil {
		glog.Fatalln(err)
	}
	defer tun.Run(downScript, (&net.IPNet{ip, ipNet.Mask}).String(), listenAddr)
	glog.Infoln(tun.Name(), " is ready")

	s := NewServer(tun, ipNet)
	defer s.Close()

	errc := make(chan error)

	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt, os.Kill, syscall.SIGTERM)

		s := <-c
		errc <- errors.New(s.String())
	}()

	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				glog.Errorln("fail to accept", err)
			}

			go s.Handle(c)
		}
	}()
	glog.Infoln("waiting client")

	err = <-errc
	glog.Info("process quit", err)

	return
}
