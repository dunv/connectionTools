package connectionTools

import "errors"

var ErrSendTimeout = errors.New("send timeout")
var ErrConnectionClosed = errors.New("tried to send on a closed connection")

type ErrNotAllReachable struct {
	ErrMap map[string]error
}

func (e ErrNotAllReachable) Error() string {
	return "could not reach all subscribers"
}
