package protocol

import (
	"cipher"

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

type request struct {
	Secret [cipher.KeySize]byte
	Key    [cipher.KeySize]byte
}

type response struct {
	Status byte
	IP     [4]byte
	IPMask [4]byte
}
