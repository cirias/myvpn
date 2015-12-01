package main

import (
	"flag"
	"github.com/golang/glog"
	//"os"
)

func main() {
	var serverAddr, password string

	flag.StringVar(&serverAddr, "server-addr", "127.0.0.1:9525", "server address")
	flag.StringVar(&password, "password", "", "password")
	flag.Parse()

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
