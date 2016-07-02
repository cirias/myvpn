package socket

import (
	"encoding/binary"
	"sync"

	"protocol"

	"github.com/golang/glog"
)

type Server struct {
	clients  map[idType]*Socket
	listener protocol.Listener
	rwm      sync.RWMutex
}

func NewServer(psk, localAddr string) (*Server, error) {
	ln, err := protocol.ListenTCP(psk, localAddr)
	if err != nil {
		return nil, err
	}

	s := &Server{
		clients:  make(map[idType]*Socket),
		listener: ln,
	}

	return s, nil
}

func (s Server) Close() error {
	return s.listener.Close()
}

func (s *Server) Accept() (*Socket, error) {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			return nil, err
		}

		req := &request{}
		if err := binary.Read(conn, binary.BigEndian, req); err != nil {
			return nil, err
		}
		glog.V(2).Info("recieve request ", req)

		socket, err := s.handleConnect(conn, req)
		if err != nil {
			return nil, err
		}

		if socket != nil {
			return socket, nil
		}
	}
}

func (s *Server) handleConnect(conn protocol.Conn, req *request) (*Socket, error) {
	res := &response{}
	s.rwm.RLock()
	socket, exist := s.clients[req.Id]
	s.rwm.RUnlock()
	if exist {
		socket.stop()
		socket.start(conn)
	} else {
		socket = NewSocket()
		s.rwm.Lock()
		s.clients[req.Id] = socket
		s.rwm.Unlock()
		socket.start(conn)
	}
	res.Status = statusOk
	glog.V(2).Info("send response ", res)
	err := binary.Write(conn, binary.BigEndian, res)
	return socket, err
}
