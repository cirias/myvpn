package main

import (
	"flag"
	"github.com/golang/glog"
)

func main() {
	var ipAddr, listenAddr, password string
	var portBase int

	flag.StringVar(&ipAddr, "ip-addr", "10.0.200.1/24", "server internal ip")
	flag.StringVar(&listenAddr, "listen-addr", "0.0.0.0:9525", "listening address")
	flag.StringVar(&password, "password", "", "password")
	flag.IntVar(&portBase, "port-base", 61000, "base of the port pool")
	flag.Parse()

	var err error

	glog.V(4).Infoln("NewServer")
	server, err := NewServer(ipAddr, password, portBase)
	if err != nil {
		glog.Fatalln(err)
	}

	glog.V(4).Infoln("server.Run")
	if err = server.Run(listenAddr); err != nil {
		glog.Fatalln(err)
	}

	return
}
