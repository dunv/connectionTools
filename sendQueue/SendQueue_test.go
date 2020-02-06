package sendQueue

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

var callMock *CallMock
var successOnSecondTry = 1
var successOnThirdTry = 1
var successOnTenthTry = 1

func setup() {
	rand.Seed(time.Now().UnixNano())
	callMock = new(CallMock)
	successOnSecondTry = 1
	successOnThirdTry = 1
	successOnTenthTry = 1
}

type CallMock struct {
	mock.Mock
}

func (c *CallMock) actualSend(item interface{}) error {
	switch item.(string) {
	case "successOnFirstTry":
		return nil
	case "successOnSecondTry":
		if successOnSecondTry < 2 {
			successOnSecondTry++
			return errors.New("send timeout")
		}
		return nil
	case "successOnThirdTry":
		if successOnThirdTry < 3 {
			successOnThirdTry++
			return errors.New("send timeout")
		}
		return nil
	case "successOnTenthTry":
		if successOnTenthTry < 10 {
			successOnTenthTry++
			return errors.New("send timeout")
		}
		return nil
	default:
		fmt.Println("received unknown instruction")
		return nil
	}
}

func (c *CallMock) Send(item interface{}) error {
	c.Called(item)
	res := c.actualSend(item)
	return res

}

func TestSendQueue_Default(t *testing.T) {
	setup()
	callMock.On("Send", "successOnFirstTry").Times(1)
	callMock.On("Send", "successOnSecondTry").Times(2)
	callMock.On("Send", "successOnThirdTry").Times(3)
	callMock.On("Send", "successOnTenthTry").Times(10)

	sendQueue := NewSendQueue(callMock.Send, context.Background())
	sendQueue.Push("successOnThirdTry")
	sendQueue.Push("successOnFirstTry")
	sendQueue.Push("successOnSecondTry")
	sendQueue.Push("successOnTenthTry")

	for sendQueue.Status().QueueLength > 0 || sendQueue.Status().SendInProgress {
		time.Sleep(100 * time.Millisecond)
	}
	status := sendQueue.Status()
	fmt.Println(status.Pretty())

	callMock.AssertExpectations(t)

	require.Equal(t, 4, status.TotalSuccessfulSends)
	require.Equal(t, 0, status.TotalFailedSends)
	require.Equal(t, 0, status.QueueLength)
}

func TestSendQueue_MaxRetry(t *testing.T) {
	setup()
	callMock.On("Send", "successOnFirstTry").Times(1)
	callMock.On("Send", "successOnSecondTry").Times(2)
	callMock.On("Send", "successOnThirdTry").Times(2)
	callMock.On("Send", "successOnTenthTry").Times(2)

	sendQueue := NewSendQueue(callMock.Send, context.Background(), WithMaxRetries(2))
	sendQueue.Push("successOnThirdTry")
	sendQueue.Push("successOnFirstTry")
	sendQueue.Push("successOnSecondTry")
	sendQueue.Push("successOnTenthTry")

	for sendQueue.Status().QueueLength > 0 || sendQueue.Status().SendInProgress {
		time.Sleep(100 * time.Millisecond)
	}
	status := sendQueue.Status()
	fmt.Println(status.Pretty())

	callMock.AssertExpectations(t)
	require.Equal(t, 2, status.TotalSuccessfulSends)
	require.Equal(t, 2, status.TotalFailedSends)
	require.Equal(t, 0, status.QueueLength)
}
