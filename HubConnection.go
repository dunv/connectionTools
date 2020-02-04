package connectionTools

import (
	"context"
	"fmt"
	"strings"
	"time"

	"golang.org/x/sync/semaphore"
)

type HubConnection struct {
	lastSeen        time.Time
	err             error
	broadcastDomain string
	connected       bool
	connectionGUID  string

	sendChannel chan<- interface{}
	sendBuffer  *chan interface{}

	debug              bool
	internalCommand    chan interface{}
	ctx                context.Context
	cancel             context.CancelFunc
	controlRoutineLock *semaphore.Weighted

	// Statistics
	openSendRequests    int
	totalSendRequests   int
	openCloseRequests   int
	totalCloseRequests  int
	openStatusRequests  int
	totalStatusRequests int
	bufferRoutines      int
	controlRoutines     int
}

type hubConnectionInternalSendRequest struct {
	item     interface{}
	response chan error
}

type hubConnectionInternalCloseRequest struct {
	reason   error
	response chan error
}

type hubConnectionInternalStatusRequest struct {
	response chan HubConnectionRepr
}

type hubConnectionInternalLastSeenRequest struct {
	lastSeen time.Time
	response chan error
}

type HubConnectionRepr struct {
	LastSeen        time.Time `json:"lastSeen,omitempty"`
	Err             string    `json:"err,omitempty"`
	BroadcastDomain string    `json:"broadcastDomain"`
	Connected       bool      `json:"connected"`
	ConnectionGUID  string    `json:"connectionGuid"`

	// Statistics
	OpenSendRequests   int `json:"openSendRequests"`
	TotalSendRequests  int `json:"totalSendRequests"`
	OpenCloseRequests  int `json:"openCloseRequests"`
	TotalCloseRequests int `json:"totalCloseRequests"`
	BufferRoutines     int `json:"bufferRoutines"`
	ControlRoutines    int `json:"controlRoutines"`
}

func (h HubConnectionRepr) String() string {
	parts := []string{}

	if h.OpenSendRequests > 0 {
		parts = append(parts, fmt.Sprintf("openSendRequest=%d", h.OpenSendRequests))
	}
	if h.OpenCloseRequests > 0 {
		parts = append(parts, fmt.Sprintf("openCloseRequests=%d", h.OpenCloseRequests))
	}

	parts = append(parts, fmt.Sprintf("totalSendRequests=%d", h.TotalSendRequests))
	parts = append(parts, fmt.Sprintf("totalCloseRequests=%d", h.TotalCloseRequests))
	parts = append(parts, fmt.Sprintf("bufferRoutines=%d", h.BufferRoutines))
	parts = append(parts, fmt.Sprintf("controlRoutines=%d", h.ControlRoutines))

	return fmt.Sprintf("Connection[%s]", strings.Join(parts, ", "))
}

func (h *HubConnection) LastSeen() time.Time {
	return h.lastSeen
}

func (h *HubConnection) Err() error {
	return h.err
}

func (h *HubConnection) BroadcastDomain() string {
	return h.broadcastDomain
}

func (h *HubConnection) Connected() bool {
	return h.connected
}

func (h *HubConnection) ConnectionGUID() string {
	return h.connectionGUID
}

func NewHubConnection(
	guid string,
	broadcastDomain string,
	sendChannel chan<- interface{},
	bufferSize int,
	debug bool,
) *HubConnection {
	ctx, cancel := context.WithCancel(context.Background())

	var buffer *chan interface{}
	if bufferSize > 0 {
		tmp := make(chan interface{}, bufferSize-1)
		buffer = &tmp
	}

	conn := HubConnection{
		lastSeen:           time.Now(),
		broadcastDomain:    broadcastDomain,
		connected:          true,
		connectionGUID:     guid,
		sendChannel:        sendChannel,
		sendBuffer:         buffer,
		debug:              debug,
		internalCommand:    make(chan interface{}),
		controlRoutineLock: semaphore.NewWeighted(1),
		ctx:                ctx,
		cancel:             cancel,
	}
	conn.start()
	return &conn
}

