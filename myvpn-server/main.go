package main

import (
	"flag"
	"github.com/golang/glog"
	"time"
)

var clientTimeout time.Duration

func main() {
	var ipAddr, listenAddr, password, timeout string

	flag.StringVar(&ipAddr, "ip-addr", "10.0.200.1/24", "server internal ip")
	flag.StringVar(&listenAddr, "listen-addr", "0.0.0.0:9525", "listening address")
	flag.StringVar(&password, "password", "", "password")
	flag.StringVar(&timeout, "timeout", "12h", "client timeout")
	flag.Parse()

	var err error

	clientTimeout, err = time.ParseDuration(timeout)
	if err != nil {
		glog.Fatalln(err)
	}

	server, err := NewServer(ipAddr, password)
	if err != nil {
		glog.Fatalln(err)
	}

	if err = server.Run(listenAddr); err != nil {
		glog.Fatalln(err)
	}

	return
}
