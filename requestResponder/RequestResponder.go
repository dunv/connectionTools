package requestResponder

import (
	"context"

	nh "github.com/dunv/connectionTools/notificationHub"
	"github.com/dunv/uhelpers"
	"github.com/dunv/ulog"
)

type RequestResponder struct {
	responseHub *nh.NotificationHub
	requestHub  *nh.NotificationHub
}

func NewRequestResponder() *RequestResponder {
	return &RequestResponder{
		responseHub: nh.NewNotificationHub(nh.NotificationHubOptions{Debug: uhelpers.PtrToBool(false)}),
		requestHub:  nh.NewNotificationHub(nh.NotificationHubOptions{Debug: uhelpers.PtrToBool(false)}),
	}
}

func (r *RequestResponder) Status(ctx context.Context) (map[string]nh.NotificationHubStatus, error) {
	requestHubStatus, err := r.requestHub.Status(ctx)
	if err != nil {
		return nil, err
	}

	responseHubStatus, err := r.responseHub.Status(ctx)
	if err != nil {
		return nil, err
	}

	return map[string]nh.NotificationHubStatus{
		"requestHub":  *requestHubStatus,
		"responseHub": *responseHubStatus,
	}, nil
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
			matchedResponseChannel <- nh.ErrNoOneListeningToRequest
			// close(matchedResponseChannel)
		}
	}()

	return matchedResponseChannel
}
