package protocol

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

type CipherReadWriter struct {
	io.ReadWriteCloser
	Cipher *Cipher
}

func (crw *CipherReadWriter) Read(b []byte) (n int, err error) {
	ciphertext := make([]byte, IVSize+len(b))
	n, err = crw.ReadWriteCloser.Read(ciphertext)
	if err != nil {
		return 0, err
	}

	stream := cipher.NewOFB(crw.Cipher.block, ciphertext[:IVSize])
	stream.XORKeyStream(b, ciphertext[IVSize:n])
	return n - IVSize, err
}

func (crw *CipherReadWriter) Write(b []byte) (n int, err error) {
	ciphertext := make([]byte, IVSize+len(b))
	if _, err = io.ReadFull(rand.Reader, ciphertext[:IVSize]); err != nil {
		return
	}

	stream := cipher.NewOFB(crw.Cipher.block, ciphertext[:IVSize])
	stream.XORKeyStream(ciphertext[IVSize:], b)
	return crw.ReadWriteCloser.Write(ciphertext)
}
