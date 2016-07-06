package protocol

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"io"
	"net"
	"reflect"

	"github.com/cirias/myvpn/cipher"
	"github.com/golang/glog"
)

type TCPConn struct {
	net.Conn
	cipher *cipher.Cipher
}

func newTCPConn(c net.Conn, key []byte) (conn *TCPConn, err error) {
	conn = &TCPConn{
		Conn: c,
	}
	conn.cipher, err = cipher.NewCipher(key)
	return
}

func newRequest(key string) (*request, error) {
	req := &request{}
	nextkey := make([]byte, cipher.KeySize)
	if _, err := io.ReadFull(rand.Reader, nextkey); err != nil {
		return nil, err
	}

	req.Secret = sha256.Sum256([]byte(key))
	copy(req.Key[:], nextkey)
	return req, nil
}

func handleResponse(res *response) (err error) {
	switch res.Status {
	case StatusOK:
	case StatusInvalidSecret:
		err = ErrInvalidSecret
	default:
		err = ErrUnknowErr
	}
	return
}

func (ln *TCPListener) handleRequest(req *request) *response {
	res := &response{}
	if req.Secret == ln.secret {
		res.Status = StatusOK
	} else {
		res.Status = StatusInvalidSecret
	}
	return res
}

func DialTCP(secret, remoteAddr string) (conn *TCPConn, err error) {
	c, err := net.Dial("tcp", remoteAddr)
	if err != nil {
		glog.Errorln("fail to dial to server", err, remoteAddr)
		return
	}

	// in case idle connection session being cleaned by NAT server
	if err = c.(*net.TCPConn).SetKeepAlive(true); err != nil {
		glog.Errorln("fail to set keep alive", err)
		return
	}

	cph, err := cipher.NewCipher([]byte(secret))
	if err != nil {
		return nil, err
	}

	req, err := newRequest(secret)
	if err != nil {
		return
	}

	if err = send(cph, c, req); err != nil {
		glog.Error("fail to send request", err)
		return
	}

	res := &response{}
	if err = recieve(cph, c, res); err != nil {
		glog.Error("fail to recieve response", err)
		return
	}

	if err = handleResponse(res); err != nil {
		return
	}

	conn, err = newTCPConn(c, req.Key[:])
	return
}

func ListenTCP(secret, localAddr string) (ln *TCPListener, err error) {
	ln = &TCPListener{}
	ln.Listener, err = net.Listen("tcp", localAddr)
	if err != nil {
		return
	}

	ln.cipher, err = cipher.NewCipher([]byte(secret))
	if err != nil {
		return
	}

	ln.secret = sha256.Sum256([]byte(secret))

	return
}

type TCPListener struct {
	net.Listener
	cipher *cipher.Cipher
	secret [cipher.KeySize]byte
}

func (ln *TCPListener) Accept() (Conn, error) {
	return ln.AcceptTCP()
}

// Currently one client acceping will block all other clients' request
func (ln *TCPListener) AcceptTCP() (conn *TCPConn, err error) {
	var c net.Conn
	for {
		c, err = ln.Listener.Accept()
		if err != nil {
			glog.Errorln("fail to accept from listener", err)
			return conn, err
		}

		// in case idle connection session being cleaned by NAT server
		if err = c.(*net.TCPConn).SetKeepAlive(true); err != nil {
			glog.Errorln("fail to set keep alive", err)
			return
		}

		req := &request{}
		if err := recieve(ln.cipher, c, req); err != nil {
			glog.Errorln("fail to recieve request", err)
			continue
		}

		res := ln.handleRequest(req)
		if err = send(ln.cipher, c, res); err != nil {
			return nil, err
		}

		conn, err = newTCPConn(c, req.Key[:])
		return conn, err
	}
}

func (conn *TCPConn) Read(b []byte) (n int, err error) {
	return read(conn.cipher, conn.Conn, b)
}

func (conn *TCPConn) Write(b []byte) (n int, err error) {
	return write(conn.cipher, conn.Conn, b)
}

