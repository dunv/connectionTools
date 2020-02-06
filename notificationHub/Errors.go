package notificationHub

import "errors"

var ErrTooManyBufferRoutines = errors.New("cannot call start more than once (too many buffer routines)")
var ErrTooManyControlRoutines = errors.New("cannot call start more than once (too many buffer routines)")

var ErrSendTimeout = errors.New("send timeout")
var ErrConnectionClosed = errors.New("tried to send on a closed connection")
var ErrConnectionLocked = errors.New("connection is locked")
var ErrHubLocked = errors.New("hub is locked")

type ErrNotAllReachable struct {
	ErrMap map[string]error
}

func (e ErrNotAllReachable) Error() string {
	return "could not reach all subscribers"
}

var ErrNoOneListeningToRequest = errors.New("no one is listening to request")
