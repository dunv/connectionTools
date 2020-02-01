package connectionTools

import (
	"context"
	"fmt"

	"github.com/dunv/ulog"
)

type RequestResponder struct {
	responseHub *NotificationHub
	requestHub  *NotificationHub
}

func NewRequestResponder() *RequestResponder {
	return &RequestResponder{
		responseHub: NewNotificationHub(),
		requestHub:  NewNotificationHub(),
	}
}

func (r *RequestResponder) AddRequestChannel(domain string, requestChannel chan<- interface{}) context.CancelFunc {
	ctx, cancelFunc := context.WithCancel(context.Background())
	guid := r.requestHub.Register(domain, requestChannel)
	go func() {
		<-ctx.Done()
		r.requestHub.Unregister(guid, nil)
	}()
	return cancelFunc
}

func (r *RequestResponder) AddResponseChannel(domain string, responseChannel chan interface{}) context.CancelFunc {
	ctx, cancelFunc := context.WithCancel(context.Background())
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case response := <-responseChannel:
				// fmt.Println("<--- ", "response: ", response.(*BaseResponse).Data)
				ulog.LogIfErrorSecondArg(r.responseHub.Notify(domain, response))
			}
		}
	}()
	return cancelFunc
}

func (r *RequestResponder) Request(domain string, request Request, ctx ...context.Context) <-chan interface{} {
	if len(ctx) != 0 && len(ctx) != 1 {
		ulog.Panicf("incorrect usage (len(ctx) mus be 1)")
	}

	possibleResponseChannel := make(chan interface{})
	subscriptionGUID := r.responseHub.Register(domain, possibleResponseChannel)
	matchedResponseChannel := make(chan interface{})
	go func() {
		for {
			if len(ctx) == 1 {
				select {
				case <-ctx[0].Done():
					matchedResponseChannel <- ctx[0].Err()
					return
				case possibleResponse := <-possibleResponseChannel:
					if typed, ok := possibleResponse.(Response); ok {
						if request.Match(typed) {
							fmt.Println("received and match")
							r.responseHub.Unregister(subscriptionGUID, nil)
							matchedResponseChannel <- possibleResponse
							return
						}
					} else {
						ulog.Panicf("received wrong type when consuming responses %+V", possibleResponse)
					}
				}
			} else {
				possibleResponse := <-possibleResponseChannel
				if typed, ok := possibleResponse.(Response); ok {
					if request.Match(typed) {
						r.responseHub.Unregister(subscriptionGUID, nil)
						matchedResponseChannel <- possibleResponse
						return
					}
				} else {
					ulog.Panicf("received wrong type when consuming responses %+V", possibleResponse)
				}
			}
		}
	}()

	go func() {
		sends, _ := r.requestHub.Notify(domain, request)
		if sends == 0 {
			matchedResponseChannel <- ErrNoOneListeningToRequest
			close(matchedResponseChannel)
		}
	}()

	return matchedResponseChannel
}
