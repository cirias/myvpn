package main

import (
	"protocol"
)

type Client struct {
	protocol.ServerConn
	quit chan struct{}
}
