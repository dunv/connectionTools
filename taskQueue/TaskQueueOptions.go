package taskQueue

import (
	"context"
	"time"
)

type TaskQueueOption interface {
	apply(*taskQueueOptions)
}

type taskQueueOptions struct {
	maxRetries         int
	backoffInitial     time.Duration
	backoffFactor      int
	backoffLimit       time.Duration
	keepTaskReportsFor time.Duration
	successChannel     *chan struct{}
	failureChannel     *chan error
	priority           int
	ctx                context.Context
	startManually      bool
}

type funcTaskQueueOption struct {
	f func(*taskQueueOptions)
}

func (fdo *funcTaskQueueOption) apply(do *taskQueueOptions) {
	fdo.f(do)
}

func newFuncTaskQueueOption(f func(*taskQueueOptions)) *funcTaskQueueOption {
	return &funcTaskQueueOption{f: f}
}

func WithMaxRetries(maxRetries int) TaskQueueOption {
	return newFuncTaskQueueOption(func(o *taskQueueOptions) {
		o.maxRetries = maxRetries
	})
}
func WithBackOff(backoffInitial time.Duration, backoffFactor int, backoffLimit time.Duration) TaskQueueOption {
	return newFuncTaskQueueOption(func(o *taskQueueOptions) {
		o.backoffInitial = backoffInitial
		o.backoffFactor = backoffFactor
		o.backoffLimit = backoffLimit
	})
}

func WithTaskReportRetention(dur time.Duration) TaskQueueOption {
	return newFuncTaskQueueOption(func(o *taskQueueOptions) {
		o.keepTaskReportsFor = dur
	})
}

func WithSuccessChannel(c chan struct{}) TaskQueueOption {
	return newFuncTaskQueueOption(func(o *taskQueueOptions) {
		o.successChannel = &c
	})
}

func WithFailureChannel(c chan error) TaskQueueOption {
	return newFuncTaskQueueOption(func(o *taskQueueOptions) {
		o.failureChannel = &c
	})
}

func WithContext(c context.Context) TaskQueueOption {
	return newFuncTaskQueueOption(func(o *taskQueueOptions) {
		o.ctx = c
	})
}

func WithPriority(priority int) TaskQueueOption {
	return newFuncTaskQueueOption(func(o *taskQueueOptions) {
		o.priority = priority
	})
}

// for testing only
func withStartManually() TaskQueueOption {
	return newFuncTaskQueueOption(func(o *taskQueueOptions) {
		o.startManually = true
	})
}
