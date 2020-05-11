package taskQueue

import (
	"context"
	"sync"
	"time"

	"github.com/dunv/concurrentList"
	"github.com/google/uuid"
)

type TaskQueueFunc func(context context.Context) error

type TaskReport struct {
	TaskGUID uuid.UUID
	Time     time.Time
	Item     interface{}
	Duration time.Duration
	Retries  int
	Success  bool
}

type TaskQueue struct {
	list        *concurrentList.ConcurrentList
	ctx         context.Context
	defaultOpts taskQueueOptions

	// sync for queue
	// this is used to make sure that queueLength and inProgress can only
	// be set and accessed when they are in sync
	inProgress bool
	length     int
	queueLock  sync.Mutex

	// sync for status
	statusLock sync.Mutex

	// statistics
	successful        int
	successfulByRetry map[int]int
	failed            int
	reports           *concurrentList.ConcurrentList
}

type task struct {
	taskGUID uuid.UUID
	opts     taskQueueOptions
	fn       TaskQueueFunc
}

func NewTaskQueue(ctx context.Context, opts ...TaskQueueOption) *TaskQueue {
	// Default options
	mergedOpts := taskQueueOptions{
		keepTaskReportsFor: 24 * time.Hour,
		maxRetries:         -1,
		backoffInitial:     0,
		backoffFactor:      1,
		backoffLimit:       1,
		priority:           1000,
		ctx:                context.Background(),
	}
	for _, opt := range opts {
		opt.apply(&mergedOpts)
	}

	// Passing a sortFn to concurrentList makes it a "priorityQueue"
	sortFn := func(i, j interface{}) bool {
		return i.(task).opts.priority > j.(task).opts.priority
	}

	queue := &TaskQueue{
		list:              concurrentList.NewConcurrentList(concurrentList.WithSorting(sortFn)),
		ctx:               ctx,
		successfulByRetry: make(map[int]int),
		reports:           concurrentList.NewConcurrentList(),
		defaultOpts:       mergedOpts,
	}

	if !mergedOpts.startManually {
		go queue.run()
	}

	return queue
}

func (p *TaskQueue) DetailedStatus() TaskQueueStatus {
	p.statusLock.Lock()
	status := statusFromTaskQueue(p)
	p.statusLock.Unlock()

	return status
}

func (p *TaskQueue) Status() (int, bool) {
	p.queueLock.Lock()
	length := p.length
	inProgress := p.inProgress
	p.queueLock.Unlock()

	return length, inProgress
}

func (p *TaskQueue) Push(fn TaskQueueFunc, opts ...TaskQueueOption) uuid.UUID {
	// single tasks can have their own options
	mergedOpts := p.defaultOpts
	for _, opt := range opts {
		opt.apply(&mergedOpts)
	}

	GUID := uuid.New()

	p.queueLock.Lock()
	p.list.Push(task{
		taskGUID: GUID,
		fn:       fn,
		opts:     mergedOpts,
	})
	p.length = p.list.Length()
	p.queueLock.Unlock()

	return GUID
}

func (p *TaskQueue) run() {
	for {
		// check if context is done yet
		select {
		case <-p.ctx.Done():
			return
		default:
		}

		taskRaw, err := p.list.GetNext(p.ctx)
		if err != nil {
			// this happens when the context runs out -> end this routine (will run into ctx.Done above)
			continue
		}
		task := taskRaw.(task)

		p.queueLock.Lock()
		p.length = p.list.Length()
		p.inProgress = true
		p.queueLock.Unlock()

		retries := 0
		backoff := task.opts.backoffInitial

		var success bool
		startTime := time.Now()
		var duration time.Duration

		for {
			// Setup context for fn-execution
			var cancel context.CancelFunc
			ctx := withContext(task.opts, retries, backoff)
			if task.opts.timeout != nil {
				ctx, cancel = context.WithTimeout(ctx, *task.opts.timeout)
			}

			// Execute fn
			err = task.fn(ctx)

			// Cleanup context
			if cancel != nil {
				cancel()
			}

			// Calculate backoff and retries
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

		p.queueLock.Lock()
		p.inProgress = false
		p.queueLock.Unlock()

		p.statusLock.Lock()
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
		p.statusLock.Unlock()

		p.reports.Push(TaskReport{
			TaskGUID: task.taskGUID,
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
