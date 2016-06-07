package cipher

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"io"
)

const (
	KeySize = sha256.Size
	IVSize  = aes.BlockSize
)

type Cipher struct {
	block cipher.Block
}

func NewCipher(key []byte) (c *Cipher, err error) {
	if len(key) != KeySize {
		k := sha256.Sum256(key)
		key = k[:]
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return
	}

	c = &Cipher{block: block}
	return
}

type Decrypter struct {
	stream cipher.Stream
}

func NewDecrypter(c *Cipher, iv []byte) *Decrypter {
	return &Decrypter{
		stream: cipher.NewOFB(c.block, iv),
	}
}

func (dec *Decrypter) Decrypt(dst, src []byte) {
	dec.stream.XORKeyStream(dst, src)
}

type Encrypter struct {
	stream cipher.Stream
}

func NewEncrypter(c *Cipher, iv []byte) *Encrypter {
	return &Encrypter{
		stream: cipher.NewOFB(c.block, iv),
	}
}

func (enc *Encrypter) Encrypt(dst, src []byte) {
	enc.stream.XORKeyStream(dst, src)
}

func NewIV() (iv []byte, err error) {
	iv = make([]byte, IVSize)
	if _, err = io.ReadFull(rand.Reader, iv); err != nil {
		return
	}
	return
}
