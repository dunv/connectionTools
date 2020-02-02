package connectionTools

import (
	"context"

	"github.com/dunv/uhelpers"
	"github.com/dunv/ulog"
)

type RequestResponder struct {
	responseHub *NotificationHub
	requestHub  *NotificationHub
}

func NewRequestResponder() *RequestResponder {
	return &RequestResponder{
		responseHub: NewNotificationHub(NotificationHubOptions{Debug: uhelpers.PtrToBool(false)}),
		requestHub:  NewNotificationHub(NotificationHubOptions{Debug: uhelpers.PtrToBool(false)}),
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
					if request.Match(possibleResponse) {
						// important: return before unregistering
						matchedResponseChannel <- possibleResponse

						// fmt.Println("     -> Unregister")
						r.responseHub.UnregisterBlocking(subscriptionGUID, nil)
						// fmt.Println("     <- Unregister")
						return
					}
				}
			} else {
				possibleResponse := <-possibleResponseChannel
				if request.Match(possibleResponse) {
					// important: return before unregistering
					matchedResponseChannel <- possibleResponse

					// fmt.Println("     -> Unregister")
					r.responseHub.UnregisterBlocking(subscriptionGUID, nil)
					// fmt.Println("     <- Unregister")
					return
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
