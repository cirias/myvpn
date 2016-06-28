package socket

import (
	"encoding/binary"
	"sync"

	"protocol"
)

type Server struct {
	clients  map[idType]*Socket
	listener protocol.Listener
	rwm      sync.RWMutex
}

func NewSocketServer(psk, localAddr string) (*Server, error) {
	ln, err := protocol.ListenTCP(psk, localAddr)
	if err != nil {
		return nil, err
	}

	s := &Server{
		listener: ln,
	}

	return s, nil
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

		if req.reqType == typeReqReconnect {
			if err := s.handleReconnect(conn, req); err != nil {
				return nil, err
			}
			continue
		}

		if req.reqType == typeReqConnect {
			socket, err := s.handleConnect(conn, req)
			if err != nil {
				return nil, err
			}

			if socket != nil {
				return socket, nil
			}
		}
	}
}

func (s *Server) handleConnect(conn protocol.Conn, req *request) (*Socket, error) {
	res := &response{}
	s.rwm.RLock()
	_, exist := s.clients[req.id]
	s.rwm.RUnlock()
	if exist {
		res.status = statusIdConflict
	}
	res.status = statusOk
	err := binary.Write(conn, binary.BigEndian, res)
	if err != nil || res.status != statusOk {
		return nil, err
	}

	socket := NewSocket()
	s.rwm.Lock()
	s.clients[req.id] = socket
	s.rwm.Unlock()

	socket.start(conn)

	return socket, nil
}

func (s *Server) handleReconnect(conn protocol.Conn, req *request) error {
	res := &response{}
	s.rwm.RLock()
	socket, exist := s.clients[req.id]
	s.rwm.RUnlock()
	if !exist {
		res.status = statusNotExist
	}

	res.status = statusOk

	if err := binary.Write(conn, binary.BigEndian, res); err != nil {
		return err
	}

	socket.stop()
	socket.start(conn)

	return nil
}
