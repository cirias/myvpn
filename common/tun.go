package common

import (
	"github.com/songgao/water"
	"log"
	"os/exec"
)

func ConfigureTun(tun *water.Interface, addr string) (err error) {
	log.Printf("Assigning %s to %s\n", addr, tun.Name())
	err = exec.Command("ip", "addr", "add", addr, "dev", tun.Name()).Run()
	if err != nil {
		return
	}
	err = exec.Command("ip", "link", "set", "dev", tun.Name(), "up").Run()
	if err != nil {
		return
	}
	return
}
