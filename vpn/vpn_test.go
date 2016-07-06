package vpn

import (
	"errors"
	"fmt"
	"io"
	"net"
	"reflect"
	"sync"
	"testing"
)

type End struct {
	Reader *io.PipeReader
	Writer *io.PipeWriter
}

func (e End) Read(data []byte) (n int, err error)  { return e.Reader.Read(data) }
func (e End) Write(data []byte) (n int, err error) { return e.Writer.Write(data) }

type Conn struct {
	Server *End
	Client *End
}

func NewConn() *Conn {
	// A connection consists of two pipes:
	// Client      |      Server
	//   writes   ===>  reads
	//    reads  <===   writes

	serverRead, clientWrite := io.Pipe()
	clientRead, serverWrite := io.Pipe()

	return &Conn{
		Server: &End{
			Reader: serverRead,
			Writer: serverWrite,
		},
		Client: &End{
			Reader: clientRead,
			Writer: clientWrite,
		},
	}
}

func TestServerClient(t *testing.T) {
	var wg sync.WaitGroup

	serverIfce := NewConn()
	server, err := NewServer(serverIfce.Server, "10.0.0.1/24")
	if err != nil {
		t.Error(err)
		return
	}

	conn := NewConn()
	wg.Add(1)
	go func() {
		defer wg.Done()
		err := server.Handle(conn.Server)
		if err != nil {
			t.Error(err)
		}
	}()

	clientIfce := NewConn()
	client, err := NewClient(clientIfce.Server, conn.Client)
	if err != nil {
		t.Error(err)
		return
	}

	if !reflect.DeepEqual(client.IP, net.IP([]byte{10, 0, 0, 2})) {
		fmt.Println("client IP", []byte(client.IP))
		t.Error(errors.New("client IP is not correct"))
	}

	if !reflect.DeepEqual(client.IPMask, net.IPMask([]byte{255, 255, 255, 0})) {
		fmt.Println("client IPMask", []byte(client.IPMask))
		t.Error(errors.New("client IPMask is not correct"))
	}

	wg.Add(1)
	go func() {
		wg.Done()
		err := client.ReadWrite()
		if err != nil {
			t.Error(err)
		}
	}()

	_, err = clientIfce.Client.Write([]byte("test data"))
	if err != nil {
		t.Error(err)
		return
	}

	b := make([]byte, 1024)
	n, err := serverIfce.Client.Read(b)
	if err != nil {
		t.Error(err)
		return
	}
	if !reflect.DeepEqual(b[:n], []byte("test data")) {
		fmt.Println("b[:n] ", b[:n])
		t.Error(errors.New("server recieve is not correct"))
	}

	/*
	 * p := []byte{16: 10, 17: 0, 18: 0, 19: 2}
	 * p = append(p, []byte("test data 2")...)
	 * _, err = serverIfce.Client.Write(p)
	 * if err != nil {
	 *   t.Error(err)
	 *   return
	 * }
	 * b = make([]byte, 1024)
	 * n, err = clientIfce.Client.Read(b)
	 * if err != nil {
	 *   t.Error(err)
	 *   return
	 * }
	 * if !reflect.DeepEqual(b[:n], p) {
	 *   fmt.Println("b[:n] ", b[:n])
	 *   t.Error(errors.New("client recieve is not correct"))
	 * }
	 */

	client.Close()
	server.Close()

	wg.Wait()
}
