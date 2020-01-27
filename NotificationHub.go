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
	sendTimeout time.Duration
}

func NewNotificationHub(sendTimeout time.Duration) *NotificationHub {
	return &NotificationHub{
		connections: map[string]*HubConnection{},
		connMap:     map[string][]string{},
		sendTimeout: sendTimeout,
	}
}

func (s *NotificationHub) Connections() []HubConnection {
	s.connLock.Lock()
	defer s.connLock.Unlock()

	copy := []HubConnection{}
	for _, conn := range s.connections {
		copy = append(copy, HubConnection{
			LastSeen:        conn.LastSeen,
			LastErr:         conn.LastErr,
			BroadcastDomain: conn.BroadcastDomain,
			Connected:       conn.Connected,
			ConnectionGUID:  conn.ConnectionGUID,
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
	s.connections[guid] = &HubConnection{
		LastSeen:        time.Now(),
		BroadcastDomain: broadcastDomain,
		Connected:       true,
		ConnectionGUID:  guid,
		Channel:         &channel,
	}

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
		close(*conn.Channel)
		conn.Connected = false
		conn.LastErr = reason

		allConns := s.connMap[conn.BroadcastDomain]
		i, _ := uhelpers.StringIndexOf(allConns, conn.ConnectionGUID)
		if i > -1 {
			allConns[i] = allConns[len(allConns)-1]
			allConns[len(allConns)-1] = ""
			allConns = allConns[:len(allConns)-1]
			s.connMap[conn.BroadcastDomain] = allConns
		}
	}
}

func (s *NotificationHub) Notify(broadcastDomain string, data interface{}) (int, map[string]error) {
	s.connLock.Lock()
	defer s.connLock.Unlock()

	errs := map[string]error{}
	successfulSends := 0
	if connGUIDs, ok := s.connMap[broadcastDomain]; ok {
		for _, connGUID := range connGUIDs {
			if conn, ok := s.connections[connGUID]; ok && conn.Connected {
				select {
				case <-time.After(s.sendTimeout):
					s.unregister(connGUID, ErrSendTimeout)
					errs[connGUID] = ErrSendTimeout
				case *conn.Channel <- data:
					conn.LastSeen = time.Now()
					successfulSends++
				}
			}
		}
	}

	if len(errs) != 0 {
		return successfulSends, errs
	}

	return successfulSends, nil
}
