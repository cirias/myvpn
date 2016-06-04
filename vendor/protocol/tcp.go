package protocol

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"io"
	"net"

	"cipher"

	"github.com/golang/glog"
)

type Conn interface {
	ReadPacket(b []byte) (int, error)
}

type Listener interface {
	Accept() (Conn, error)
	Close() error
}

func ListenTCP(secret, localAddr string, internalIP net.IP, ipNet *net.IPNet) (ln Listener, err error) {
	tcpln := &TCPListener{}
	tcpln.Listener, err = net.Listen("tcp", localAddr)
	if err != nil {
		return
	}

	tcpln.cipher, err = cipher.NewCipher([]byte(secret))
	if err != nil {
		return
	}

	tcpln.ipAddrPool = NewIPAddrPool(internalIP, ipNet)
	tcpln.secret = sha256.Sum256([]byte(secret))

	ln = tcpln
	return
}

type TCPListener struct {
	net.Listener
	cipher     *cipher.Cipher
	ipAddrPool IPAddrPool
	secret     [cipher.KeySize]byte
}

// Currently one client acceping will block all other clients' request
func (ln *TCPListener) Accept() (conn Conn, err error) {
	var c net.Conn
	for {
		c, err = ln.Listener.Accept()
		if err != nil {
			glog.Errorln("fail to accept from listener", err)
			return conn, err
		}

		req, err := ln.recieve(c)
		if err != nil {
			glog.Errorln("fail to recieve request", err)
			continue
		}

		if req.Secret != ln.secret {
			ln.send(c, &response{Status: StatusInvalidSecret})
			continue
		}

		tcpConn := &TCPConn{}
		ip, err := ln.ipAddrPool.Get()
		if err != nil {
			ln.send(c, &response{Status: StatusNoIPAddrAvaliable})
			continue
		}

		tcpConn.cipher, err = cipher.NewCipher(req.Key[:])
		if err != nil {
			return conn, err
		}

		res := &response{
			Status: StatusOK,
		}
		copy(res.IP[:], ip.IP.To4())
		copy(res.IPMask[:], ip.Mask)
		ln.send(c, res)

		tcpConn.Conn = c
		conn = tcpConn

		return conn, err
	}
}

func (ln *TCPListener) recieve(tcpConn net.Conn) (req *request, err error) {
	iv := make([]byte, cipher.IVSize)
	if _, err = io.ReadFull(tcpConn, iv); err != nil {
		glog.V(3).Infoln("fail to read iv", err)
		return
	}
	dec := cipher.NewDecrypter(ln.cipher, iv)
	req = &request{}
	encrypted := make([]byte, binary.Size(*req))
	if _, err = io.ReadFull(tcpConn, encrypted); err != nil {
		glog.V(3).Infoln("fail to read encrypted request", err)
		return
	}
	decrypted := make([]byte, binary.Size(encrypted))
	dec.Decrypt(encrypted, decrypted)
	if err = binary.Read(bytes.NewBuffer(decrypted), binary.BigEndian, req); err != nil {
		glog.V(3).Infoln("fail to unmarshal request", err)
		return
	}
	glog.V(3).Infoln("request recieved", *req)
	return
}

func (ln *TCPListener) send(tcpConn net.Conn, res *response) error {
	iv, err := cipher.NewIV()
	if err != nil {
		glog.V(3).Infoln("fail to create iv", err)
		return err
	}

	enc := cipher.NewEncrypter(ln.cipher, iv)
	unencypted := make([]byte, binary.Size(*res))
	if err := binary.Write(bytes.NewBuffer(unencypted), binary.BigEndian, res); err != nil {
		glog.V(3).Infoln("fail to marshal response", err)
		return err
	}

	packet := make([]byte, len(iv)+len(unencypted))
	enc.Encrypt(unencypted, packet[len(iv):])
	copy(iv, packet)

	if _, err := tcpConn.Write(packet); err != nil {
		glog.V(3).Infoln("fail to write response to c", err)
		return err
	}

	glog.V(3).Infoln("reponse send", res)
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
