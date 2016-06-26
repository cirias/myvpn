package protocol

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"io"
	"net"
	"reflect"
	"time"

	"cipher"

	"github.com/golang/glog"
)

func DialTCP(psk, remoteAddr string) (conn *tcpClientConn, err error) {
	c, err := net.Dial("tcp", remoteAddr)
	if err != nil {
		glog.Errorln("fail to dial to server", err, remoteAddr)
		return
	}

	if conn, err = newTCPClientConn(c.(*net.TCPConn), psk); err != nil {
		return
	}

	if err = conn.connect(); err != nil {
		return
	}

	go func() {
		for {
			if err := conn.readWrite(); err == nil {
				break
			}

			for {
				if err = conn.reconnect(); err != nil {
					glog.Errorln("fail to reconnect", err)
					time.Sleep(10 * time.Second)
					continue
				}

				break
			}
		}
	}()

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
	ln.psk = sha256.Sum256([]byte(secret))

	return

}

type TCPListener struct {
	net.Listener
	cipher     *cipher.Cipher
	ipAddrPool IPAddrPool
	psk        [cipher.KeySize]byte
}

func (ln *TCPListener) Accept() (ServerConn, error) {
	return ln.AcceptTCP()
}

// Currently one client acceping will block all other clients' request
func (ln *TCPListener) AcceptTCP() (conn *tcpServerConn, err error) {
	var c net.Conn
	for {
		c, err = ln.Listener.Accept()
		if err != nil {
			glog.Errorln("fail to accept from listener, quit", err)
			return conn, err
		}

		if conn, err = newTCPServerConn(c.(*net.TCPConn), ln); err != nil {
			return
		}

		packet := &Packet{Header: &Header{}}
		if err := recieve(ln.cipher, conn.Conn, packet); err != nil {
			glog.Errorln("fail to recieve request, drop", err)
			c.Close()
			continue
		}
		req := packet.Body.(*ConnectionRequest)

		switch packet.Header.Type {
		case TypeConnectionRequest:
			res, err := conn.handleConnectionRequest(req)
			if err != nil {
				return nil, err
			}

			err = send(ln.cipher, conn.Conn, &Packet{&Header{Type: TypeConnectionResponse}, res})

			go func() {
				if err := conn.readWrite(); err != nil {
					conn.Conn.Close()
					// TODO
				}
			}()

			return conn, err
		case TypeReconnectionRequest:
			// TODO
			continue
		default:
			glog.Warningln("invalid request, drop", err)
			c.Close()
			continue
		}
	}
}

func recieve(cph *cipher.Cipher, r io.Reader, p *Packet) (err error) {
	iv := make([]byte, cipher.IVSize)
	if _, err = io.ReadFull(r, iv); err != nil {
		glog.Errorln("fail to read iv", err)
		return
	}

	dec := cipher.NewDecrypter(cph, iv)
	if err = decode(dec, r, p.Header); err != nil {
		glog.Errorln("fail to decode header ", err)
		return
	}

	switch p.Header.Type {
	case TypeConnectionRequest:
		p.Body = &ConnectionRequest{}
	case TypeConnectionResponse:
		p.Body = &ConnectionResponse{}
	case TypeReconnectionRequest:
		p.Body = &ReconnectionRequest{}
	case TypeReconnectionResponse:
		p.Body = &ReconnectionResponse{}
	case TypeTraffic:
		p.Body = make([]byte, p.Header.Length)
	case TypeQuitRequest:
		p.Body = &QuitRequest{}
	case TypeQuitResponse:
		p.Body = &QuitResponse{}
	default:
		return ErrInvalidDataType
	}

	if err = decode(dec, r, p.Body); err != nil {
		glog.Errorln("fail to decode body ", err)
		return
	}

	glog.V(3).Infoln("body recieved", p.Body)
	return
}

func decode(dec *cipher.Decrypter, r io.Reader, data interface{}) (err error) {
	if data == nil {
		return ErrUninitializedData
	}

	encrypted := make([]byte, binary.Size(data))
	if _, err = io.ReadFull(r, encrypted); err != nil {
		return
	}
	t := reflect.TypeOf(data)
	if t.Kind() == reflect.Slice && t.Elem().Kind() == reflect.Uint8 { // data's type is []byte
		dec.Decrypt(data.([]byte), encrypted)
	} else {
		decrypted := make([]byte, len(encrypted))
		dec.Decrypt(decrypted, encrypted)
		err = binary.Read(bytes.NewBuffer(decrypted), binary.BigEndian, data)
	}
	return
}

func send(cph *cipher.Cipher, w io.Writer, p *Packet) error {
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
	if err := encode(enc, w, p.Header); err != nil {
		glog.Errorln("fail to encode header to connection", err)
		return err
	}

	if err := encode(enc, w, p.Body); err != nil {
		glog.Errorln("fail to encode body to connection", err)
		return err
	}

	glog.V(3).Infoln("body send", p.Body)
	return nil
}

func encode(enc *cipher.Encrypter, w io.Writer, data interface{}) (err error) {
	if data == nil {
		return ErrUninitializedData
	}

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
