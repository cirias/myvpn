package main

import (
	"flag"
	"github.com/golang/glog"
	"time"
)

var enableHeartbeat bool
var timeout time.Duration
var interval time.Duration

func main() {
	var serverAddr, password, intervalStr, timeoutStr string

	flag.StringVar(&serverAddr, "server-addr", "127.0.0.1:9525", "server address")
	flag.StringVar(&password, "password", "", "password")
	flag.BoolVar(&enableHeartbeat, "enable-heartbeat", false, "whether to enable heartbeat")
	flag.StringVar(&intervalStr, "interval", "5s", "interval of heartbeat")
	flag.StringVar(&timeoutStr, "timeout", "20s", "timeout of heartbeat")
	flag.Parse()

	var err error
	interval, err = time.ParseDuration(intervalStr)
	if err != nil {
		glog.Fatalln(err)
	}

	timeout, err = time.ParseDuration(timeoutStr)
	if err != nil {
		glog.Fatalln(err)
	}

	client, err := NewClient(serverAddr, password)
	if err != nil {
		glog.Fatalln(err)
	}
	err = client.Run()
	if err != nil {
		glog.Fatalln(err)
	}

	return
}
