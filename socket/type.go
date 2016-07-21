package socket

import (
	"errors"

	"github.com/cirias/myvpn/cipher"
)

var ErrUnknowErr = errors.New("Unknow error")
var ErrInvalidSecret = errors.New("Invalid secret")

type id [8]byte

func (i id) IsEmpty() bool {
	var emptyId id
	return i == emptyId
}

const (
	statusOK byte = iota
	statusInvalidSecret
	statusNoIPAvaliable
)

type request struct {
	PSK    [cipher.KeySize]byte
	NewPSK [cipher.KeySize]byte
	Id     id
}

type response struct {
	Status byte
	Id     id
}
