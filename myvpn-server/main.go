package main

import (
	"flag"
	"github.com/golang/glog"
)

func main() {
	var ipAddr, listenAddr, password string

	flag.StringVar(&ipAddr, "ip-addr", "10.0.200.1/24", "server internal ip")
	flag.StringVar(&listenAddr, "listen-addr", "0.0.0.0:9525", "listening address")
	flag.StringVar(&password, "password", "", "password")
	flag.Parse()

	server, err := NewServer(ipAddr, password)
	if err != nil {
		glog.Fatalln(err)
	}
	err = server.Run(listenAddr)
	if err != nil {
		glog.Fatalln(err)
	}

	return
}
