package common

import (
	"errors"
	"fmt"
	"net"
)

var ErrIPPoolEmpty = errors.New("No avaliable ip")
var ErrIPPoolFull = errors.New("IPPool is full")

type IPPool struct {
	Network *net.IPNet
	exists  []bool
	ips     chan net.IP
}

func NewIPPool(network *net.IPNet) (pool *IPPool) {
	pool = &IPPool{
		Network: network,
	}

	pool.exists = make([]bool, pool.capacity(), pool.capacity())
	return
}

func NewIPPoolWithCIDR(addr string) (ip net.IP, pool *IPPool, err error) {
	// Parse server IP address
	ip, network, err := net.ParseCIDR(addr)
	if err != nil {
		return
	}

	pool = &IPPool{
		Network: network,
	}

	size := pool.capacity()

	pool.ips = make(chan net.IP, size)
	id, err := pool.getIdentity(ip)
	if err != nil {
		return
	}

	for identity := 1; identity <= size; identity++ {
		if identity == id {
			continue
		}

		newIP := make(net.IP, 4)
		copy(newIP, pool.Network.IP.To4())
		for i, index := 1, identity; index != 0; i++ {
			newIP[len(newIP)-i] = newIP[len(newIP)-i] | (byte)(0xFF&index)
			index = index >> 8
		}
		pool.ips <- newIP
	}

	return
}

func (pool *IPPool) capacity() int {
	ones, bits := pool.Network.Mask.Size()
	return (1 << uint(bits-ones)) - 2
}

func (pool *IPPool) getIdentity(ip net.IP) (identity int, err error) {
	if !pool.Network.Contains(ip) {
		err = fmt.Errorf("%v is not in this ip pool", ip)
		return
	}
	ones, bits := pool.Network.Mask.Size()
	zeros := 1<<uint(bits-ones) - 1
	identity = 0
	for i := 1; zeros > 0; i++ {
		identity = identity + (0xFF&zeros&int(ip[len(ip)-i]))<<uint(8*(i-1))
		zeros = zeros >> 8
	}
	return
}

func (pool *IPPool) GetIP() (ip net.IP, err error) {
	select {
	case ip = <-pool.ips:
	default:
		err = ErrIPPoolEmpty
	}
	return
}

func (pool *IPPool) ReturnIP(ip net.IP) (err error) {
	select {
	case pool.ips <- ip:
	default:
		err = ErrIPPoolFull
	}
	return
}
