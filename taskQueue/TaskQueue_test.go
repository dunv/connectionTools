package taskQueue

import (
	"context"
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
