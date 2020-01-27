package connectionTools

import "time"

type HubConnection struct {
	LastSeen        time.Time         `json:"lastSeen,omitempty"`
	LastErr         error             `json:"lastErr,omitempty"`
	BroadcastDomain string            `json:"broadcastDomain,omitempty"`
	Connected       bool              `json:"connected"`
	ConnectionGUID  string            `json:"connectionGuid,omitempty"`
	Channel         *chan interface{} `json:"channel,omitempty"`
}
