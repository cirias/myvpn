package main

import (
	"flag"
	"log"
	"os"
)

func main() {
	var ipAddr, listenAddr, method, password string
	var debug bool

	flag.StringVar(&ipAddr, "ip-addr", "10.0.200.1/24", "server internal ip")
	flag.StringVar(&listenAddr, "listen-addr", "0.0.0.0:9222", "listening address")
	flag.StringVar(&method, "method", "aes-256-cfb", "cipher method")
	flag.StringVar(&password, "password", "milk", "password")
	flag.BoolVar(&debug, "debug", false, "enable debug")
	flag.Parse()

	if debug {
		log.SetOutput(os.Stderr)
		log.SetPrefix("[SERVER]")
		log.SetFlags(log.LstdFlags | log.Lshortfile)
	} else {
		log.SetOutput(os.Stdout)
	}

	server, err := NewServer(ipAddr)
	if err != nil {
		log.Fatalln(err)
	}
	err = server.Run(listenAddr)
	if err != nil {
		log.Fatalln(err)
	}

	return
}
