package notificationHub

import "time"

type NotificationHubOptions struct {
	SendTimeout *time.Duration
	SendBuffer  *int
	Debug       *bool
}
