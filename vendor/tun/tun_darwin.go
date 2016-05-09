// +build darwin

package tun

import "os"

func newTUN(ifName string) (ifce *Interface, err error) {
	file, err := os.OpenFile("/dev/"+ifName, os.O_RDWR, 0)
	if err != nil {
		return nil, err
	}
	ifce = &Interface{file: file, name: name}
	return
}
