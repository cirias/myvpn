package protocol

import (
	"cipher"
	"net"

	"errors"
)

const (
	IPHeaderSize = 20
)

const (
	StatusOK byte = iota
	StatusInvalidSecret
)

var ErrUnknowErr = errors.New("Unknow error")
var ErrInvalidSecret = errors.New("Invalid secret")

type request struct {
	Secret [cipher.KeySize]byte
	Key    [cipher.KeySize]byte
}

type response struct {
	Status byte
}

type Conn interface {
	net.Conn
	ReadIPPacket(b []byte) (int, error)
}

type Listener interface {
	Accept() (Conn, error)
	Close() error
}
