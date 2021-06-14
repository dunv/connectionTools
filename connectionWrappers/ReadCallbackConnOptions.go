package connectionwrappers

import (
	"context"
	"log"
	"time"
)

type ReadCallbackConnOption interface {
	apply(*readCallbackConnOptions)
}

type readCallbackConnOptions struct {
	ctx                            context.Context
	connectBackoffInitial          time.Duration
	connectBackoffLimit            time.Duration
	connectBackoffFactor           uint8
	readBackoffInitial             time.Duration
	readBackoffLimit               time.Duration
	readBackoffFactor              uint8
	consumeTimeout                 time.Duration
	consumeTimeoutExceededAbortAll bool

	onConnect               func()
	onDisconnect            func()
	onPermanentDisconnect   func()
	isPermanentConnectError func(err error) bool
	isPermanentReadError    func(err error) bool
	connectErrorHandler     func(err error)
	readErrorHandler        func(err error)
	disconnectErrorHandler  func(err error)
}

type funcReadCallbackConnOption struct {
	f func(*readCallbackConnOptions)
}

func (fdo *funcReadCallbackConnOption) apply(do *readCallbackConnOptions) {
	fdo.f(do)
}

func newFuncReadCallbackConnOption(f func(*readCallbackConnOptions)) *funcReadCallbackConnOption {
	return &funcReadCallbackConnOption{f: f}
}

// Pass a context which is checked by every Connect and Read
// Cancelling this context can be used to cancel all running routines
func WithContext(ctx context.Context) ReadCallbackConnOption {
	return newFuncReadCallbackConnOption(func(o *readCallbackConnOptions) {
		o.ctx = ctx
	})
}

// Use exponential backoff when connecting. (default: static backoff of 1 second)
func WithConnectBackoff(initial time.Duration, limit time.Duration, factor uint8) ReadCallbackConnOption {
	return newFuncReadCallbackConnOption(func(o *readCallbackConnOptions) {
		if factor > 0 {
			o.connectBackoffInitial = initial
			o.connectBackoffLimit = limit
			o.connectBackoffFactor = factor
			return
		}
		log.Println("cannot assign backoff-factor less than 0. using defaults instead.")
	})
}

// Use exponential backoff when reading. (default: static backoff of 1 second)
func WithReadBackoff(initial time.Duration, limit time.Duration, factor uint8) ReadCallbackConnOption {
	return newFuncReadCallbackConnOption(func(o *readCallbackConnOptions) {
		if factor > 0 {
			o.readBackoffInitial = initial
			o.readBackoffLimit = limit
			o.readBackoffFactor = factor
			return
		}
		log.Println("cannot assign backoff-factor less than 0. using defaults instead.")
	})
}

// Check every error that the Connect(ctx context.Context) method returns
// if the error is considered permanent (default: all errors are transient)
// the connection will be stopped alltogether
func WithIsPermanentConnectError(isPermanentConnectError func(err error) bool) ReadCallbackConnOption {
	return newFuncReadCallbackConnOption(func(o *readCallbackConnOptions) {
		o.isPermanentConnectError = isPermanentConnectError
	})
}

// Check every error that the Read(ctx context.Context) method returns
// if the error is considered permanent (default, if this handler is not passed)
// the connection will be disconnected and retried
// if the error is transient, the response will be skipped and another Read is called
func WithIsPermanentReadError(isPermanentReadError func(err error) bool) ReadCallbackConnOption {
	return newFuncReadCallbackConnOption(func(o *readCallbackConnOptions) {
		o.isPermanentReadError = isPermanentReadError
	})
}

func WithConnectErrorHandler(handler func(err error)) ReadCallbackConnOption {
	return newFuncReadCallbackConnOption(func(o *readCallbackConnOptions) {
		o.connectErrorHandler = handler
	})
}

func WithReadErrorHandler(handler func(err error)) ReadCallbackConnOption {
	return newFuncReadCallbackConnOption(func(o *readCallbackConnOptions) {
		o.readErrorHandler = handler
	})
}

func WithDisconnectErrorHandler(handler func(err error)) ReadCallbackConnOption {
	return newFuncReadCallbackConnOption(func(o *readCallbackConnOptions) {
		o.disconnectErrorHandler = handler
	})
}

// Define a maximum time that the read-call waits when pushing into the channel
// When the timeout is exceeded two possibilities exist:
// - continue with reading as if nothing happened
// - cancel all running routines, close channel (default)
func WithConsumeTimeout(timeout time.Duration, abortAllWhenExceeded bool) ReadCallbackConnOption {
	return newFuncReadCallbackConnOption(func(o *readCallbackConnOptions) {
		o.consumeTimeout = timeout
		o.consumeTimeoutExceededAbortAll = abortAllWhenExceeded
	})
}

func WithOnConnect(handler func()) ReadCallbackConnOption {
	return newFuncReadCallbackConnOption(func(o *readCallbackConnOptions) {
		o.onConnect = handler
	})
}

func WithOnDisconnect(handler func()) ReadCallbackConnOption {
	return newFuncReadCallbackConnOption(func(o *readCallbackConnOptions) {
		o.onDisconnect = handler
	})
}

func WithOnPermanentDisconnect(handler func()) ReadCallbackConnOption {
	return newFuncReadCallbackConnOption(func(o *readCallbackConnOptions) {
		o.onDisconnect = handler
	})
}
