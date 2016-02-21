package tun

import (
	"net"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/songgao/water"
)

const (
	UpHook   = "if-up.sh"
	DownHook = "if-down.sh"
)

type Tun struct {
	*water.Interface
	IPNet   *net.IPNet
	hookDir string
}

func NewTun(ip *net.IPNet, hookDir string) (*Tun, error) {
	i, err := water.NewTUN("")
	return &Tun{i, ip, hookDir}, err
}

func execScript(path string, args ...string) (err error) {
	cmd := exec.Command(path, args...)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	err = cmd.Run()
	return
}

func (t *Tun) Up(args ...string) error {
	return execScript(filepath.Join(t.hookDir, UpHook), append([]string{t.Name(), t.IPNet.String()}, args...)...)
}

func (t *Tun) Down(args ...string) error {
	return execScript(filepath.Join(t.hookDir, DownHook), append([]string{t.Name(), t.IPNet.String()}, args...)...)
}
