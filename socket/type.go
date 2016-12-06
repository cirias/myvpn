package socket

type id [8]byte

type request struct {
	Id id
}

const (
	statusOk = iota
)

type response struct {
	Status byte
}
