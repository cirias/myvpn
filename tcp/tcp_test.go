package tcp

import (
	"sync"
	"testing"
)

func TestTCP(t *testing.T) {
	var wg sync.WaitGroup

	s, err := NewServer("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		_, err := s.Accept()
		if err != nil {
			t.Fatal(err)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		conn, err := NewClient(s.ln.Addr().String())
		if err != nil {
			t.Fatal(err)
		}
		conn.Close()
		conn.InCh <- []byte("abc")
	}()

	wg.Wait()
}
