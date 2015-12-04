package main

import (
	"net"
)

type IPPool chan net.IP

func NewIPPool(serverIP net.IP, network *net.IPNet) (pool IPPool, err error) {
	// calculate pool size
	ones, bits := network.Mask.Size()
	size := 1<<uint(bits-ones) - 2

	// Initialize pool
	pool = make(chan net.IP, size)
	for identity := 1; identity <= size; identity++ {
		newIP := make(net.IP, 4)
		copy(newIP, network.IP.To4())
		for i, index := 1, identity; index != 0; i++ {
			newIP[len(newIP)-i] = newIP[len(newIP)-i] | (byte)(0xFF&index)
			index = index >> 8
		}

		if newIP.Equal(serverIP) {
			continue
		}

		pool <- newIP
	}
	return
}

func IPMapPort(ip net.IP, network *net.IPNet, base int) (port int) {
	ones, bits := network.Mask.Size()
	zeros := 1<<uint(bits-ones) - 1
	port = base
	for i := 1; zeros > 0; i++ {
		port = port + (0xFF&zeros&int(ip[len(ip)-i]))<<uint(8*(i-1))
		zeros = zeros >> 8
	}
	return
}
