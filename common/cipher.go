package common

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"crypto/rand"
	"errors"
	"io"
)

var ErrEmptyPassword = errors.New("empty password")
var ErrKeyTooShort = errors.New("key too short")

func md5sum(d []byte) []byte {
	h := md5.New()
	h.Write(d)
	return h.Sum(nil)
}

func evpBytesToKey(password string, keyLen int) (key []byte) {
	const md5Len = 16

	cnt := (keyLen-1)/md5Len + 1
	m := make([]byte, cnt*md5Len)
	copy(m, md5sum([]byte(password)))

	// Repeatedly call md5 until bytes generated is enough.
	// Each call to md5 uses data: prev md5 sum + password.
	d := make([]byte, md5Len+len(password))
	start := 0
	for i := 1; i < cnt; i++ {
		start += md5Len
		copy(d, m[start-md5Len:start])
		copy(d[md5Len:], password)
		copy(m[start:], md5sum(d))
	}
	return m[:keyLen]
}

type DecOrEnc int

const (
	Decrypt DecOrEnc = iota
	Encrypt
)

const (
	KEY_SIZE = 32
	IV_SIZE  = 16
)

func newStream(key, iv []byte, doe DecOrEnc) (c cipher.Stream, err error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return
	}

	if doe == Encrypt {
		return cipher.NewCFBEncrypter(block, iv), nil
	} else {
		return cipher.NewCFBDecrypter(block, iv), nil
	}
}

type Cipher struct {
	key []byte
}

func NewCipher(password string) (c *Cipher, err error) {
	if password == "" {
		return nil, ErrEmptyPassword
	}

	key := evpBytesToKey(password, KEY_SIZE)

	c = &Cipher{key: key}
	return
}

func NewCipherWithKey(key []byte) (c *Cipher, err error) {
	if len(key) < KEY_SIZE {
		err = ErrKeyTooShort
		return
	}

	c = &Cipher{key: key}
	return
}

func (c *Cipher) Encrypt(iv, dst, src []byte) (err error) {
	if _, err = io.ReadFull(rand.Reader, iv); err != nil {
		return
	}

	enc, err := newStream(c.key, iv, Encrypt)
	if err != nil {
		return
	}

	enc.XORKeyStream(dst, src)
	return
}

func (c *Cipher) Decrypt(iv, dst, src []byte) (err error) {
	dec, err := newStream(c.key, iv, Decrypt)
	if err != nil {
		return
	}

	dec.XORKeyStream(dst, src)
	return
}

func (c *Cipher) Copy() *Cipher {
	nc := *c
	return &nc
}
