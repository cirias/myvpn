package tcp

import (
	"net"
)

type Server struct {
	net.Listener
}

func NewServer(listenAdd string) (*Server, error) {
	ln, err := net.Listen("tcp", listenAdd)
	if err != nil {
		return nil, err
	}

	return &Server{ln}, nil
}

func (s *Server) Accept() (*Conn, error) {
	c, err := s.Listener.Accept()
	if err != nil {
		return nil, err
	}

	return NewConn(c), nil
}
