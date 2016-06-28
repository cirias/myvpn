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
	StatusUnknowErr
	StatusInvalidSecret
	StatusInvalidProto
	StatusNoIPAddrAvaliable
)

var ErrUnknowErr = errors.New("Unknow error")
var ErrInvalidSecret = errors.New("Invalid secret")
var ErrInvalidProto = errors.New("Invalid proto")
var ErrNoIPAddrAvaliable = errors.New("No IPAddr avaliable")
var ErrIPAddrPoolFull = errors.New("IPAddrPool is full")

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
