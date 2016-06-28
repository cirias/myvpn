package socket

const (
	typeReqConnect = iota
	typeReqReconnect
)

type idType [8]byte

type request struct {
	reqType byte
	id      idType
}

const (
	statusOk = iota
	statusIdConflict
	statusNotExist
)

type response struct {
	status byte
}
