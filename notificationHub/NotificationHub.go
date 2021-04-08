package notificationHub

import (
	"context"
	"fmt"

	"github.com/dunv/uhelpers"
	"github.com/google/uuid"
	"golang.org/x/sync/semaphore"
)

type NotificationHub struct {
	connections map[string]*HubConnection
	connMap     map[string][]string
	connLock    *semaphore.Weighted
	options     notificationHubOptions
}

type NotificationHubStatus struct {
	Connections []HubConnectionRepr `json:"connections"`
	Registry    map[string][]string `json:"registry"`
}

func (h NotificationHubStatus) String() string {
	conns := []string{}
	for _, conn := range h.Connections {
		if conn.Connected {
			conns = append(conns, conn.String())
		}
	}
	registry := []string{}
	for k, v := range h.Registry {
		registry = append(registry, fmt.Sprintf("[%s:%d]", k, len(v)))
	}

	return fmt.Sprintf("Hub[connections: %s, registry: %s]", conns, registry)
}

func NewNotificationHub(opts ...NotificationHubOption) *NotificationHub {
	// Default options
	mergedOpts := notificationHubOptions{
		sendTimeout: 0,
		sendBuffer:  0,
		debug:       false,
	}
	for _, opt := range opts {
		opt.apply(&mergedOpts)
	}

	return &NotificationHub{
		connections: map[string]*HubConnection{},
		connMap:     map[string][]string{},
		options:     mergedOpts,
		connLock:    semaphore.NewWeighted(1),
	}
}

func (s *NotificationHub) Status(ctx context.Context) (*NotificationHubStatus, error) {
	if s.connLock.Acquire(ctx, 1) == nil {
		conns := []HubConnectionRepr{}
		for _, conn := range s.connections {
			repr := <-conn.Status(ctx)
			switch typed := repr.(type) {
			case HubConnectionRepr:
				// do NOT access connected on the connection directly
				if typed.Connected {
					conns = append(conns, typed)
				}
			case error:
				return nil, ErrConnectionLocked
			}
		}
		registry := map[string][]string{}
		for k, v := range s.connMap {
			i := append([]string{}, v...)
			registry[k] = i
		}

		s.connLock.Release(1)
		return &NotificationHubStatus{
			Connections: conns,
			Registry:    registry,
		}, nil
	} else {
		return nil, ErrHubLocked
	}
}

func (s *NotificationHub) Register(broadcastDomain string, channel chan<- interface{}) string {
	if s.options.debug {
		fmt.Println("-> register")
	}
	err := s.connLock.Acquire(context.Background(), 1)
	if err != nil {
		panic(err)
	}
	if s.options.debug {
		fmt.Println("   register")
	}

	guid := uuid.New().String()
	s.connections[guid] = NewHubConnection(guid, broadcastDomain, channel, s.options.sendBuffer, s.options.debug)

	// update registry
	if _, ok := s.connMap[broadcastDomain]; !ok {
		s.connMap[broadcastDomain] = []string{guid}
	} else {
		s.connMap[broadcastDomain] = append(s.connMap[broadcastDomain], guid)
	}

	s.connLock.Release(1)
	if s.options.debug {
		fmt.Println("   register ->")
	}
	return guid
}

// Unregister from Hub
func (s *NotificationHub) Unregister(connectionGUID string, reason error, ctx context.Context) error {
	if s.options.debug {
		fmt.Println("-> unregister   ")
	}

	err := s.connLock.Acquire(ctx, 1)
	if err != nil {
		return err
	}

	if s.options.debug {
		fmt.Println("   unregister   ")
	}
	s.unregister(connectionGUID, reason)

	s.connLock.Release(1)
	if s.options.debug {
		fmt.Println("   unregister ->")
	}

	return nil
}

func (s *NotificationHub) unregister(connectionGUID string, reason error) {
	if s.options.debug {
		if reason != nil {
			fmt.Printf("unregistering %s (%s) \n", connectionGUID, reason)
		} else {
			fmt.Printf("unregistering %s (no reason given) \n", connectionGUID)
		}
	}
	if conn, ok := s.connections[connectionGUID]; ok {
		<-conn.Stop(reason)

		allConns := s.connMap[conn.BroadcastDomain()]
		i, _ := uhelpers.StringIndexOf(allConns, conn.ConnectionGUID())
		if i > -1 {
			allConns[i] = allConns[len(allConns)-1]
			allConns[len(allConns)-1] = ""
			allConns = allConns[:len(allConns)-1]
			s.connMap[conn.BroadcastDomain()] = allConns
			if len(allConns) == 0 {
				delete(s.connMap, conn.BroadcastDomain())
			}
		}
	}
}

func (s *NotificationHub) Notify(broadcastDomain string, data interface{}, ctxs ...context.Context) (int, error) {
	if len(ctxs) > 1 {
		panic("wrong usage")
	}

	// Default: wait forever
	ctx := context.Background()
	if len(ctxs) == 1 {
		// If given explicitly: highest precedence
		ctx = ctxs[0]
	} else if s.options.sendTimeout != 0 {
		// If not given but configured -> assign
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, s.options.sendTimeout)
		defer cancel()
	}

	if s.options.debug {
		fmt.Println("-> notify     ", data)
	}
	err := s.connLock.Acquire(ctx, 1)
	if err != nil {
		return 0, err
	}

	if s.options.debug {
		fmt.Println("   notify     ", data)
	}
	errs := map[string]error{}
	successfulSends := 0

	if connGUIDs, ok := s.connMap[broadcastDomain]; ok {
		for _, connGUID := range connGUIDs {
			if conn, ok := s.connections[connGUID]; ok && conn.Connected() {
				err := <-conn.Send(data, ctx)
				if err != nil {
					// s.unregister(connGUID, ErrSendTimeout)
					errs[connGUID] = ErrSendTimeout
				} else {
					successfulSends++
				}
			}
		}
	}

	s.connLock.Release(1)
	if s.options.debug {
		fmt.Println("   notify ->  ", data)
	}

	if len(errs) != 0 {
		return successfulSends, ErrNotAllReachable{ErrMap: errs}
	}

	return successfulSends, nil
}
