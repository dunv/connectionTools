package notificationHub

import (
	"time"
)

type NotificationHubOption interface {
	apply(*notificationHubOptions)
}

type notificationHubOptions struct {
	sendTimeout time.Duration
	sendBuffer  int
	debug       bool
}

type funcNotificationHubOption struct {
	f func(*notificationHubOptions)
}

func (fdo *funcNotificationHubOption) apply(do *notificationHubOptions) {
	fdo.f(do)
}

func newFuncNotificationHubOption(f func(*notificationHubOptions)) *funcNotificationHubOption {
	return &funcNotificationHubOption{f: f}
}

func WithSendTimeout(timeout time.Duration) NotificationHubOption {
	return newFuncNotificationHubOption(func(o *notificationHubOptions) {
		o.sendTimeout = timeout
	})
}

func WithSendBuffer(buffer int) NotificationHubOption {
	return newFuncNotificationHubOption(func(o *notificationHubOptions) {
		o.sendBuffer = buffer
	})
}

func WithDebug(debug bool) NotificationHubOption {
	return newFuncNotificationHubOption(func(o *notificationHubOptions) {
		o.debug = debug
	})
}
