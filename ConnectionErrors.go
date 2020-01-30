package connectionTools

import "errors"

var ErrSendTimeout = errors.New("send timeout")
var ErrConnectionClosed = errors.New("tried to send on a closed connection")
