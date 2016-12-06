package main

import (
	"flag"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/cirias/myvpn/socket"
	"github.com/cirias/myvpn/tun"
	"github.com/cirias/myvpn/vpn"
	"github.com/golang/glog"
)

func main() {
	var network, secret, listenAddr, ipnet, upScript, downScript string

	flag.StringVar(&network, "network", "tcp", "network of transport layer")
	flag.StringVar(&secret, "secret", "", "secret")
	flag.StringVar(&listenAddr, "listen-addr", "0.0.0.0:9525", "listening address")
	flag.StringVar(&ipnet, "ipnet", "10.0.200.1/24", "internal ip net")
	flag.StringVar(&upScript, "up-script", "./if-up.sh", "up shell script file path")
	flag.StringVar(&downScript, "down-script", "./if-down.sh", "down shell script file path")
	flag.Parse()

	sockServer, err := socket.NewServer(secret, listenAddr)
	if err != nil {
		glog.Fatalln(err)
	}
	glog.Infoln("start listening")

	tun, err := tun.NewTUN("")
	if err != nil {
		glog.Fatalln(err)
	}
	defer tun.Close()

	ip, ipNet, err := net.ParseCIDR(ipnet)
	if err != nil {
		glog.Fatalln(err)
	}

	err = tun.Run(upScript, (&net.IPNet{ip, ipNet.Mask}).String(), listenAddr)
	if err != nil {
		glog.Fatalln(err)
	}
	defer tun.Run(downScript, (&net.IPNet{ip, ipNet.Mask}).String(), listenAddr)
	glog.Infoln(tun.Name(), " is ready")

	s, err := vpn.NewServer(tun, ipnet)
	if err != nil {
		glog.Fatalln(err)
	}
	defer s.Close()

	signalCh := make(chan os.Signal)
	go func() {
		signal.Notify(signalCh, os.Interrupt, os.Kill, syscall.SIGTERM)
	}()

	go func() {
		for {
			sock, err := sockServer.Accept()
			if err != nil {
				glog.Errorln("fail to accept", err)
			}

			go s.Handle(sock)
		}
	}()
	glog.Infoln("waiting client")

	sgn := <-signalCh
	glog.Infoln("process quit", sgn)

	return
}
