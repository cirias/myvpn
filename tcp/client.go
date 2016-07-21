package tcp

import (
	"net"
)

func NewClient(serverAddr string) (*Conn, error) {
	c, err := net.Dial("tcp", serverAddr)
	if err != nil {
		return nil, err
	}

	return NewConn(c), nil
}
