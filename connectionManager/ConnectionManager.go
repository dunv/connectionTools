package connectionManager

import (
	"errors"
	"sync"
)

// ConnectionManager is a container which manages dependent connection groups
// - every group has a name (group-name)
// - every connection has a name
// - a group is only connected, if all contained connections are connected AND all required metadata is present
// - a group is disconnected if a single contained connection is disconnected
type ConnectionManager struct {
	groups                   map[string]*ConnectionGroup
	groupRequiredConnections []string
	groupRequiredMetadata    []string
	lock                     *sync.Mutex
	changeChannel            *chan ConnectionChange
}

type ConnectionGroup struct {
	Connections map[string]bool        `json:"connections"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	Connected   bool                   `json:"connected"`
}

type ConnectionChange struct {
	Group     string `json:"group"`
	Connected bool   `json:"connected"`
}

var ErrGroupTemplateAlreadyExists = errors.New("groupTemplate already exists")
var ErrGroupAlreadyExists = errors.New("group already exists")
var ErrUnknownGroup = errors.New("unknown group")
var ErrUnknownMetadata = errors.New("unknown metadata")

func NewConnectionManager(groupRequiredConnections []string, groupRequiredMetadata []string, changeChannel ...*chan ConnectionChange) *ConnectionManager {
	var usedChannel *chan ConnectionChange
	if len(changeChannel) == 1 {
		usedChannel = changeChannel[0]
	}

	return &ConnectionManager{
		groups:                   map[string]*ConnectionGroup{},
		groupRequiredConnections: groupRequiredConnections,
		groupRequiredMetadata:    groupRequiredMetadata,
		lock:                     &sync.Mutex{},
		changeChannel:            usedChannel,
	}
}

func (c *ConnectionManager) Connect(group string, name string) {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.addGroupIfNotExists(group)
	c.groups[group].Connections[name] = true
	c.processChange(group)
}

func (c *ConnectionManager) Disconnect(group string, name string) {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.addGroupIfNotExists(group)
	c.groups[group].Connections[name] = false
	c.processChange(group)
}

func (c *ConnectionManager) SetMetadata(group string, name string, data interface{}) {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.addGroupIfNotExists(group)
	c.groups[group].Metadata[name] = data
	c.processChange(group)
}

func (c *ConnectionManager) GetMetadata(group string, name string) (interface{}, error) {
	c.lock.Lock()
	defer c.lock.Unlock()
	if group, ok := c.groups[group]; ok {
		if attr, ok := group.Metadata[name]; ok {
			return attr, nil
		}
		return nil, ErrUnknownMetadata
	}
	return nil, ErrUnknownGroup
}

func (c *ConnectionManager) IsGroupConnected(group string) bool {
	c.lock.Lock()
	defer c.lock.Unlock()
	return c.groups[group].Connected
}

func (c *ConnectionManager) Status() map[string]*ConnectionGroup {
	c.lock.Lock()
	defer c.lock.Unlock()

	copy := map[string]*ConnectionGroup{}
	for group, connection := range c.groups {
		connectionsCopy := map[string]bool{}
		for name, status := range connection.Connections {
			connectionsCopy[name] = status
		}
		copy[group] = &ConnectionGroup{
			Connections: connectionsCopy,
			Connected:   connection.Connected,
			// Metadata:    connection.Metadata,
		}
	}
	return copy
}

func (c *ConnectionManager) addGroupIfNotExists(group string) {
	if _, ok := c.groups[group]; !ok {
		c.groups[group] = &ConnectionGroup{
			Connections: map[string]bool{},
			Metadata:    map[string]interface{}{},
			Connected:   false,
		}
		for _, name := range c.groupRequiredConnections {
			c.groups[group].Connections[name] = false
		}

		for _, name := range c.groupRequiredMetadata {
			c.groups[group].Metadata[name] = nil
		}
	}
}

func (s *ConnectionManager) processChange(group string) {
	// Check if all requiredConnections are connected
	oneConnected := false
	allConnected := true
	for _, isConnected := range s.groups[group].Connections {
		if isConnected {
			oneConnected = true
		}
		if !isConnected {
			allConnected = false
		}
	}

	// Check if all requiredMetadata is present
	allMetadataPresent := true
	for _, val := range s.groups[group].Metadata {
		if val == nil {
			allMetadataPresent = false
		}
	}

	if allConnected && allMetadataPresent {
		// if everything is present, set to connected and notify interested parties if previous state was NOT connected
		if !s.groups[group].Connected && s.changeChannel != nil {
			*s.changeChannel <- ConnectionChange{Group: group, Connected: true}
		}

		s.groups[group].Connected = true
	} else if !oneConnected {
		// if not a single client is connected, consider this group disconnected, notify subscriber if it was connected previously
		if s.groups[group].Connected && s.changeChannel != nil {
			*s.changeChannel <- ConnectionChange{Group: group, Connected: false}
		}

		delete(s.groups, group)
	}
}
