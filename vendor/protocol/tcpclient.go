package protocol

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"io"
	"net"

	"cipher"

	"github.com/golang/glog"
)

type tcpClientConn struct {
	*TCPConn
	psk       string
	pskCipher *cipher.Cipher
	localAddr net.IP
	ipNetMask net.IPMask
}

func newTCPClientConn(c *net.TCPConn, psk string) (conn *tcpClientConn, err error) {
	conn = &tcpClientConn{}
	conn.TCPConn, err = newTCPConn(c)
	conn.psk = psk
	return
}

func (c *tcpClientConn) connect() (err error) {
	glog.Infoln("-1")
	key := make([]byte, cipher.KeySize)
	if _, err = io.ReadFull(rand.Reader, key); err != nil {
		glog.Errorln("fail to create key", err)
		return
	}

	req := ConnectionRequest{
		PSK: sha256.Sum256([]byte(c.psk)),
	}
	copy(req.Key[:], key)

	pskCipher, err := cipher.NewCipher([]byte(c.psk))
	if err != nil {
		glog.Errorln("fail to new cipher", err)
		return
	}
	packet := &Packet{
		Header: &Header{
			Type:   TypeConnectionRequest,
			Length: uint16(binary.Size(req)),
		},
		Body: &req,
	}

	if err = send(pskCipher, c.Conn, packet); err != nil {
		glog.Errorln("fail to send request", err)
		return
	}

	packet = &Packet{
		Header: &Header{},
	}
	if err = recieve(pskCipher, c.Conn, packet); err != nil {
		glog.Error("fail to recieve response", err)
		return
	}
	res := packet.Body.(*ConnectionResponse)

	switch res.Status {
	case StatusOK:
	case StatusUnknowErr:
		fallthrough
	case StatusInvalidSecret:
		err = ErrInvalidSecret
		return
	case StatusInvalidProto:
		err = ErrInvalidProto
		return
	case StatusNoIPAddrAvaliable:
		err = ErrNoIPAddrAvaliable
		return
	default:
		err = ErrUnknowErr
		return
	}

	c.localAddr = res.IP[:]
	c.ipNetMask = res.IPMask[:]
	c.cipher, err = cipher.NewCipher(key)
	return nil
}

func (conn *tcpClientConn) IPNetMask() net.IPMask {
	return conn.ipNetMask
}

func (conn *tcpClientConn) LocalIPAddr() net.IP {
	return conn.localAddr
}

func (conn *tcpClientConn) ExternalRemoteIPAddr() net.IP {
	return conn.RemoteAddr().(*net.TCPAddr).IP
}
