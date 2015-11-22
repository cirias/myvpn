package common

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"crypto/rand"
	"errors"
	"io"
)

var errEmptyPassword = errors.New("empty key")

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
	KEYLEN = 32
	IVLEN  = 16
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
	enc cipher.Stream
	dec cipher.Stream
	key []byte
}

// NewCipher creates a cipher that can be used in Dial() etc.
// Use cipher.Copy() to create a new cipher with the same method and password
// to avoid the cost of repeated cipher initialization.
func NewCipher(password string) (c *Cipher, err error) {
	if password == "" {
		return nil, errEmptyPassword
	}

	key := evpBytesToKey(password, KEYLEN)

	c = &Cipher{key: key}
	return
}

// Initializes the block cipher with CFB mode, returns IV.
func (c *Cipher) InitEncrypt() (iv []byte, err error) {
	iv = make([]byte, IVLEN)
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return nil, err
	}
	c.enc, err = newStream(c.key, iv, Encrypt)
	if err != nil {
		return nil, err
	}
	return
}

func (c *Cipher) InitDecrypt(iv []byte) (err error) {
	c.dec, err = newStream(c.key, iv, Decrypt)
	return
}

func (c *Cipher) Encrypt(dst, src []byte) {
	c.enc.XORKeyStream(dst, src)
}

func (c *Cipher) Decrypt(dst, src []byte) {
	c.dec.XORKeyStream(dst, src)
}

func (c *Cipher) Copy() *Cipher {
	nc := *c
	nc.enc = nil
	nc.dec = nil
	return &nc
}
