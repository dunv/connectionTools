package sendQueue

import "time"

// trying to use grpc options api...

type SendQueueOption interface {
	apply(*sendQueueOptions)
}

type sendQueueOptions struct {
	maxRetries         int
	backoffInitial     time.Duration
	backoffFactor      int
	backoffLimit       time.Duration
	keepSendReportsFor time.Duration
}

type funcSendQueueOption struct {
	f func(*sendQueueOptions)
}

func (fdo *funcSendQueueOption) apply(do *sendQueueOptions) {
	fdo.f(do)
}

func newFuncSendQueueOption(f func(*sendQueueOptions)) *funcSendQueueOption {
	return &funcSendQueueOption{f: f}
}

func WithMaxRetries(maxRetries int) SendQueueOption {
	return newFuncSendQueueOption(func(o *sendQueueOptions) {
		o.maxRetries = maxRetries
	})
}
func WithBackOff(backoffInitial time.Duration, backoffFactor int, backoffLimit time.Duration) SendQueueOption {
	return newFuncSendQueueOption(func(o *sendQueueOptions) {
		o.backoffInitial = backoffInitial
		o.backoffFactor = backoffFactor
		o.backoffLimit = backoffLimit
	})
}

func WithSendReportRetention(dur time.Duration) SendQueueOption {
	return newFuncSendQueueOption(func(o *sendQueueOptions) {
		o.keepSendReportsFor = dur
	})
}
