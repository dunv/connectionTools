package taskQueue

import (
	"context"
	"time"
)

type TaskQueueOption interface {
	apply(*taskQueueOptions)
}

type taskQueueOptions struct {
	maxRetries             int
	backoffInitial         time.Duration
	backoffFactor          int
	backoffLimit           time.Duration
	keepTaskReportsFor     time.Duration
	successChannel         *chan struct{}
	failureChannel         *chan error
	priority               int
	ctx                    context.Context
	startManually          bool
	timeout                *time.Duration
	timeoutCheckerInterval *time.Duration
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

// A task will be retried this amount of times
// Default is no retry
func WithMaxRetries(maxRetries int) TaskQueueOption {
	return newFuncTaskQueueOption(func(o *taskQueueOptions) {
		o.maxRetries = maxRetries
	})
}

// Configure exponential backoff (if tasks are retried)
// Default is no backoff
func WithBackOff(backoffInitial time.Duration, backoffFactor int, backoffLimit time.Duration) TaskQueueOption {
	return newFuncTaskQueueOption(func(o *taskQueueOptions) {
		o.backoffInitial = backoffInitial
		o.backoffFactor = backoffFactor
		o.backoffLimit = backoffLimit
	})
}

// Configure retention of statusReports (default is 1 day)
func WithTaskReportRetention(dur time.Duration) TaskQueueOption {
	return newFuncTaskQueueOption(func(o *taskQueueOptions) {
		o.keepTaskReportsFor = dur
	})
}

// Pass a channel, which gets notified if a task has been completed successfully
func WithSuccessChannel(c chan struct{}) TaskQueueOption {
	return newFuncTaskQueueOption(func(o *taskQueueOptions) {
		o.successChannel = &c
	})
}

// Pass a channel, which gets notified if a task has returned an error
func WithFailureChannel(c chan error) TaskQueueOption {
	return newFuncTaskQueueOption(func(o *taskQueueOptions) {
		o.failureChannel = &c
	})
}

// Pass a context to a task
func WithContext(c context.Context) TaskQueueOption {
	return newFuncTaskQueueOption(func(o *taskQueueOptions) {
		o.ctx = c
	})
}

// Pass a default priority for tasks
// individual priorities (relative to this one)
// can be passed when queuing a task
// Default is 1000
func WithDefaultPriority(priority int) TaskQueueOption {
	return newFuncTaskQueueOption(func(o *taskQueueOptions) {
		o.priority = priority
	})
}

// Pass a priority for a task
// TODO: find concept for separating options for the whole queue and individual tasks
func WithPriority(priority int) TaskQueueOption {
	return newFuncTaskQueueOption(func(o *taskQueueOptions) {
		o.priority = priority
	})
}

// Pass a timeout
// Default is no timeout
func WithTimeout(timeout time.Duration) TaskQueueOption {
	return newFuncTaskQueueOption(func(o *taskQueueOptions) {
		o.timeout = &timeout
	})
}

// Enable a "sanity-check" routine which runs next to a task and logs, if the task is running longer than its timeout
// Default is no checker interval
func WithTimeoutCheckerInterval(interval time.Duration) TaskQueueOption {
	return newFuncTaskQueueOption(func(o *taskQueueOptions) {
		o.timeoutCheckerInterval = &interval
	})
}

// for testing only
func withStartManually() TaskQueueOption {
	return newFuncTaskQueueOption(func(o *taskQueueOptions) {
		o.startManually = true
	})
}
