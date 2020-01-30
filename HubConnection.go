package connectionTools

import "time"

import "context"

type HubConnection struct {
	LastSeen        time.Time `json:"lastSeen,omitempty"`
	LastErr         error     `json:"lastErr,omitempty"`
	BroadcastDomain string    `json:"broadcastDomain"`
	Connected       bool      `json:"connected"`
	ConnectionGUID  string    `json:"connectionGuid"`

	sendChannel chan interface{}
	sendBuffer  chan interface{}
	cancel      context.CancelFunc
	sendContext context.Context
}

func (h HubConnection) Start() {
	go func() {
		for {
			select {
			case <-h.sendContext.Done():
				return
			case item := <-h.sendBuffer:
				select {
				case <-h.sendContext.Done():
					return
				case h.sendChannel <- item:
				}
			}
		}
	}()
}

func (h HubConnection) Stop() {
	if h.cancel != nil {
		h.cancel()
		h.cancel = nil
	}
}
