package requestResponder

import (
	"context"
	"errors"
	"fmt"
	"sync"

	nh "github.com/dunv/connectionTools/notificationHub"
)

type RequestResponder struct {
	options     requestResponderOptions
	responseHub *nh.NotificationHub
	requestHub  *nh.NotificationHub
}

func NewRequestResponder(opts ...RequestResponderOption) *RequestResponder {
	mergedOpts := requestResponderOptions{
		debug: false,
	}
	for _, opt := range opts {
		opt.apply(&mergedOpts)
	}

	return &RequestResponder{
		options:     mergedOpts,
		responseHub: nh.NewNotificationHub(nh.WithDebug(mergedOpts.debug)),
		requestHub:  nh.NewNotificationHub(nh.WithDebug(mergedOpts.debug)),
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
		if r.options.debug {
			fmt.Println("     -> Unregister (cancelled)")
		}
		// use new context for unregistering
		err := r.requestHub.Unregister(guid, errors.New("aborted"), context.Background())
		if err != nil {
			fmt.Printf("could not unregister (%s, should not happen -> unhandled)\n", err)
		}
		if r.options.debug {
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
		panic("incorrect usage (len(ctx) must be 1)")
	}

	// Default: wait forever
	ctx := context.Background()
	// If context was passed: use that one
	if len(ctxs) == 1 {
		ctx = ctxs[0]
	}

	possibleResponseChannel := make(chan interface{})
	subscriptionGUID := r.responseHub.Register(domain, possibleResponseChannel)
	matchedResponseChannel := make(chan interface{})

	// Exctract this so unregistering only takes place once
	// Since we are running two goRoutines we cannot know whose <-ctx.Done() check will return first
	// -> this needs to be synchronized for channel closing
	var doOnce sync.Once
	var replyAndUnregisterOnce = func(response interface{}, err error) {
		doOnce.Do(func() {
			matchedResponseChannel <- response
			close(matchedResponseChannel)

			if r.options.debug {
				fmt.Printf("     -> Unregister (%s)\n", err)
			}
			// use new context for unregistering
			unregisterErr := r.responseHub.Unregister(subscriptionGUID, err, context.Background())
			if unregisterErr != nil {
				fmt.Printf("could not unregister (%s, should not happen -> unhandled)\n", err)
			}
			if r.options.debug {
				fmt.Printf("     <- Unregister (%s)\n", err)
			}
		})
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				replyAndUnregisterOnce(ctx.Err(), ctx.Err())
				return
			case possibleResponse := <-possibleResponseChannel:
				if request.Match(possibleResponse) {
					replyAndUnregisterOnce(possibleResponse, nil)
					return
				}
			}
		}
	}()

	go func() {
		sends, _ := r.requestHub.Notify(domain, request, ctx)
		if sends == 0 {
			replyAndUnregisterOnce(nh.ErrNoOneListeningToRequest, nh.ErrNoOneListeningToRequest)
		}
	}()

	return matchedResponseChannel
}
