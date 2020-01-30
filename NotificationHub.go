package connectionTools

import (
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
	defer s.connLock.Unlock()
	return s.connMap
}

func (s *NotificationHub) Register(broadcastDomain string, channel chan interface{}) string {
	s.connLock.Lock()
	defer s.connLock.Unlock()

	guid := uuid.New().String()
	s.connections[guid] = NewHubConnection(guid, broadcastDomain, channel, *s.options.SendBuffer)

	// update registry
	if _, ok := s.connMap[broadcastDomain]; !ok {
		s.connMap[broadcastDomain] = []string{guid}
	} else {
		s.connMap[broadcastDomain] = append(s.connMap[broadcastDomain], guid)
	}

	return guid
}

func (s *NotificationHub) Unregister(connectionGUID string, reason error) {
	s.connLock.Lock()
	defer s.connLock.Unlock()
	s.unregister(connectionGUID, reason)
}

func (s *NotificationHub) unregister(connectionGUID string, reason error) {
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
	s.connLock.Lock()
	defer s.connLock.Unlock()

	errs := map[string]error{}
	successfulSends := 0
	if connGUIDs, ok := s.connMap[broadcastDomain]; ok {
		for _, connGUID := range connGUIDs {
			if conn, ok := s.connections[connGUID]; ok && conn.Connected() {
				if s.options.SendTimeout != nil {
					// Send with timeout
					select {
					case <-time.After(*s.options.SendTimeout):
						s.unregister(connGUID, ErrSendTimeout)
						errs[connGUID] = ErrSendTimeout
					case err := <-conn.Send(data):
						if err != nil {
							s.unregister(connGUID, ErrSendTimeout)
							errs[connGUID] = ErrSendTimeout
						} else {
							successfulSends++
						}
					}
				} else {
					// Send without timeout
					err := <-conn.Send(data)
					if err != nil {
						s.unregister(connGUID, ErrSendTimeout)
						errs[connGUID] = ErrSendTimeout
					} else {
						successfulSends++
					}
				}
			}
		}
	}

	if len(errs) != 0 {
		return successfulSends, ErrNotAllReachable{ErrMap: errs}
	}

	return successfulSends, nil
}
