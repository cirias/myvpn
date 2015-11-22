package common

import (
	"errors"
	"fmt"
	"net"
)

var ErrIPPoolFull = errors.New("IPPool is full")

type IPPool struct {
	Network *net.IPNet
	exists  []bool
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

	pool = NewIPPool(network)
	id, err := pool.getIdentity(ip)
	if err != nil {
		return
	}
	pool.exists[id] = true

	return
}

func (pool *IPPool) capacity() int {
	ones, bits := pool.Network.Mask.Size()
	return (1 << uint(bits-ones)) - 2
}

func (pool *IPPool) GetIP() (ip net.IP, err error) {
	var identity int
	for identity = 1; identity < len(pool.exists); identity++ {
		if pool.exists[identity] == false {
			break
		}
	}

	if identity == len(pool.exists) {
		err = ErrIPPoolFull
		return
	}
	defer func() {
		pool.exists[identity] = true
	}()

	ip = net.IPv4(0, 0, 0, 0).To4()
	copy(ip, pool.Network.IP.To4())
	for i := 1; identity != 0; i++ {
		ip[len(ip)-i] = ip[len(ip)-i] | (byte)(0xFF&identity)
		identity = identity >> 8
	}
	return
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

func (pool *IPPool) ReturnIP(ip net.IP) (err error) {
	identity, err := pool.getIdentity(ip)
	if err != nil {
		return
	}

	pool.exists[identity] = false
	return
}
