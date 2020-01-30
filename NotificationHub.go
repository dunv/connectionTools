package connectionTools

import (
	"context"
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
	}

	return &NotificationHub{
		connections: map[string]*HubConnection{},
		connMap:     map[string][]string{},
		options:     optsWithDefaults,
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

	buffer := make(chan interface{})
	if s.options.SendBuffer != nil {
		buffer = make(chan interface{}, *s.options.SendBuffer)
	}

	guid := uuid.New().String()
	ctx, cancel := context.WithCancel(context.Background())
	s.connections[guid] = &HubConnection{
		LastSeen:        time.Now(),
		BroadcastDomain: broadcastDomain,
		Connected:       true,
		ConnectionGUID:  guid,
		sendChannel:     channel,
		sendBuffer:      buffer,
		sendContext:     ctx,
		cancel:          cancel,
	}
	s.connections[guid].Start()

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
		conn.Stop()
		close(conn.sendChannel)
		close(conn.sendBuffer)
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
				if s.options.SendTimeout != nil {
					// Send with timeout
					select {
					case <-time.After(*s.options.SendTimeout):
						fmt.Println("timeout reached")
						s.unregister(connGUID, ErrSendTimeout)
						errs[connGUID] = ErrSendTimeout
					case conn.sendBuffer <- data:
						conn.LastSeen = time.Now()
						successfulSends++
					}
				} else {
					// Send without timeout
					conn.sendBuffer <- data
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
