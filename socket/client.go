package socket

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"io"
	"net"
	"sync"
	"time"

	"github.com/cirias/myvpn/cipher"
	"github.com/cirias/myvpn/encrypted"
	"github.com/cirias/myvpn/tcp"
	"github.com/golang/glog"
)

type Client struct {
	conn       *encrypted.Conn
	mu         sync.RWMutex
	id         id
	quit       chan struct{}
	remoteAddr string
	psk        string
	IP         net.IP
	IPMask     net.IPMask
}

func NewClient(psk, remoteAddr string) (c *Client, err error) {
	c = &Client{
		quit:       make(chan struct{}),
		psk:        psk,
		remoteAddr: remoteAddr,
	}

	var wg sync.WaitGroup
	var once sync.Once

	wg.Add(1)
	go func() {
		for {
			select {
			case <-c.quit:
				break
			default:
			}

			glog.V(2).Infoln("start dial connection")
			conn, err := c.dial()
			if err != nil {
				glog.Warningf("fail to dail %s, retry...\n", err)
				time.Sleep(10 * time.Second)
				continue
			}
			glog.V(2).Infoln("dial connection success")

			once.Do(wg.Done)

			c.mu.Lock()
			c.conn = conn
			c.mu.Unlock()

			for {
				select {
				case err = <-conn.InErrCh:
				case err = <-conn.OutErrCh:
				}
				if err != nil {
					glog.Error("error occured during socket run", err)
					break
				}
			}
			conn.Close()
		}
	}()

	wg.Wait()
	return
}

func (c *Client) In() chan<- []byte {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.conn.InCh
}

func (c *Client) Out() <-chan []byte {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.conn.OutCh
}

func (c *Client) InErr() <-chan error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.conn.InErrCh
}

func (c *Client) OutErr() <-chan error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.conn.InErrCh
}

func (c *Client) dial() (*encrypted.Conn, error) {
	cph, err := cipher.NewCipher([]byte(c.psk))
	if err != nil {
		return nil, err
	}

	tcpConn, err := tcp.NewClient(c.remoteAddr)
	if err != nil {
		return nil, err
	}
	conn := encrypted.NewConn(cph, tcpConn)

	req, err := c.newRequest()
	glog.V(2).Infoln("send request", req)
	w := &bytes.Buffer{}
	if err := binary.Write(w, binary.BigEndian, req); err != nil {
		return nil, err
	}
	conn.InCh <- w.Bytes()

	b := <-conn.OutCh
	r := bytes.NewBuffer(b)
	res := &response{}
	if err := binary.Read(r, binary.BigEndian, res); err != nil {
		return nil, err
	}
	glog.V(2).Infoln("recieve response", res)

	switch res.Status {
	case statusOK:
		c.id = res.Id
	case statusInvalidSecret:
		err = ErrInvalidSecret
	default:
		err = ErrUnknowErr
	}

	return conn, err
}

func (c *Client) Close() error {
	close(c.quit)
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.conn.Close()
}

func (c *Client) newRequest() (*request, error) {
	req := &request{}

	req.PSK = sha256.Sum256([]byte(c.psk))

	newPSK := make([]byte, cipher.KeySize)
	if _, err := io.ReadFull(rand.Reader, newPSK); err != nil {
		return nil, err
	}
	copy(req.NewPSK[:], newPSK)

	req.Id = c.id

	return req, nil
}
