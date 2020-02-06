package sendQueue

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/dunv/concurrentList"
	"github.com/dunv/ulog"
)

type SendQueueFunc func(interface{}) error

type SendReport struct {
	Time     time.Time
	Item     interface{}
	Duration time.Duration
	Retries  int
	Success  bool
}

type SendQueue struct {
	list *concurrentList.ConcurrentList
	send func(interface{}) error
	ctx  context.Context

	opts sendQueueOptions

	// statistics
	sendInProgress         bool
	successfulSends        int
	successfulSendsByRetry map[int]int
	failedSends            int
	sendReports            *concurrentList.ConcurrentList

	lock sync.Mutex
}

func NewSendQueue(send SendQueueFunc, ctx context.Context, opts ...SendQueueOption) *SendQueue {
	mergedOpts := sendQueueOptions{
		keepSendReportsFor: 24 * time.Hour,
		maxRetries:         -1,
		backoffInitial:     0,
		backoffFactor:      1,
		backoffLimit:       1,
	}
	for _, opt := range opts {
		opt.apply(&mergedOpts)
	}

	queue := &SendQueue{
		list:                   concurrentList.NewConcurrentList(),
		send:                   send,
		ctx:                    ctx,
		opts:                   mergedOpts,
		successfulSendsByRetry: make(map[int]int),
		sendReports:            concurrentList.NewConcurrentList(),
	}
	go queue.run()
	return queue
}

func (p *SendQueue) Status() SendQueueStatus {
	p.lock.Lock()
	defer p.lock.Unlock()

	return statusFromSendQueue(p)
}

func (p *SendQueue) Length() int {
	return p.list.Length()
}

func (p *SendQueue) Push(item interface{}) {
	p.list.Append(item)
}

func (p *SendQueue) run() {
	for {
		// check if context is done yet
		select {
		case <-p.ctx.Done():
			fmt.Println("context is done")
			return
		default:
		}

		item, err := p.list.GetNextWithContext(p.ctx)
		if err != nil {
			ulog.Errorf("could not getNext (%s)", err)
			continue
		}

		p.lock.Lock()
		p.sendInProgress = true
		p.lock.Unlock()

		retries := 0
		backoff := p.opts.backoffInitial

		var success bool
		startTime := time.Now()
		var duration time.Duration

		for {
			err = p.send(item)
			if err != nil {
				time.Sleep(backoff)

				newBackoff := backoff * time.Duration(p.opts.backoffFactor)
				if newBackoff <= p.opts.backoffLimit {
					backoff = newBackoff
				}

				retries++

				if retries == p.opts.maxRetries && p.opts.maxRetries != -1 {
					success = false
					break
				}

				continue
			}

			success = true
			break
		}
		duration = time.Since(startTime)

		p.lock.Lock()
		p.sendInProgress = false
		if success {
			p.successfulSends++
			if val, ok := p.successfulSendsByRetry[retries]; ok {
				p.successfulSendsByRetry[retries] = val + 1
			} else {
				p.successfulSendsByRetry[retries] = 1
			}
		} else {
			p.failedSends++
		}
		p.lock.Unlock()

		p.sendReports.Append(SendReport{
			Time:     time.Now(),
			Item:     item,
			Duration: duration,
			Retries:  retries,
			Success:  success,
		})
		p.sendReports.DeleteWithFilter(func(item interface{}) bool {
			return time.Since(item.(SendReport).Time) > p.opts.keepSendReportsFor
		})
	}
}
