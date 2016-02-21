package main

import (
	"time"

	"protocol"
)

type Client struct {
	*protocol.Conn
	timestamp time.Time
	quit      chan struct{}
}
