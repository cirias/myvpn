package protocol

import (
	"bytes"
	"reflect"
	"testing"

	"github.com/cirias/myvpn/cipher"
)

func TestSendRecieve(t *testing.T) {
	type S struct {
		A byte
		B [4]byte
	}

	cph, _ := cipher.NewCipher([]byte("key"))

	b := &bytes.Buffer{}

	src := &S{
		A: 0x0,
		B: [...]byte{'T', 'E', 'S', 'T'},
	}

	if err := send(cph, b, src); err != nil {
		t.Error(err)
		return
	}

	dst := &S{}

	if err := recieve(cph, b, dst); err != nil {
		t.Error(err)
		return
	}

	if !reflect.DeepEqual(src, dst) {
		print(src)
		print(dst)
		t.Error("recieve data not equal to send data")
	}
}

func TestReadWrite(t *testing.T) {
	src := []byte("test text. blabla....")

	cph, _ := cipher.NewCipher([]byte("key"))

	b := &bytes.Buffer{}

	n, err := write(cph, b, src)
	if err != nil {
		t.Error(err)
		return
	}
	if n != len(src) {
		t.Error("write return n not equal to length of src")
	}

	dst := make([]byte, 1024)

	n, err = read(cph, b, dst)
	if err != nil {
		t.Error(err)
		return
	}

	if !reflect.DeepEqual(src, dst[:n]) {
		print(src)
		print(dst[:n])
		t.Error("recieve data not equal to send data")
	}
}
