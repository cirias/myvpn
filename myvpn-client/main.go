package main

import (
	"errors"
	"flag"
	"os"
	"os/signal"
	"syscall"

	"protocol"
	"tun"

	"github.com/golang/glog"
)

func main() {
	var network, secret, serverAddr, hookDir string

	flag.StringVar(&network, "network", "udp", "network of transport layer")
	flag.StringVar(&secret, "secret", "", "secret")
	flag.StringVar(&serverAddr, "server-addr", "127.0.0.1:9525", "server address")
	flag.StringVar(&hookDir, "hook-dir", "/etc/myvpn", "hook directory")
	flag.Parse()

	conn, err := protocol.Dial(network, secret, serverAddr)
	if err != nil {
		glog.Fatalln(err)
	}

	tun, err := tun.NewTun(conn.IPNet, hookDir)
	if err != nil {
		glog.Fatalln(err)
	}

	err = tun.Up(serverAddr)
	if err != nil {
		glog.Fatalln(err)
	}
	defer tun.Down(serverAddr)

	errc := make(chan error)

	go func() {
		b := make([]byte, 65535)
		for {
			n, err := conn.Read(b)
			if err != nil {
				errc <- err
				continue
			}

			_, err = tun.Write(b[:n])
			if err != nil {
				errc <- err
			}
		}
	}()

	go func() {
		b := make([]byte, 65535)
		for {
			n, err := tun.Read(b)
			if err != nil {
				errc <- err
				continue
			}

			_, err = conn.Write(b[:n])
			if err != nil {
				errc <- err
			}
		}
	}()

	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt, os.Kill, syscall.SIGTERM)

		s := <-c
		errc <- errors.New("signal: " + s.String())
	}()

	err = <-errc
	if err != nil {
		glog.Infoln("Process quit for:", err)
	}

	return
}
