package connectionTools

import (
	"context"
	"fmt"
	"sync"
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
	cancel      context.CancelFunc
	sendContext context.Context

	lock  sync.Mutex
	debug bool
}

type HubConnectionRepr struct {
	LastSeen        time.Time `json:"lastSeen,omitempty"`
	Err             string    `json:"err,omitempty"`
	BroadcastDomain string    `json:"broadcastDomain"`
	Connected       bool      `json:"connected"`
	ConnectionGUID  string    `json:"connectionGuid"`
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
		sendContext:     ctx,
		cancel:          cancel,
		debug:           debug,
	}
	conn.start()
	return &conn
}

func (h *HubConnection) start() {
	if h.sendBuffer != nil {
		go func() {
			for h.sendContext.Err() == nil {
				select {
				case <-h.sendContext.Done():
					h.connected = false
					h.err = h.sendContext.Err()
					return
				case item := <-*h.sendBuffer:
					select {
					case <-h.sendContext.Done():
						h.connected = false
						h.err = h.sendContext.Err()
						return
					case h.sendChannel <- item:
						h.lastSeen = time.Now()
					}
				}
			}
		}()
	}
}

func (h *HubConnection) Stop(err error) {
	if h.debug {
		fmt.Println("           -> stop   ")
	}
	h.lock.Lock()
	if h.debug {
		fmt.Println("              stop   ")
	}
	defer h.lock.Unlock()

	h.err = err
	h.connected = false
	if h.cancel != nil {
		h.cancel()
		h.cancel = nil
	}
	if h.debug {
		fmt.Println("              stop -> ")
	}
}

func (h *HubConnection) Send(item interface{}) <-chan error {
	res := make(chan error)
	go func() {
		if h.debug {
			fmt.Println("           -> send    ")
		}
		h.lock.Lock()
		if h.debug {
			fmt.Println("              send    ")
		}
		if !h.connected {
			res <- ErrConnectionClosed
			h.lock.Unlock()
			return
		}
		h.lock.Unlock()
		if h.debug {
			fmt.Println("              send -> ")
		}

		if h.sendBuffer != nil {
			*h.sendBuffer <- item
			res <- nil
			return
		}

		h.sendChannel <- item
		if h.debug {
			fmt.Println("           -> send    ")
		}
		h.lock.Lock()
		if h.debug {
			fmt.Println("              send    ")
		}
		h.lastSeen = time.Now()
		h.lock.Unlock()
		if h.debug {
			fmt.Println("              send -> ")
		}
		res <- nil

	}()
	return res
}
