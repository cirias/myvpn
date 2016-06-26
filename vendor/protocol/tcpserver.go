package protocol

import (
	"net"

	"cipher"
)

type tcpServerConn struct {
	*TCPConn
	remoteAddr net.IP
	ipNetMask  net.IPMask
	listener   *TCPListener
}

func newTCPServerConn(c *net.TCPConn, ln *TCPListener) (conn *tcpServerConn, err error) {
	conn = &tcpServerConn{
		listener: ln,
	}
	conn.TCPConn, err = newTCPConn(c)
	return
}

func (c *tcpServerConn) Close() error {
	if c.listener != nil {
		c.listener.ipAddrPool.Put(&net.IPNet{c.remoteAddr, c.ipNetMask})
	}

	return c.TCPConn.Close()
}

func (c *tcpServerConn) handleConnectionRequest(req *ConnectionRequest) (*ConnectionResponse, error) {
	if req.PSK != c.listener.psk {
		c.Conn.Close()
		return &ConnectionResponse{Status: StatusInvalidSecret}, nil
	}

	ip, err := c.listener.ipAddrPool.Get()
	if err != nil {
		c.Conn.Close()
		return &ConnectionResponse{Status: StatusNoIPAddrAvaliable}, nil
	}
	c.remoteAddr = ip.IP.To4()

	c.cipher, err = cipher.NewCipher(req.Key[:])
	if err != nil {
		return nil, err
	}

	res := &ConnectionResponse{
		Status: StatusOK,
	}
	copy(res.IP[:], ip.IP.To4())
	copy(res.IPMask[:], ip.Mask)

	return res, nil
}

func (conn *tcpServerConn) RemoteIPAddr() net.IP {
	return conn.remoteAddr
}
