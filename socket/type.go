package socket

type idType [8]byte

type request struct {
	Id idType
}

const (
	statusOk = iota
)

type response struct {
	Status byte
}
