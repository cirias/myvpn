package protocol

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"io"
	"net"

	"cipher"

	"github.com/golang/glog"
)

func DialTCP(secret, remoteAddr string) (conn *TCPConn, err error) {
	c, err := net.Dial("tcp", remoteAddr)
	if err != nil {
		glog.Errorln("fail to dial to server", err, remoteAddr)
		return
	}

	cph, err := cipher.NewCipher([]byte(secret))
	if err != nil {
		return
	}

	key := make([]byte, cipher.KeySize)
	if _, err = io.ReadFull(rand.Reader, key); err != nil {
		return
	}

	req := request{
		Secret: sha256.Sum256([]byte(secret)),
	}
	copy(req.Key[:], key)

	if err = send(cph, c, &req); err != nil {
		glog.Error("fail to send request", err)
		return
	}

	res := response{}
	if err = recieve(cph, c, &res); err != nil {
		glog.Error("fail to recieve response", err)
		return
	}

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

	// Initialize conn
	conn = &TCPConn{}
	conn.Conn = c
	conn.cipher, err = cipher.NewCipher(key)
	return
}

func ListenTCP(secret, localAddr string, internalIP net.IP, ipNet *net.IPNet) (ln *TCPListener, err error) {
	ln = &TCPListener{}
	ln.Listener, err = net.Listen("tcp", localAddr)
	if err != nil {
		return
	}

	ln.cipher, err = cipher.NewCipher([]byte(secret))
	if err != nil {
		return
	}

	ln.ipAddrPool = NewIPAddrPool(internalIP, ipNet)
	ln.secret = sha256.Sum256([]byte(secret))

	return

}

type TCPListener struct {
	net.Listener
	cipher     *cipher.Cipher
	ipAddrPool IPAddrPool
	secret     [cipher.KeySize]byte
}

// Currently one client acceping will block all other clients' request
func (ln *TCPListener) Accept() (conn *TCPConn, err error) {
	var c net.Conn
	for {
		c, err = ln.Listener.Accept()
		if err != nil {
			glog.Errorln("fail to accept from listener", err)
			return conn, err
		}

		req := request{}
		if err := recieve(ln.cipher, c, &req); err != nil {
			glog.Errorln("fail to recieve request", err)
			continue
		}

		if req.Secret != ln.secret {
			send(ln.cipher, c, response{Status: StatusInvalidSecret})
			continue
		}

		conn = &TCPConn{}
		ip, err := ln.ipAddrPool.Get()
		if err != nil {
			send(ln.cipher, c, response{Status: StatusNoIPAddrAvaliable})
			continue
		}

		conn.cipher, err = cipher.NewCipher(req.Key[:])
		if err != nil {
			return conn, err
		}

		res := response{
			Status: StatusOK,
		}
		copy(res.IP[:], ip.IP.To4())
		copy(res.IPMask[:], ip.Mask)
		send(ln.cipher, c, &res)

		conn.Conn = c

		return conn, err
	}
}

func recieve(cph *cipher.Cipher, tcpConn net.Conn, data interface{}) (err error) {
	iv := make([]byte, cipher.IVSize)
	if _, err = io.ReadFull(tcpConn, iv); err != nil {
		glog.V(3).Infoln("fail to read iv", err)
		return
	}
	dec := cipher.NewDecrypter(cph, iv)
	encrypted := make([]byte, binary.Size(data))
	if _, err = io.ReadFull(tcpConn, encrypted); err != nil {
		glog.V(3).Infoln("fail to read encrypted data", err)
		return
	}
	decrypted := make([]byte, binary.Size(encrypted))
	dec.Decrypt(encrypted, decrypted)
	if err = binary.Read(bytes.NewBuffer(decrypted), binary.BigEndian, data); err != nil {
		glog.V(3).Infoln("fail to unmarshal data", err)
		return
	}
	glog.V(3).Infoln("data recieved", data)
	return
}

func send(cph *cipher.Cipher, tcpConn net.Conn, data interface{}) error {
	iv, err := cipher.NewIV()
	if err != nil {
		glog.V(3).Infoln("fail to create iv", err)
		return err
	}

	enc := cipher.NewEncrypter(cph, iv)
	unencypted := make([]byte, binary.Size(data))
	if err := binary.Write(bytes.NewBuffer(unencypted), binary.BigEndian, data); err != nil {
		glog.V(3).Infoln("fail to marshal data", err)
		return err
	}

	packet := make([]byte, len(iv)+len(unencypted))
	enc.Encrypt(unencypted, packet[len(iv):])
	copy(iv, packet)

	if _, err := tcpConn.Write(packet); err != nil {
		glog.V(3).Infoln("fail to write data through connection", err)
		return err
	}

	glog.V(3).Infoln("data send", data)
	return nil
}

type TCPConn struct {
	net.Conn
	cipher *cipher.Cipher
}

func (conn *TCPConn) ReadPacket(b []byte) (n int, err error) {
	ivheader := make([]byte, cipher.IVSize+IPHeaderSize)
	if _, err = io.ReadFull(conn, ivheader); err != nil {
		glog.V(3).Infoln("fail to read iv and encrypted header", err)
		return
	}
	dec := cipher.NewDecrypter(conn.cipher, ivheader[:cipher.IVSize])
	dec.Decrypt(ivheader[cipher.IVSize:], b[:IPHeaderSize])

	n = int(binary.BigEndian.Uint16(b[2:4]))
	payload := make([]byte, n-IPHeaderSize)
	if _, err = io.ReadFull(conn, payload); err != nil {
		glog.V(3).Infoln("fail to read encrypted payload", err)
		return
	}
	dec.Decrypt(payload, b[IPHeaderSize:n])

	return
}
