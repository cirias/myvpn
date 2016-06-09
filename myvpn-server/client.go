package main

import (
	"protocol"
)

type Client struct {
	protocol.Conn
	quit chan struct{}
}
