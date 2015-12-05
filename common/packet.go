package common

const (
	IPSize     = 4
	IPMaskSize = 4
	PortSize   = 2
)

var (
	Heartbeat = []byte{0xff, 0xff, 0xff, 0xff}
)
