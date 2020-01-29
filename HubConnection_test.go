package connectionTools

import (
	"testing"
	"time"

	"github.com/dunv/uhelpers"
	"github.com/stretchr/testify/assert"
)

func TestNotificationHubConnection1(t *testing.T) {
	hub := NewNotificationHub()
	expectedMessages := []string{"test1", "test2", "test3", "test4"}
	domain := "testDomain"
	done := make(chan bool)

	for i := 0; i < 100; i++ {
		startReceiving(t, hub, domain, expectedMessages, done)
	}

	for _, v := range expectedMessages {
		sends, errs := hub.Notify(domain, v)
		assert.Equal(t, 100, sends)
		assert.Equal(t, 0, len(errs))
	}

	for i := 0; i < len(expectedMessages); i++ {
		select {
		case <-time.After(500 * time.Millisecond):
			t.Error("timeout")
		case <-done:
		}
	}
}

func TestNotificationHubConnection2(t *testing.T) {
	hub := NewNotificationHub()
	expectedMessages1 := []string{"test11", "test12", "test13", "test14"}
	expectedMessages2 := []string{"test21", "test22", "test23", "test24"}
	domain1 := "testDomain1"
	domain2 := "testDomain2"
	done := make(chan bool)

	for i := 0; i < 100; i++ {
		startReceiving(t, hub, domain1, expectedMessages1, done)
	}

	for i := 0; i < 100; i++ {
		startReceiving(t, hub, domain2, expectedMessages2, done)
	}

	for _, v := range expectedMessages1 {
		sends, errs := hub.Notify(domain1, v)
		assert.Equal(t, 100, sends)
		assert.Equal(t, 0, len(errs))
	}

	for _, v := range expectedMessages2 {
		sends, errs := hub.Notify(domain2, v)
		assert.Equal(t, 100, sends)
		assert.Equal(t, 0, len(errs))
	}

	for i := 0; i < len(expectedMessages1)+len(expectedMessages2); i++ {
		select {
		case <-time.After(500 * time.Millisecond):
			t.Error("timeout")
		case <-done:
		}
	}
}

func TestNotificationHubConnectionNoTimeout(t *testing.T) {
	hub := NewNotificationHub()
	hub.Register("test", make(chan interface{}))

	done := make(chan bool)
	go func() {
		hub.Notify("test", "test")
		done <- true
	}()

	select {
	case <-time.After(200 * time.Millisecond):
	case <-done:
		t.Error("notify did not block forever")
	}

}

// func TestNotificationHubConnectionTimeout(t *testing.T) {
// 	hub := NewNotificationHub(NotificationHubOptions{SendTimeout: uhelpers.PtrToDuration(100 * time.Millisecond)})
// 	hub.Register("test", make(chan interface{}))

// 	done := make(chan bool)
// 	go func() {
// 		hub.Notify("test", "test")
// 		done <- true
// 	}()

// 	select {
// 	case <-time.After(200 * time.Millisecond):
// 		t.Error("notify did not run into timeout, but should have")
// 	case <-done:
// 	}
// }

func TestNotificationHubConnectionWithBuffering1(t *testing.T) {
	hub := NewNotificationHub(NotificationHubOptions{SendBuffer: uhelpers.PtrToInt(1)})

	hub.Register("test", make(chan interface{}))
	done := make(chan bool)
	go func() {
		hub.Notify("test", "test")
		done <- true
	}()

	select {
	case <-time.After(200 * time.Millisecond):
		t.Error("did block, but should not have")
	case <-done:
	}

}

func startReceiving(t *testing.T, hub *NotificationHub, domain string, expectedMessages []string, done chan bool) {
	receiver := make(chan interface{})
	hub.Register(domain, receiver)

	go func() {
		i := 0
		for msg := range receiver {
			assert.Equal(t, expectedMessages[i], msg)
			i++
			if len(expectedMessages) == i {
				done <- true
			}
		}
	}()
}
