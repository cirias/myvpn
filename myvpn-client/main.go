package main

import (
	"flag"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/cirias/myvpn/socket"
	"github.com/cirias/myvpn/tun"
	"github.com/cirias/myvpn/vpn"
	"github.com/golang/glog"
)

func main() {
	var network, secret, serverAddr, upScript, downScript, addrScript string

	flag.StringVar(&network, "network", "udp", "network of transport layer")
	flag.StringVar(&secret, "secret", "", "secret")
	flag.StringVar(&serverAddr, "server-addr", "127.0.0.1:9525", "server address")
	flag.StringVar(&upScript, "up-script", "./if-up.sh", "up shell script file path")
	flag.StringVar(&downScript, "down-script", "./if-down.sh", "down shell script file path")
	flag.StringVar(&addrScript, "addr-script", "./if-addr.sh", "script file path for set ip address")
	flag.Parse()

	sock, err := socket.NewClient(secret, serverAddr)
	if err != nil {
		glog.Fatalln(err)
	}
	defer sock.Close()

	tun, err := tun.NewTUN("")
	if err != nil {
		glog.Fatalln(err)
	}
	defer tun.Close()

	client, err := vpn.NewClient(tun, sock)
	if err != nil {
		glog.Fatalln(err)
	}
	defer client.Close()

	glog.Infoln("connected to server")

	err = tun.Run(addrScript, (&net.IPNet{client.IP, client.IPMask}).String())
	if err != nil {
		glog.Fatalln(err)
	}

	ips, err := net.LookupIP(strings.Split(serverAddr, ":")[0])
	if err != nil {
		glog.Fatalln(err)
	}
	glog.Infoln("server ip: ", ips)
	err = tun.Run(upScript, ips[0].String())
	if err != nil {
		glog.Fatalln(err)
	}
	defer tun.Run(downScript, (&net.IPNet{client.IP, client.IPMask}).String(), ips[0].String())

	glog.Infoln(tun.Name(), "is ready")

	signalCh := make(chan os.Signal)
	go func() {
		signal.Notify(signalCh, os.Interrupt, os.Kill, syscall.SIGTERM)
	}()

	sgn := <-signalCh
	glog.Infoln("process quit", sgn)

	return
}
