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

func main() {
	var network, secret, listenAddr, ipnet, hookDir string

	flag.StringVar(&network, "network", "udp", "network of transport layer")
	flag.StringVar(&secret, "secret", "", "secret")
	flag.StringVar(&listenAddr, "listen-addr", "0.0.0.0:9525", "listening address")
	flag.StringVar(&ipnet, "ipnet", "10.0.200.1/24", "internal ip net")
	flag.StringVar(&hookDir, "hook-dir", "/etc/myvpn", "hook directory")
	flag.Parse()

	ip, ipNet, err := net.ParseCIDR(ipnet)
	if err != nil {
		glog.Fatalln(err)
	}
	ln, err := protocol.Listen(network, secret, listenAddr, ip, ipNet)
	if err != nil {
		glog.Fatalln(err)
	}

	tun, err := tun.NewTun(&net.IPNet{ip, ipNet.Mask}, hookDir)
	if err != nil {
		glog.Fatalln(err)
	}

	err = tun.Up(listenAddr)
	if err != nil {
		glog.Fatalln(err)
	}
	defer tun.Down(listenAddr)

	s := NewServer(tun, ipNet)
	defer s.Close()

	errc := make(chan error)

	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt, os.Kill, syscall.SIGTERM)

		s := <-c
		errc <- errors.New("signal: " + s.String())
	}()

	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				glog.Errorln("Accept", err)
			}

			go s.Handle(c)
		}
	}()

	err = <-errc
	glog.Info("Process quit", err)

	return
}
