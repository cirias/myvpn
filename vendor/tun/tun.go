package tun

import (
	"net"
	"os"
	"os/exec"
)

const (
	UpHook   = "if-up.sh"
	DownHook = "if-down.sh"
)

type Interface struct {
	file   *os.File
	name   string
	ip     *net.IP
	ipmask *net.IPMask
}

func NewTUN(ifName string, ip *net.IP, ipmask *net.IPMask) (ifce *Interface, err error) {
	ifce, err = newTUN(ifName)
	if err != nil {
		return
	}

	ifce.ip = ip
	ifce.ipmask = ipmask
	return
}

func (ifce *Interface) Name() string {
	return ifce.name
}

func (ifce *Interface) IP() *net.IP {
	return ifce.ip
}

func (ifce *Interface) IPMask() *net.IPMask {
	return ifce.ipmask
}

func (ifce *Interface) Write(p []byte) (n int, err error) {
	n, err = ifce.file.Write(p)
	return
}

func (ifce *Interface) Read(p []byte) (n int, err error) {
	n, err = ifce.file.Read(p)
	return
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

func (ifce *Interface) Up(path string, args ...string) error {
	return execScript(path, append([]string{ifce.name, (&net.IPNet{*ifce.ip, *ifce.ipmask}).String()}, args...)...)
}

func (ifce *Interface) Down(path string, args ...string) error {
	return execScript(path, append([]string{ifce.name, (&net.IPNet{*ifce.ip, *ifce.ipmask}).String()}, args...)...)
}
