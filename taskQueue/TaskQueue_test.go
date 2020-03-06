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
	rand.Seed(time.Now().UnixNano())
	successOnSecondTry = 1
	successOnThirdTry = 1
	successOnTenthTry = 1
	return WithContext(context.WithValue(context.Background(), "testing", t))
}

func TestTaskQueue_Default(t *testing.T) {
	opt := setup(t)
	sendQueue := NewTaskQueue(context.Background(), opt)
	sendQueue.Push(successOnFirstTryFn)
	sendQueue.Push(successOnSecondTryFn)
	sendQueue.Push(successOnThirdTryFn)
	sendQueue.Push(successOnTenthTryFn)

	// Wait until finished
	for sendQueue.Status().QueueLength > 0 || sendQueue.Status().InProgress {
		time.Sleep(100 * time.Millisecond)
	}
	status := sendQueue.Status()
	fmt.Println(status.Pretty())

	require.Equal(t, 4, status.TotalSuccessful)
	require.Equal(t, 0, status.TotalFailed)
	require.Equal(t, 0, status.QueueLength)
}

func TestTaskQueue_MaxRetry_ByTask(t *testing.T) {
	opt := setup(t)
	sendQueue := NewTaskQueue(context.Background(), opt)
	sendQueue.Push(successOnFirstTryFn, WithMaxRetries(2))
	sendQueue.Push(successOnSecondTryFn, WithMaxRetries(2))
	sendQueue.Push(successOnThirdTryFn, WithMaxRetries(2))
	sendQueue.Push(successOnTenthTryFn, WithMaxRetries(2))

	// Wait until finished
	for sendQueue.Status().QueueLength > 0 || sendQueue.Status().InProgress {
		time.Sleep(100 * time.Millisecond)
	}
	status := sendQueue.Status()
	fmt.Println(status.Pretty())

	require.Equal(t, 2, status.TotalSuccessful)
	require.Equal(t, 2, status.TotalFailed)
	require.Equal(t, 0, status.QueueLength)
}

func TestTaskQueue_MaxRetry_ByDefault(t *testing.T) {
	opt := setup(t)
	sendQueue := NewTaskQueue(context.Background(), opt, WithMaxRetries(2))
	sendQueue.Push(successOnFirstTryFn)
	sendQueue.Push(successOnSecondTryFn)
	sendQueue.Push(successOnThirdTryFn)
	sendQueue.Push(successOnTenthTryFn)

	// Wait until finished
	for sendQueue.Status().QueueLength > 0 || sendQueue.Status().InProgress {
		time.Sleep(100 * time.Millisecond)
	}
	status := sendQueue.Status()
	fmt.Println(status.Pretty())

	require.Equal(t, 2, status.TotalSuccessful)
	require.Equal(t, 2, status.TotalFailed)
	require.Equal(t, 0, status.QueueLength)
}

func TestTaskQueue_Priority(t *testing.T) {
	opt := setup(t)

	// setup WithStartManually (this way we can schedule everything first and then start)
	queue := NewTaskQueue(context.Background(), opt, withStartManually())

	firstTask := queue.Push(successOnFirstTryFn, WithPriority(100))
	secondTask := queue.Push(successOnSecondTryFn, WithPriority(1000))
	thirdTask := queue.Push(successOnThirdTryFn, WithPriority(2000))

	// run manually
	go queue.run()

	// Wait until finished
	for queue.Status().QueueLength > 0 || queue.Status().InProgress {
		time.Sleep(100 * time.Millisecond)
	}

	status := queue.Status()
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
	require.Equal(t, 0, status.QueueLength)
}

// Fn that runs into timeout
func TestTaskQueue_Timeout_Error(t *testing.T) {
	opt := setup(t)
	sendQueue := NewTaskQueue(context.Background(), opt)

	// Function which completes successfully after one second, if it is interrupted by context -> will return error
	// Queued with a timeout of 100ms -> should error out
	sendQueue.Push(func(ctx context.Context) error {
		_, ok := ctx.Deadline()
		if !ok {
			return errors.New("there is no deadline but there should be")
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second):
			return nil
		}
	}, WithTimeout(100*time.Millisecond), WithMaxRetries(1))

	// Wait until finished
	for sendQueue.Status().QueueLength > 0 || sendQueue.Status().InProgress {
		time.Sleep(100 * time.Millisecond)
	}

	status := sendQueue.Status()
	require.Equal(t, 0, status.TotalSuccessful)
	require.Equal(t, 1, status.TotalFailed)
	require.Equal(t, 0, status.QueueLength)
}

// Fn that does not run into timeout, but a timeout is configured
func TestTaskQueue_Timeout_Success(t *testing.T) {
	opt := setup(t)
	sendQueue := NewTaskQueue(context.Background(), opt)

	// Function which completes successfully after 50ms, if it is interrupted by context -> will return error
	// Queued with a timeout of 100ms -> should return successfully
	sendQueue.Push(func(ctx context.Context) error {
		_, ok := ctx.Deadline()
		if !ok {
			return errors.New("there is no deadline but there should be")
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(50 * time.Millisecond):
			return nil
		}
	}, WithTimeout(100*time.Millisecond), WithMaxRetries(1))

	// Wait until finished
	for sendQueue.Status().QueueLength > 0 || sendQueue.Status().InProgress {
		time.Sleep(100 * time.Millisecond)
	}

	status := sendQueue.Status()
	require.Equal(t, 1, status.TotalSuccessful)
	require.Equal(t, 0, status.TotalFailed)
	require.Equal(t, 0, status.QueueLength)
}

// Fn that does not run into timeout (making sure no timeout is passed into it)
func TestTaskQueue_Timeout_NoTimeout(t *testing.T) {
	opt := setup(t)
	sendQueue := NewTaskQueue(context.Background(), opt)

	sendQueue.Push(func(ctx context.Context) error {
		_, ok := ctx.Deadline()
		if ok {
			return errors.New("there is a deadline but there should not be")
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(500 * time.Millisecond):
			return nil
		}
	}, WithMaxRetries(1))

	// Wait until finished
	for sendQueue.Status().QueueLength > 0 || sendQueue.Status().InProgress {
		time.Sleep(100 * time.Millisecond)
	}

	status := sendQueue.Status()
	require.Equal(t, 1, status.TotalSuccessful)
	require.Equal(t, 0, status.TotalFailed)
	require.Equal(t, 0, status.QueueLength)
}
