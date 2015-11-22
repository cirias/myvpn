package main

import (
	"flag"
	"log"
	"os"
)

func init() {
	log.SetOutput(os.Stdout)
}

func main() {
	var serverAddr, method, password string
	var debug bool

	flag.StringVar(&serverAddr, "server-addr", "52.68.80.84:9222", "server address")
	flag.StringVar(&method, "method", "aes-256-cfb", "cipher method")
	flag.StringVar(&password, "password", "milk", "password")
	flag.BoolVar(&debug, "debug", false, "enable debug")
	flag.Parse()

	if debug {
		log.SetOutput(os.Stderr)
		log.SetPrefix("[CLIENT]")
		log.SetFlags(log.LstdFlags | log.Lshortfile)
	} else {
		log.SetOutput(os.Stdout)
	}

	client, err := NewClient(serverAddr, password)
	if err != nil {
		log.Fatalln(err)
	}
	err = client.Run()
	if err != nil {
		log.Fatalln(err)
	}

	return
}
