package connectionwrappers

import (
	"context"
	"fmt"
	"sync"
	"time"
)

var ErrNoOneListening = fmt.Errorf("no one listing on channel")

// ReadCallbackConnection is the interface that a
// connection must implement in order to be used with
// the ReadCallbackConn wrapper
type ReadCallbackConnection interface {
	Connect(ctx context.Context) error
	Disconnect(ctx context.Context) error
	Read(ctx context.Context) (interface{}, error)
}

// ReadCallbackConn is a wrapper for a connection with a read-callback
// it will
// - connect (with exponential backoff if wanted)
// - read (with exponential backoff if wanted)
// - pass read data out via channel
// until a permanent connect-error is encountered
type ReadCallbackConn struct {
	localCancel    context.CancelFunc
	localContext   context.Context
	conn           ReadCallbackConnection
	opts           readCallbackConnOptions
	connectBackoff time.Duration
	readBackoff    time.Duration

	isConnectedMu *sync.Mutex
	_isConnected  bool
}

// Create a new ReadCallbackConn
func NewReadCallbackConn(conn ReadCallbackConnection, opts ...ReadCallbackConnOption) *ReadCallbackConn {
	mergedOpts := readCallbackConnOptions{
		ctx:                            context.Background(),
		connectBackoffInitial:          time.Second,
		connectBackoffLimit:            time.Second,
		connectBackoffFactor:           1,
		readBackoffInitial:             time.Second,
		readBackoffLimit:               time.Second,
		readBackoffFactor:              1,
		consumeTimeout:                 time.Minute,
		consumeTimeoutExceededAbortAll: true,
		isPermanentConnectError:        func(err error) bool { return false },
		isPermanentReadError:           func(err error) bool { return true },
		connectErrorHandler:            func(err error) {},
		readErrorHandler:               func(err error) {},
		disconnectErrorHandler:         func(err error) {},
		onConnect:                      func() {},
		onDisconnect:                   func() {},
		onPermanentDisconnect:          func() {},
	}
	for _, opt := range opts {
		opt.apply(&mergedOpts)
	}

	// create a local context so we can abort in an error condition
	localContext, localCancel := context.WithCancel(mergedOpts.ctx)

	return &ReadCallbackConn{
		localContext:   localContext,
		localCancel:    localCancel,
		conn:           conn,
		opts:           mergedOpts,
		connectBackoff: mergedOpts.connectBackoffInitial,
		readBackoff:    mergedOpts.readBackoffInitial,
		isConnectedMu:  &sync.Mutex{},
		_isConnected:   false,
	}
}

// Run starts all routines in the backround
func (s *ReadCallbackConn) Run() chan interface{} {
	c := make(chan interface{})

	go func() {
		// Outer cycle:
		// - check context
		// - make sure we never stop UNLESS we encounter a permanent error
		for {
			// check outside-context:
			// context.DeadlinExceeded always ceases all operations
			// it does so without setting an error
			if s.opts.ctx.Err() != nil {
				close(c)
				s.opts.onPermanentDisconnect()
				return
			}

			// execute "one cycle": connect, run until we encounter an error
			err := s.internalRun(c)
			if err != nil {
				// check local-context: one error condition arises if no one is listeing on output-channel
				if s.localContext.Err() != nil && err == ErrNoOneListening {
					close(c)
					s.opts.onPermanentDisconnect()
					return
				}

				// in case of a permanent error
				// - stop alltogether
				if s.opts.isPermanentConnectError(err) {
					close(c)
					s.opts.onPermanentDisconnect()
					return
				}
			}

			// in case of a non-permanent error:
			// - wait connectBackoff
			// - increase connectBackoff
			// - try again
			time.Sleep(s.connectBackoff)
			if s.connectBackoff*time.Duration(s.opts.connectBackoffFactor) < s.opts.connectBackoffLimit {
				s.connectBackoff = s.connectBackoff * time.Duration(s.opts.connectBackoffFactor)
			} else {
				s.connectBackoff = s.opts.connectBackoffLimit
			}
		}
	}()

	return c
}

// internalRun performs one cycle
// - connect
// - read until we encounter a permanent error (per default all read errors are permanent)
func (s *ReadCallbackConn) internalRun(c chan<- interface{}) error {
	// connect
	if err := s.conn.Connect(s.opts.ctx); err != nil {
		return err
	}

	// disconnect at the end of this functions, no matter where and why it ends
	defer func() {
		s.isConnectedMu.Lock()
		s._isConnected = false
		s.isConnectedMu.Unlock()
		s.opts.disconnectErrorHandler(s.conn.Disconnect(s.opts.ctx))
		s.opts.onDisconnect()
	}()

	// let everyone know we are connected
	s.isConnectedMu.Lock()
	s._isConnected = true
	s.isConnectedMu.Unlock()
	s.opts.onConnect()

	// connect was successful: reset backoff
	s.connectBackoff = s.opts.connectBackoffInitial

	// Inner cycle:
	// - check context
	// - read until error
	for {
		if s.opts.ctx.Err() != nil || s.localContext.Err() != nil {
			return nil
		}

		if res, err := s.conn.Read(s.opts.ctx); err != nil {
			// pass error into optional handler
			s.opts.readErrorHandler(err)

			// if error is permanent: break cycle
			if s.opts.isPermanentReadError(err) {
				// return nil, as this error is not a connect error
				// internalRun() should only return connect errors
				return nil
			}

			// if error is transient:
			// - ignore data
			// - wait backoff
			// - increase backoff
			// - continue reading
			time.Sleep(s.readBackoff)
			if s.readBackoff*time.Duration(s.opts.readBackoffFactor) < s.opts.readBackoffLimit {
				s.readBackoff = s.readBackoff * time.Duration(s.opts.readBackoffFactor)
			} else {
				s.readBackoff = s.opts.readBackoffLimit
			}
			continue

		} else {
			// read was successful: reset backoff
			s.readBackoff = s.opts.readBackoffInitial

			select {
			// check context
			case <-s.opts.ctx.Done():
			// pass data into the channel
			case c <- res:
			// "emergency-timeout" for when no consumer is listening
			case <-time.After(s.opts.consumeTimeout):
				if s.opts.consumeTimeoutExceededAbortAll {
					s.localCancel()
					return ErrNoOneListening
				}
				// it is possible to skip this message if wanted
				continue
			}

		}
	}
}

func (s *ReadCallbackConn) IsConnected() bool {
	s.isConnectedMu.Lock()
	defer s.isConnectedMu.Unlock()
	return s._isConnected
}
