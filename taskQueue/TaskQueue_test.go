package taskQueue

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setup(t *testing.T) TaskQueueOption {
	taskQueueMockLock.Lock()
	defer taskQueueMockLock.Unlock()

	rand.Seed(time.Now().UnixNano())
	successOnSecondTry = 1
	successOnThirdTry = 1
	successOnTenthTry = 1
	return WithContext(context.WithValue(context.Background(), TaskContextKey("testing"), t))
}

func waitUntilFinished(queue *TaskQueue) {
	for {
		length, inProgress := queue.Status()
		if length > 0 || inProgress {
			time.Sleep(10 * time.Millisecond)
			continue
		}
		break
	}
}

func TestTaskQueue_Default(t *testing.T) {
	opt := setup(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	queue := NewTaskQueue(ctx, opt)
	queue.Push(successOnFirstTryFn)
	queue.Push(successOnSecondTryFn)
	queue.Push(successOnThirdTryFn)
	queue.Push(successOnTenthTryFn)
	waitUntilFinished(queue)

	status := queue.DetailedStatus()
	assert.Equal(t, 4, status.TotalSuccessful, "totalSuccessful")
	assert.Equal(t, 0, status.TotalFailed, "totalFailed")
}

func TestTaskQueue_MaxRetry_ByTask(t *testing.T) {
	opt := setup(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	queue := NewTaskQueue(ctx, opt)
	queue.Push(successOnFirstTryFn, WithMaxRetries(2))
	queue.Push(successOnSecondTryFn, WithMaxRetries(2))
	queue.Push(successOnThirdTryFn, WithMaxRetries(2))
	queue.Push(successOnTenthTryFn, WithMaxRetries(2))
	waitUntilFinished(queue)

	status := queue.DetailedStatus()
	require.Equal(t, 2, status.TotalSuccessful)
	require.Equal(t, 2, status.TotalFailed)
}

func TestTaskQueue_MaxRetry_ByDefault(t *testing.T) {
	opt := setup(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	queue := NewTaskQueue(ctx, opt, WithMaxRetries(2))
	queue.Push(successOnFirstTryFn)
	queue.Push(successOnSecondTryFn)
	queue.Push(successOnThirdTryFn)
	queue.Push(successOnTenthTryFn)
	waitUntilFinished(queue)

	status := queue.DetailedStatus()
	require.Equal(t, 2, status.TotalSuccessful)
	require.Equal(t, 2, status.TotalFailed)
}

func TestTaskQueue_Priority(t *testing.T) {
	opt := setup(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	queue := NewTaskQueue(ctx, opt, withStartManually())

	firstTask := queue.Push(successOnFirstTryFn, WithDefaultPriority(100))
	secondTask := queue.Push(successOnSecondTryFn, WithDefaultPriority(1000))
	thirdTask := queue.Push(successOnThirdTryFn, WithDefaultPriority(2000))

	// run manually
	go queue.run()
	waitUntilFinished(queue)

	status := queue.DetailedStatus()
	for i, report := range status.Reports {
		switch i {
		case 0:
			assert.Equal(t, thirdTask, report.TaskGUID, "thirdTask goes 1st")
		case 1:
			assert.Equal(t, secondTask, report.TaskGUID, "secondTask goes 2nd")
		case 2:
			assert.Equal(t, firstTask, report.TaskGUID, "firstTask goes 3rd")
		default:
			t.Error("this should never be reached")
		}
	}

	require.Equal(t, 3, status.TotalSuccessful)
	require.Equal(t, 0, status.TotalFailed)
}

// Fn that runs into timeout
func TestTaskQueue_Timeout_Error(t *testing.T) {
	opt := setup(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	queue := NewTaskQueue(ctx, opt)

	// Function which completes successfully after 20ms, if it is interrupted by context -> will return error
	// Queued with a timeout of 10ms -> should error out
	queue.Push(func(ctx context.Context) error {
		_, ok := ctx.Deadline()
		if !ok {
			return errors.New("there is no deadline but there should be")
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(40 * time.Millisecond):
			return nil
		}
	}, WithTimeout(10*time.Millisecond), WithMaxRetries(1))

	waitUntilFinished(queue)

	status := queue.DetailedStatus()
	if status.TotalFailed != 1 {
		fmt.Println(status.Pretty())
		t.Error(status)
	}
	require.Equal(t, 0, status.TotalSuccessful, "total successful")
	require.Equal(t, 1, status.TotalFailed, "total failed")
}

// Fn that does not run into timeout, but a timeout is configured
func TestTaskQueue_Timeout_Success(t *testing.T) {
	opt := setup(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	queue := NewTaskQueue(ctx, opt)

	// Function which completes successfully after 50ms, if it is interrupted by context -> will return error
	// Queued with a timeout of 100ms -> should return successfully
	queue.Push(func(ctx context.Context) error {
		_, ok := ctx.Deadline()
		if !ok {
			return errors.New("there is no deadline but there should be")
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(10 * time.Millisecond):
			return nil
		}
	}, WithTimeout(20*time.Millisecond), WithMaxRetries(1))

	waitUntilFinished(queue)

	status := queue.DetailedStatus()
	require.Equal(t, 1, status.TotalSuccessful, "total successful")
	require.Equal(t, 0, status.TotalFailed, "total failed")
}

// Fn that does not run into timeout (making sure no timeout is passed into it)
func TestTaskQueue_Timeout_NoTimeout(t *testing.T) {
	opt := setup(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	queue := NewTaskQueue(ctx, opt)

	queue.Push(func(ctx context.Context) error {
		_, ok := ctx.Deadline()
		if ok {
			return errors.New("there is a deadline but there should not be")
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Millisecond):
			return nil
		}
	}, WithMaxRetries(1))
	waitUntilFinished(queue)

	status := queue.DetailedStatus()
	require.Equal(t, 1, status.TotalSuccessful)
	require.Equal(t, 0, status.TotalFailed)
}
