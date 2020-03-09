package requestResponder

import (
	"context"
	"errors"
	"fmt"

	nh "github.com/dunv/connectionTools/notificationHub"
	"github.com/dunv/uhelpers"
	"github.com/dunv/ulog"
)

type RequestResponder struct {
	debug       bool
	responseHub *nh.NotificationHub
	requestHub  *nh.NotificationHub
}

func NewRequestResponder() *RequestResponder {
	var debug bool
	// debug = true
	return &RequestResponder{
		debug:       debug,
		responseHub: nh.NewNotificationHub(nh.NotificationHubOptions{Debug: uhelpers.PtrToBool(debug)}),
		requestHub:  nh.NewNotificationHub(nh.NotificationHubOptions{Debug: uhelpers.PtrToBool(debug)}),
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
		if r.debug {
			fmt.Println("     -> Unregister (cancelled)")
		}
		// use new context for unregistering
		err := r.requestHub.Unregister(guid, errors.New("aborted"), context.Background())
		if err != nil {
			ulog.Errorf("could not unregister (%s, should not happen -> unhandled)", err)
		}
		if r.debug {
			fmt.Println("     <- Unregister (cancelled)")
		}
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

				// here we have no way of handling errors -> discard them for now
				_, _ = r.responseHub.Notify(domain, response, ctx)
			}
		}
	}()
	return cancelFunc
}

func (r *RequestResponder) Request(domain string, request Request, ctxs ...context.Context) <-chan interface{} {
	if len(ctxs) != 0 && len(ctxs) != 1 {
		ulog.Panicf("incorrect usage (len(ctx) must be 1)")
	}

	// Default: wait forever
	ctx := context.Background()
	if len(ctxs) == 1 {
		ctx = ctxs[0]
	}

	possibleResponseChannel := make(chan interface{})
	subscriptionGUID := r.responseHub.Register(domain, possibleResponseChannel)
	matchedResponseChannel := make(chan interface{})
	go func() {
		for {
			select {
			case <-ctx.Done():
				matchedResponseChannel <- ctx.Err()
				if r.debug {
					fmt.Printf("     -> Unregister (%s)\n", ctx.Err())
				}
				// use new context for unregistering
				err := r.responseHub.Unregister(subscriptionGUID, ctx.Err(), context.Background())
				if err != nil {
					ulog.Errorf("could not unregister (%s, should not happen -> unhandled)", err)
				}
				if r.debug {
					fmt.Printf("     <- Unregister (%s)\n", ctx.Err())
				}
				return
			case possibleResponse := <-possibleResponseChannel:
				if request.Match(possibleResponse) {
					// important: return before unregistering
					matchedResponseChannel <- possibleResponse

					if r.debug {
						fmt.Println("     -> Unregister (matched)")
					}
					// use new context for unregistering
					err := r.responseHub.Unregister(subscriptionGUID, nil, context.Background())
					if err != nil {
						ulog.Errorf("could not unregister (%s, should not happen -> unhandled)", err)
					}
					if r.debug {
						fmt.Println("     <- Unregister (matched)")
					}
					return
				}
			}
		}
	}()

	go func() {
		sends, _ := r.requestHub.Notify(domain, request, ctx)
		if sends == 0 {
			matchedResponseChannel <- nh.ErrNoOneListeningToRequest
			if r.debug {
				fmt.Println("     -> Unregister (no one listening)")
			}
			// use new context for unregistering
			err := r.responseHub.Unregister(subscriptionGUID, nh.ErrNoOneListeningToRequest, context.Background())
			if err != nil {
				ulog.Errorf("could not unregister (%s, should not happen -> unhandled)", err)
			}

			if r.debug {
				fmt.Println("     <- Unregister (no one listening)")
			}
		}
	}()

	return matchedResponseChannel
}
