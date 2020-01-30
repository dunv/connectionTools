package connectionTools

import (
	"context"
	"time"
)

type HubConnection struct {
	LastSeen        time.Time `json:"lastSeen,omitempty"`
	LastErr         error     `json:"lastErr,omitempty"`
	BroadcastDomain string    `json:"broadcastDomain"`
	Connected       bool      `json:"connected"`
	ConnectionGUID  string    `json:"connectionGuid"`

	sendChannel chan<- interface{}
	sendBuffer  *chan interface{}
	cancel      context.CancelFunc
	sendContext context.Context
}

func NewHubConnection(
	guid string,
	broadcastDomain string,
	sendChannel chan<- interface{},
	bufferSize int,
) *HubConnection {
	ctx, cancel := context.WithCancel(context.Background())

	var buffer *chan interface{}
	if bufferSize > 0 {
		tmp := make(chan interface{}, bufferSize-1)
		buffer = &tmp
	}

	conn := HubConnection{
		LastSeen:        time.Now(),
		BroadcastDomain: broadcastDomain,
		Connected:       true,
		ConnectionGUID:  guid,
		sendChannel:     sendChannel,
		sendBuffer:      buffer,
		sendContext:     ctx,
		cancel:          cancel,
	}
	conn.Start()
	return &conn
}

func (h HubConnection) Start() {
	if h.sendBuffer != nil {
		go func() {
			for h.sendContext.Err() == nil {
				select {
				case <-h.sendContext.Done():
					h.Connected = false
					h.LastErr = h.sendContext.Err()
					return
				case item := <-*h.sendBuffer:
					select {
					case <-h.sendContext.Done():
						h.Connected = false
						h.LastErr = h.sendContext.Err()
						return
					case h.sendChannel <- item:
						h.LastSeen = time.Now()
					}
				}
			}
		}()
	}
}

func (h HubConnection) Stop(err error) {
	h.LastErr = err
	h.Connected = false
	if h.cancel != nil {
		h.cancel()
		h.cancel = nil
	}
}

func (h HubConnection) Send(item interface{}) <-chan error {
	res := make(chan error)
	go func() {
		if !h.Connected {
			res <- ErrConnectionClosed
			return
		}

		if h.sendBuffer != nil {
			*h.sendBuffer <- item
			res <- nil
			return
		}

		h.sendChannel <- item
		h.LastSeen = time.Now()
		res <- nil

	}()
	return res
}