func (h *HubConnection) start() {
	stopBufferChannel := make(chan chan struct{})
	if h.sendBuffer != nil {
		go func() {
			h.bufferRoutines++
			if h.bufferRoutines > 1 {
				// failsafe
				panic(ErrTooManyBufferRoutines)
			}
			for {
				select {
				case returnChannel := <-stopBufferChannel:
					returnChannel <- struct{}{}
					return
				case item := <-*h.sendBuffer:
					select {
					case h.sendChannel <- item:
						res := make(chan error)
						h.internalCommand <- hubConnectionInternalLastSeenRequest{
							lastSeen: time.Now(),
							response: res,
						}
						<-res
					case returnChannel := <-stopBufferChannel:
						returnChannel <- struct{}{}
						return
					}
				}
			}
		}()
	}

	// Start control routine
	go func() {
		err := h.controlRoutineLock.Acquire(context.Background(), 1)
		if err != nil {
			panic(ErrTooManyControlRoutines)
		}

		h.controlRoutines++
		if h.controlRoutines > 1 {
			panic(ErrTooManyControlRoutines)
		}

		for {
			select {
			case <-h.ctx.Done():
				h.controlRoutines--
				h.connected = false
				h.controlRoutineLock.Release(1)
				if h.sendBuffer != nil {
					// "handshake" for stopping buffer operations
					bufferStopped := make(chan struct{})
					stopBufferChannel <- bufferStopped
					<-bufferStopped
				}
				close(h.internalCommand)
				return

			case req := <-h.internalCommand:
				switch request := req.(type) {
				case hubConnectionInternalSendRequest:
					h.openSendRequests++
					h.totalSendRequests++
					if !h.connected {
						request.response <- ErrConnectionClosed
						h.openSendRequests--
						continue
					}

					// Send into buffer if we have one
					if h.sendBuffer != nil {
						*h.sendBuffer <- request.item
						request.response <- nil
						h.openSendRequests--
						continue
					}

					// Send into actual channel
					h.sendChannel <- request.item
					h.lastSeen = time.Now()
					request.response <- nil
					h.openSendRequests--
				case hubConnectionInternalCloseRequest:
					h.openCloseRequests++
					h.totalCloseRequests++
					h.err = request.reason
					h.cancel()
					request.response <- nil
					h.openCloseRequests--
				case hubConnectionInternalStatusRequest:
					h.totalStatusRequests++
					h.openStatusRequests++
					request.response <- h.unsafeStatus()
					h.openStatusRequests--
				case hubConnectionInternalLastSeenRequest:
					h.lastSeen = request.lastSeen
					request.response <- nil
				}
			}
		}

	}()
}

func (h *HubConnection) Stop(err error) <-chan error {
	res := make(chan error)
	h.internalCommand <- hubConnectionInternalCloseRequest{
		reason:   err,
		response: res,
	}
	return res
}

func (h *HubConnection) Send(item interface{}, ctx ...context.Context) <-chan error {
	res := make(chan error)
	if len(ctx) > 1 {
		panic("wrong usage")
	}

	if len(ctx) == 1 {
		select {
		case <-ctx[0].Done():
			res <- ErrSendTimeout
		case h.internalCommand <- hubConnectionInternalSendRequest{
			item:     item,
			response: res,
		}:
		}
	} else {
		h.internalCommand <- hubConnectionInternalSendRequest{
			item:     item,
			response: res,
		}
	}

	return res
}

func (h *HubConnection) Status() <-chan HubConnectionRepr {
	res := make(chan HubConnectionRepr)
	if h.controlRoutineLock.TryAcquire(1) {
		go func() {
			res <- h.unsafeStatus()
			h.controlRoutineLock.Release(1)
		}()
	} else {
		h.internalCommand <- hubConnectionInternalStatusRequest{
			response: res,
		}
	}
	return res
}

func (h *HubConnection) unsafeStatus() HubConnectionRepr {
	var err string
	if h.err != nil {
		err = h.err.Error()
	}
	return HubConnectionRepr{
		LastSeen:           h.lastSeen,
		Err:                err,
		BroadcastDomain:    h.broadcastDomain,
		Connected:          h.connected,
		ConnectionGUID:     h.connectionGUID,
		OpenSendRequests:   h.openSendRequests,
		TotalSendRequests:  h.totalSendRequests,
		OpenCloseRequests:  h.openCloseRequests,
		TotalCloseRequests: h.totalCloseRequests,
		BufferRoutines:     h.bufferRoutines,
		ControlRoutines:    h.controlRoutines,
	}
}
