package connectionTools

import (
	"fmt"
	"sync"
	"time"

	"github.com/dunv/uhelpers"
	"github.com/google/uuid"
)

type NotificationHub struct {
	connections map[string]*HubConnection
	connMap     map[string][]string
	connLock    sync.Mutex
	options     NotificationHubOptions
}

func NewNotificationHub(opts ...NotificationHubOptions) *NotificationHub {
	optsWithDefaults := NotificationHubOptions{}
	if len(opts) > 0 && opts[0].SendTimeout != nil {
		optsWithDefaults.SendTimeout = opts[0].SendTimeout
	}

	if len(opts) > 0 && opts[0].SendBuffer != nil {
		optsWithDefaults.SendBuffer = opts[0].SendBuffer
	} else {
		optsWithDefaults.SendBuffer = uhelpers.PtrToInt(0)
	}

	if len(opts) > 0 && opts[0].Debug != nil {
		optsWithDefaults.Debug = opts[0].Debug
	} else {
		optsWithDefaults.Debug = uhelpers.PtrToBool(false)
	}

	return &NotificationHub{
		connections: map[string]*HubConnection{},
		connMap:     map[string][]string{},
		options:     optsWithDefaults,
	}
}

func (s *NotificationHub) Connections() []HubConnectionRepr {
	s.connLock.Lock()
	defer s.connLock.Unlock()

	copy := []HubConnectionRepr{}
	for _, conn := range s.connections {
		var err string
		if conn.Err() != nil {
			err = conn.Err().Error()
		}
		copy = append(copy, HubConnectionRepr{
			LastSeen:        conn.LastSeen(),
			Err:             err,
			BroadcastDomain: conn.BroadcastDomain(),
			Connected:       conn.Connected(),
			ConnectionGUID:  conn.ConnectionGUID(),
		})
	}

	return copy
}

func (s *NotificationHub) Registry() map[string][]string {
	s.connLock.Lock()
	copy := map[string][]string{}
	for k, v := range s.connMap {
		i := append([]string{}, v...)
		copy[k] = i
	}

	defer s.connLock.Unlock()
	return copy
}

func (s *NotificationHub) Register(broadcastDomain string, channel chan<- interface{}) string {
	if *s.options.Debug {
		fmt.Println("-> register")
	}
	s.connLock.Lock()
	defer s.connLock.Unlock()

	guid := uuid.New().String()
	s.connections[guid] = NewHubConnection(guid, broadcastDomain, channel, *s.options.SendBuffer, *s.options.Debug)
	if *s.options.Debug {
		fmt.Println("   register")
	}

	// update registry
	if _, ok := s.connMap[broadcastDomain]; !ok {
		s.connMap[broadcastDomain] = []string{guid}
	} else {
		s.connMap[broadcastDomain] = append(s.connMap[broadcastDomain], guid)
	}

	if *s.options.Debug {
		fmt.Println("   register ->")
	}
	return guid
}

// Unregister from hub
// this is asyncronous so we cannot get stuck in a deadlock, when we
// - listen to messages
// - want to unregister when we received the one we wanted
// - the next message is already being sent onto the channel
// 	   -> notify blocks (locking) until the outgoing channel is read
//     -> if we unregister in the same goroutine as we are reading the outgoing-channel
// 	   -> deadlock
func (s *NotificationHub) Unregister(connectionGUID string, reason error) {
	go func() {
		s.Unregister(connectionGUID, reason)
	}()
}

// UnregisterBlocking from hub. ONLY USE THIS IF YOU KNOW WHAT YOU ARE DOING
// This is mainly included for testing and special cases where we need to be sure, that
// no more messages are sent after this call is through
func (s *NotificationHub) UnregisterBlocking(connectionGUID string, reason error) {
	if *s.options.Debug {
		fmt.Println("-> unregister   ")
	}

	s.connLock.Lock()
	if *s.options.Debug {
		fmt.Println("   unregister   ")
	}
	s.unregister(connectionGUID, reason)

	s.connLock.Unlock()
	if *s.options.Debug {
		fmt.Println("   unregister ->")
	}
}

func (s *NotificationHub) unregister(connectionGUID string, reason error) {
	if *s.options.Debug {
		fmt.Printf("unregistering %s (%s) \n", connectionGUID, reason)
	}
	if conn, ok := s.connections[connectionGUID]; ok {
		conn.Stop(reason)

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

func (s *NotificationHub) Notify(broadcastDomain string, data interface{}) (int, error) {
	if *s.options.Debug {
		fmt.Println("-> notify     ", data)
	}
	s.connLock.Lock()
	if *s.options.Debug {
		fmt.Println("   notify     ", data)
	}
	errs := map[string]error{}
	successfulSends := 0

	if connGUIDs, ok := s.connMap[broadcastDomain]; ok {
		for _, connGUID := range connGUIDs {
			if conn, ok := s.connections[connGUID]; ok && conn.Connected() {
				if s.options.SendTimeout != nil {
					// Send with timeout
					select {
					case <-time.After(*s.options.SendTimeout):
						s.unregister(conn.ConnectionGUID(), ErrSendTimeout)
						errs[conn.ConnectionGUID()] = ErrSendTimeout
					case err := <-conn.Send(data):
						if err != nil {
							s.unregister(conn.ConnectionGUID(), ErrSendTimeout)
							errs[conn.ConnectionGUID()] = ErrSendTimeout
						} else {
							successfulSends++
						}
					}
				} else {
					// Send without timeout
					err := <-conn.Send(data)
					if err != nil {
						s.unregister(conn.ConnectionGUID(), ErrSendTimeout)
						errs[conn.ConnectionGUID()] = ErrSendTimeout
					} else {
						successfulSends++
					}
				}
			}
		}
	}

	s.connLock.Unlock()
	if *s.options.Debug {
		fmt.Println("   notify ->  ", data)
	}

	if len(errs) != 0 {
		return successfulSends, ErrNotAllReachable{ErrMap: errs}
	}

	return successfulSends, nil
}
