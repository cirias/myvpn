package socket

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"io"
	"sync"

	"github.com/cirias/myvpn/cipher"
	"github.com/cirias/myvpn/encrypted"
	"github.com/cirias/myvpn/tcp"
	"github.com/golang/glog"
)

type Server struct {
	*tcp.Server
	clients map[id]*encrypted.Conn
	rwm     sync.RWMutex
	psk     string
}

func NewServer(psk, localAddr string) (*Server, error) {
	s, err := tcp.NewServer(localAddr)
	if err != nil {
		return nil, err
	}

	server := &Server{
		Server:  s,
		clients: make(map[id]*encrypted.Conn),
		psk:     psk,
	}

	return server, nil
}

func (s *Server) Accept() (*encrypted.Conn, error) {
	for {
		tcpConn, err := s.Server.Accept()
		if err != nil {
			return nil, err
		}

		cph, err := cipher.NewCipher([]byte(s.psk))
		if err != nil {
			return nil, err
		}
		conn := encrypted.NewConn(cph, tcpConn)

		r := bytes.NewBuffer(<-conn.OutCh)
		req := &request{}
		if err := binary.Read(r, binary.BigEndian, req); err != nil {
			return nil, err
		}
		glog.V(2).Infoln("recieve request", req)

		if err := s.handleConnect(conn, req); err != nil {
			return nil, err
		}

		return conn, nil
	}
}

func (s *Server) handleConnect(conn *encrypted.Conn, req *request) error {
	res := &response{}
	if req.Id.IsEmpty() {
		glog.V(2).Infoln("client id is empty")
		if _, err := io.ReadFull(rand.Reader, res.Id[:]); err != nil {
			return err
		}
	}

	s.register(res.Id, conn)

	res.Status = statusOK
	glog.V(2).Infoln("send response", res)
	w := &bytes.Buffer{}
	if err := binary.Write(w, binary.BigEndian, res); err != nil {
		return err
	}
	conn.InCh <- w.Bytes()

	return nil
}

func (s *Server) register(i id, conn *encrypted.Conn) {
	s.rwm.RLock()
	c, exist := s.clients[i]
	s.rwm.RUnlock()

	if exist {
		c.Close()
	}

	s.rwm.Lock()
	s.clients[i] = conn
	s.rwm.Unlock()
}