func (conn *TCPConn) ExternalRemoteIPAddr() net.IP {
	return conn.RemoteAddr().(*net.TCPAddr).IP
}

func (conn *TCPConn) Close() error {
	return conn.Conn.Close()
}

func read(cph *cipher.Cipher, r io.Reader, b []byte) (n int, err error) {
	iv := make([]byte, cipher.IVSize)
	if _, err = io.ReadFull(r, iv); err != nil {
		glog.Errorln("fail to read iv", err)
		return
	}

	dec := cipher.NewDecrypter(cph, iv)

	var length uint16
	if err = decode(dec, r, &length); err != nil {
		glog.Errorln("fail to decode length ", err)
		return
	}
	n = int(length)

	if err = decode(dec, r, b[:n]); err != nil {
		glog.Errorln("fail to decode body ", err)
		return
	}

	glog.V(3).Infoln("body recieved", b[:n])
	return
}

func recieve(cph *cipher.Cipher, r io.Reader, data interface{}) (err error) {
	iv := make([]byte, cipher.IVSize)
	if _, err = io.ReadFull(r, iv); err != nil {
		glog.Errorln("fail to read iv", err)
		return
	}

	dec := cipher.NewDecrypter(cph, iv)

	if err = decode(dec, r, data); err != nil {
		glog.Errorln("fail to decode body ", err)
		return
	}

	glog.V(3).Infoln("body recieved", data)
	return
}

func decode(dec *cipher.Decrypter, r io.Reader, data interface{}) (err error) {
	encrypted := make([]byte, binary.Size(data))
	if _, err = io.ReadFull(r, encrypted); err != nil {
		return
	}
	t := reflect.TypeOf(data)
	if t.Kind() == reflect.Slice && t.Elem().Kind() == reflect.Uint8 { // if data's type is []byte
		dec.Decrypt(data.([]byte), encrypted)
	} else {
		decrypted := make([]byte, len(encrypted))
		dec.Decrypt(decrypted, encrypted)
		err = binary.Read(bytes.NewBuffer(decrypted), binary.BigEndian, data)
	}
	return
}

func write(cph *cipher.Cipher, w io.Writer, b []byte) (n int, err error) {
	iv, err := cipher.NewIV()
	if err != nil {
		glog.Errorln("fail to create iv", err)
		return
	}

	if _, err = w.Write(iv); err != nil {
		glog.Errorln("fail to write iv to connection", err)
		return
	}

	length := uint16(len(b))

	enc := cipher.NewEncrypter(cph, iv)

	if err = encode(enc, w, length); err != nil {
		glog.Errorln("fail to encode header to connection", err)
		return
	}

	if err = encode(enc, w, b); err != nil {
		glog.Errorln("fail to encode body to connection", err)
		return
	}

	n = len(b)

	glog.V(3).Infoln("body send", b)
	return
}

func send(cph *cipher.Cipher, w io.Writer, data interface{}) error {
	iv, err := cipher.NewIV()
	if err != nil {
		glog.Errorln("fail to create iv", err)
		return err
	}

	if _, err := w.Write(iv); err != nil {
		glog.Errorln("fail to write iv to connection", err)
		return err
	}

	enc := cipher.NewEncrypter(cph, iv)

	if err := encode(enc, w, data); err != nil {
		glog.Errorln("fail to encode body to connection", err)
		return err
	}

	glog.V(3).Infoln("body send", data)
	return nil
}

func encode(enc *cipher.Encrypter, w io.Writer, data interface{}) (err error) {
	var unencypted []byte
	t := reflect.TypeOf(data)
	if t.Kind() == reflect.Slice && t.Elem().Kind() == reflect.Uint8 { // data's type is []byte
		unencypted = data.([]byte)
	} else {
		buf := &bytes.Buffer{}
		if err = binary.Write(buf, binary.BigEndian, data); err != nil {
			return
		}
		unencypted = buf.Bytes()
	}

	encrypted := make([]byte, len(unencypted))
	enc.Encrypt(encrypted, unencypted)

	_, err = w.Write(encrypted)

	return
}
