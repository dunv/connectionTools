package taskQueue

import (
	"context"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func setup(t *testing.T) TaskQueueOption {
	rand.Seed(time.Now().UnixNano())
	successOnSecondTry = 1
	successOnThirdTry = 1
	successOnTenthTry = 1
	return WithContext(context.WithValue(context.Background(), "testing", t))
}

func TestSendQueue_Default(t *testing.T) {
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

func TestSendQueue_MaxRetry_ByTask(t *testing.T) {
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

func TestSendQueue_MaxRetry_ByDefault(t *testing.T) {
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
