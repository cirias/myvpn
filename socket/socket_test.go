package socket

import (
	"fmt"
	"reflect"
	"sync"
	"testing"
)

func TestStop(t *testing.T) {
	s := NewSocket()
	s.stop()
	s.stop()
}

func TestStart(t *testing.T) {
	var wg sync.WaitGroup
	testData := "test data"

	wg.Add(1)
	go func() {
		defer wg.Done()
		server, err := NewServer("psk", ":3456")
		if err != nil {
			t.Error(err)
			return
		}
		defer func() {
			if err := server.Close(); err != nil {
				t.Error(err)
			}
		}()

		s, err := server.Accept()
		if err != nil {
			t.Error(err)
			return
		}
		defer func() {
			if err := s.Close(); err != nil {
				t.Error(err)
			}
		}()

		d := make([]byte, 1024)
		n, err := s.Read(d)
		if err != nil {
			t.Error(err)
			return
		}

		if !reflect.DeepEqual(d[:n], []byte(testData)) {
			fmt.Println(d[:n])
			t.Error("read result not equal to write")
			return
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		client, err := NewClient("psk", ":3456")
		if err != nil {
			t.Error(err)
			return
		}
		defer func() {
			if err := client.Close(); err != nil {
				t.Error(err)
			}
		}()

		d := []byte(testData)
		n, err := client.Write(d)
		if err != nil {
			t.Error(err)
			return
		}

		if n != len(d) {
			t.Error("write return n not equal to length of input")
			return
		}
	}()

	wg.Wait()
}
