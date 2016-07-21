package tun

import (
	"os"
	"os/exec"
)

const (
	UpHook   = "if-up.sh"
	DownHook = "if-down.sh"
)

type Interface struct {
	file     *os.File
	name     string
	inCh     chan []byte
	outCh    chan []byte
	inErrCh  chan error
	outErrCh chan error
}

func NewTUN(ifName string) (ifce *Interface, err error) {
	ifce, err = newTUN(ifName)
	if err != nil {
		return
	}

	ifce.inCh = make(chan []byte)
	ifce.outCh = make(chan []byte)
	ifce.inErrCh = make(chan error)
	ifce.outErrCh = make(chan error)

	go ifce.reading()
	go ifce.writing()
	return
}

func (i *Interface) In() chan<- []byte {
	return i.inCh
}

func (i *Interface) Out() <-chan []byte {
	return i.outCh
}

func (i *Interface) InErr() <-chan error {
	return i.inErrCh
}

func (i *Interface) OutErr() <-chan error {
	return i.outErrCh
}

func (ifce *Interface) Name() string {
	return ifce.name
}

func (ifce *Interface) Write(p []byte) (n int, err error) {
	n, err = ifce.file.Write(p)
	return
}

/*
 * func (ifce *Interface) Read(p []byte) (n int, err error) {
 *   n, err = ifce.file.Read(p)
 *   return
 * }
 */

func (ifce *Interface) Read(p []byte) (n int, err error) {
	for {
		n, err = ifce.file.Read(p)
		if err != nil {
			return
		}

		// only keep ipv4 packet
		if (p[0] >> 4) == 0x04 {
			return
		}
	}
}

func (ifce *Interface) Close() (err error) {
	return ifce.file.Close()
}

func execScript(path string, args ...string) (err error) {
	cmd := exec.Command(path, args...)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	return cmd.Run()
}

func (ifce *Interface) Run(path string, args ...string) error {
	return execScript(path, append([]string{ifce.name}, args...)...)
}

func (i *Interface) reading() {
	b := make([]byte, 65535)
	for {
		n, err := i.Read(b)
		if err != nil {
			select {
			case i.outErrCh <- err:
			default:
			}
		}

		i.outCh <- b[:n]
	}
}

func (i *Interface) writing() {
	for b := range i.inCh {
		_, err := i.Write(b)
		if err != nil {
			select {
			case i.inErrCh <- err:
			default:
			}
		}
	}
}
