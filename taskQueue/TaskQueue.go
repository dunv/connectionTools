package taskQueue

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/dunv/concurrentList"
	"github.com/dunv/ulog"
)

type TaskQueueFunc func(context context.Context) error

type TaskReport struct {
	Time     time.Time
	Item     interface{}
	Duration time.Duration
	Retries  int
	Success  bool
}

type TaskQueue struct {
	list *concurrentList.ConcurrentList
	ctx  context.Context

	defaultOpts taskQueueOptions

	// statistics
	sendInProgress    bool
	successful        int
	successfulByRetry map[int]int
	failed            int
	reports           *concurrentList.ConcurrentList

	lock sync.Mutex
}

type Task struct {
	opts taskQueueOptions
	fn   TaskQueueFunc
}

func NewTaskQueue(ctx context.Context, opts ...TaskQueueOption) *TaskQueue {
	// Default options
	mergedOpts := taskQueueOptions{
		keepTaskReportsFor: 24 * time.Hour,
		maxRetries:         -1,
		backoffInitial:     0,
		backoffFactor:      1,
		backoffLimit:       1,
		ctx:                context.Background(),
	}
	for _, opt := range opts {
		opt.apply(&mergedOpts)
	}

	queue := &TaskQueue{
		list:              concurrentList.NewConcurrentList(),
		ctx:               ctx,
		successfulByRetry: make(map[int]int),
		reports:           concurrentList.NewConcurrentList(),
		defaultOpts:       mergedOpts,
	}
	go queue.run()
	return queue
}

func (p *TaskQueue) Status() TaskQueueStatus {
	p.lock.Lock()
	defer p.lock.Unlock()

	return statusFromTaskQueue(p)
}

func (p *TaskQueue) Length() int {
	return p.list.Length()
}

func (p *TaskQueue) Push(fn TaskQueueFunc, opts ...TaskQueueOption) {
	// single tasks can have their own options
	mergedOpts := p.defaultOpts
	for _, opt := range opts {
		opt.apply(&mergedOpts)
	}

	p.list.Append(Task{
		fn:   fn,
		opts: mergedOpts,
	})
}

func (p *TaskQueue) run() {
	for {
		// check if context is done yet
		select {
		case <-p.ctx.Done():
			fmt.Println("context is done")
			return
		default:
		}

		taskRaw, err := p.list.GetNextWithContext(p.ctx)
		if err != nil {
			ulog.Errorf("could not getNext (%s)", err)
			continue
		}

		task := taskRaw.(Task)

		p.lock.Lock()
		p.sendInProgress = true
		p.lock.Unlock()

		retries := 0
		backoff := task.opts.backoffInitial

		var success bool
		startTime := time.Now()
		var duration time.Duration

		for {
			err = task.fn(withContext(task.opts, retries, backoff))
			if err != nil {
				time.Sleep(backoff)

				newBackoff := backoff * time.Duration(task.opts.backoffFactor)
				if newBackoff <= task.opts.backoffLimit {
					backoff = newBackoff
				}

				retries++

				if retries == task.opts.maxRetries && task.opts.maxRetries != -1 {
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
			p.successful++
			if val, ok := p.successfulByRetry[retries]; ok {
				p.successfulByRetry[retries] = val + 1
			} else {
				p.successfulByRetry[retries] = 1
			}
		} else {
			p.failed++
		}
		p.lock.Unlock()

		p.reports.Append(TaskReport{
			Time:     time.Now(),
			Item:     task,
			Duration: duration,
			Retries:  retries,
			Success:  success,
		})
		p.reports.DeleteWithFilter(func(item interface{}) bool {
			return time.Since(item.(TaskReport).Time) > p.defaultOpts.keepTaskReportsFor
		})
	}
}
