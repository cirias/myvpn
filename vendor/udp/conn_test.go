package udp

import (
	"fmt"
	"testing"
)

const (
	address = "127.0.0.1:6666"
)

func TestConn(t *testing.T) {
	fmt.Println("0")
	ln, err := Listen("udp", address)
	if err != nil {
		t.Errorf("%s\n", err)
	}

	dc, err := Dial("udp", address)
	if err != nil {
		t.Errorf("%s\n", err)
	}

	lc, err := ln.Accept(dc.LocalAddr().String())
	if err != nil {
		t.Errorf("%s\n", err)
	}

	dc.Write([]byte("Hello World"))
	bs := make([]byte, 30)
	_, err = lc.Read(bs)
	if err != nil {
		t.Errorf("%s\n", err)
	}
}
