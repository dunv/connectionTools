package connectionTools

import (
	"context"
	"fmt"
	"time"
)

type HubConnection struct {
	lastSeen        time.Time
	err             error
	broadcastDomain string
	connected       bool
	connectionGUID  string

	sendChannel chan<- interface{}
	sendBuffer  *chan interface{}

	debug         bool
	internalSend  chan hubConnectionInternalSendRequest
	internalClose chan hubConnectionInternalCloseRequest
	ctx           context.Context
	cancel        context.CancelFunc

	// Statistics
	openSendRequests   int
	totalSendRequests  int
	openCloseRequests  int
	totalCloseRequests int
	bufferRoutines     int
	controlRoutines    int
}

type hubConnectionInternalSendRequest struct {
	item     interface{}
	response chan error
}

type hubConnectionInternalCloseRequest struct {
	reason   error
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

func (h *HubConnection) Status() HubConnectionRepr {
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
		lastSeen:        time.Now(),
		broadcastDomain: broadcastDomain,
		connected:       true,
		connectionGUID:  guid,
		sendChannel:     sendChannel,
		sendBuffer:      buffer,
		debug:           debug,
		internalSend:    make(chan hubConnectionInternalSendRequest),
		internalClose:   make(chan hubConnectionInternalCloseRequest),
		ctx:             ctx,
		cancel:          cancel,
	}
	conn.start()
	return &conn
}

func (h *HubConnection) start() {
	// Start sending routine
	if h.sendBuffer != nil {
		go func() {
			h.bufferRoutines++
			if h.bufferRoutines > 1 {
				fmt.Println("bufferRoutines", h.bufferRoutines)
			}
			for {
				select {
				case <-h.ctx.Done():
					h.bufferRoutines--
					return
				case item := <-*h.sendBuffer:
					select {
					case <-h.ctx.Done():
						return
					case h.sendChannel <- item:
						h.lastSeen = time.Now()
					}
				}
			}
		}()
	}

	// Start control routine
	go func() {
		h.controlRoutines++
		if h.controlRoutines > 1 {
			fmt.Println("controlRoutines", h.controlRoutines)
		}
		for {
			select {
			case <-h.ctx.Done():
				h.controlRoutines--
				return
			case req := <-h.internalSend:
				h.openSendRequests++
				h.totalSendRequests++
				// check for closed connection -> this should never happen?
				if !h.connected {
					req.response <- ErrConnectionClosed
					h.openSendRequests--
					continue
				}

				// Send into buffer if we have one
				if h.sendBuffer != nil {
					*h.sendBuffer <- req.item
					req.response <- nil
					h.openSendRequests--
					continue
				}

				// Send into actual channel
				h.sendChannel <- req.item
				h.lastSeen = time.Now()
				req.response <- nil
				h.openSendRequests--
			case req := <-h.internalClose:
				h.openCloseRequests++
				h.totalCloseRequests++
				h.connected = false
				h.err = req.reason
				h.cancel()
				req.response <- nil
				h.openCloseRequests--
			}
		}
	}()
}

func (h *HubConnection) Stop(err error) <-chan error {
	res := make(chan error)
	h.internalClose <- hubConnectionInternalCloseRequest{
		reason:   err,
		response: res,
	}
	return res
}

func (h *HubConnection) Send(item interface{}) <-chan error {
	res := make(chan error)
	h.internalSend <- hubConnectionInternalSendRequest{
		item:     item,
		response: res,
	}
	return res
}
