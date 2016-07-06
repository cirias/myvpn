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
	file *os.File
	name string
}

func NewTUN(ifName string) (ifce *Interface, err error) {
	ifce, err = newTUN(ifName)
	if err != nil {
		return
	}

	return
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
