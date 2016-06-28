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

type TCPConn struct {
	net.Conn
	cipher *cipher.Cipher
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
	conn = &TCPConn{
		Conn: c,
	}
	conn.cipher, err = cipher.NewCipher(key)
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

		req := request{}
		if err := recieve(ln.cipher, c, &req); err != nil {
			glog.Errorln("fail to recieve request", err)
			continue
		}

		if req.Secret != ln.secret {
			send(ln.cipher, c, response{Status: StatusInvalidSecret})
			continue
		}

		conn = &TCPConn{
			Conn: c,
		}

		conn.cipher, err = cipher.NewCipher(req.Key[:])
		if err != nil {
			return conn, err
		}

		res := response{
			Status: StatusOK,
		}
		send(ln.cipher, c, &res)

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
	decrypted := make([]byte, len(encrypted))
	dec.Decrypt(decrypted, encrypted)
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
	buf := &bytes.Buffer{}
	if err := binary.Write(buf, binary.BigEndian, data); err != nil {
		glog.V(3).Infoln("fail to marshal data", err)
		return err
	}
	unencypted := buf.Bytes()

	packet := make([]byte, len(iv)+len(unencypted))
	copy(packet, iv)
	enc.Encrypt(packet[len(iv):], unencypted)

	if _, err := tcpConn.Write(packet); err != nil {
		glog.V(3).Infoln("fail to write data through connection", err)
		return err
	}

	glog.V(3).Infoln("data send", data)
	return nil
}

func (conn *TCPConn) ReadIPPacket(b []byte) (n int, err error) {
	ivheader := make([]byte, cipher.IVSize+IPHeaderSize)
	if _, err = io.ReadFull(conn, ivheader); err != nil {
		glog.V(3).Infoln("fail to read iv and encrypted header", err)
		return
	}
	dec := cipher.NewDecrypter(conn.cipher, ivheader[:cipher.IVSize])
	dec.Decrypt(b[:IPHeaderSize], ivheader[cipher.IVSize:])

	n = int(binary.BigEndian.Uint16(b[2:4]))
	payload := make([]byte, n-IPHeaderSize)
	if _, err = io.ReadFull(conn, payload); err != nil {
		glog.V(3).Infoln("fail to read encrypted payload", err)
		return
	}
	dec.Decrypt(b[IPHeaderSize:n], payload)

	return
}

func (conn *TCPConn) Write(b []byte) (n int, err error) {
	defer func() {
		if r := recover(); r != nil {
			glog.Warningln(r)
			err = r.(error)
		}
	}()
	iv, err := cipher.NewIV()
	if err != nil {
		glog.V(3).Infoln("fail to create iv", err)
		return 0, err
	}

	enc := cipher.NewEncrypter(conn.cipher, iv)
	encrypted := make([]byte, len(iv)+len(b))
	copy(encrypted, iv)
	enc.Encrypt(encrypted[len(iv):], b)

	if _, err := conn.Conn.Write(encrypted); err != nil {
		glog.V(3).Infoln("fail to write data through connection", err)
		return 0, err
	}

	glog.V(3).Infoln("data writed", encrypted)
	return len(encrypted), nil
}

func (conn *TCPConn) ExternalRemoteIPAddr() net.IP {
	return conn.RemoteAddr().(*net.TCPAddr).IP
}

func (conn *TCPConn) Close() error {
	return conn.Conn.Close()
}
