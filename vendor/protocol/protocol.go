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

const (
	TypeConnectionRequest byte = iota
	TypeConnectionResponse
	TypeReconnectionRequest
	TypeReconnectionResponse
	TypeTraffic
	TypeQuitRequest
	TypeQuitResponse
)

var ErrUnknowErr = errors.New("Unknow error")
var ErrInvalidSecret = errors.New("Invalid secret")
var ErrInvalidProto = errors.New("Invalid proto")
var ErrNoIPAddrAvaliable = errors.New("No IPAddr avaliable")
var ErrIPAddrPoolFull = errors.New("IPAddrPool is full")
var ErrInvalidDataType = errors.New("Invalid data type")
var ErrUninitializedData = errors.New("Uninitialized data")

type (
	Header struct {
		Type   byte
		Length uint16
	}
	Packet struct {
		Header *Header
		Body   interface{}
	}
	ConnectionRequest struct {
		PSK [cipher.KeySize]byte
		Key [cipher.KeySize]byte
	}
	ConnectionResponse struct {
		Status byte
		IP     [4]byte
		IPMask [4]byte
	}
	ReconnectionRequest struct {
	}
	ReconnectionResponse struct {
	}
	QuitRequest struct {
	}
	QuitResponse struct {
	}
)

type ClientConn interface {
	net.Conn
	IPNetMask() net.IPMask
	LocalIPAddr() net.IP
	ExternalRemoteIPAddr() net.IP
}

type ServerConn interface {
	net.Conn
	RemoteIPAddr() net.IP
}

type Listener interface {
	Accept() (ServerConn, error)
	Close() error
}
