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
		assert.Len(t, errs, 0)
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

	for i := 0; i < 1000; i++ {
		startReceiving(t, hub, domain1, expectedMessages1, done)
	}

	for i := 0; i < 1000; i++ {
		startReceiving(t, hub, domain2, expectedMessages2, done)
	}

	for _, v := range expectedMessages1 {
		sends, errs := hub.Notify(domain1, v)
		assert.Equal(t, 1000, sends)
		assert.Len(t, errs, 0)
	}

	for _, v := range expectedMessages2 {
		sends, errs := hub.Notify(domain2, v)
		assert.Equal(t, 1000, sends)
		assert.Len(t, errs, 0)
	}

	for i := 0; i < len(expectedMessages1)+len(expectedMessages2); i++ {
		select {
		case <-time.After(500 * time.Millisecond):
			t.Error("timeout")
		case <-done:
		}
	}
}

func TestNotificationHubConnection3(t *testing.T) {
	hub := NewNotificationHub(NotificationHubOptions{SendBuffer: uhelpers.PtrToInt(100)})
	expectedMessages1 := []string{"test11", "test12", "test13", "test14"}
	expectedMessages2 := []string{"test21", "test22", "test23", "test24"}
	domain1 := "testDomain1"
	domain2 := "testDomain2"
	done := make(chan bool)

	for i := 0; i < 1000; i++ {
		startReceiving(t, hub, domain1, expectedMessages1, done)
	}

	for i := 0; i < 1000; i++ {
		startReceiving(t, hub, domain2, expectedMessages2, done)
	}

	for _, v := range expectedMessages1 {
		sends, errs := hub.Notify(domain1, v)
		assert.Equal(t, 1000, sends)
		assert.Len(t, errs, 0)
	}

	for _, v := range expectedMessages2 {
		sends, errs := hub.Notify(domain2, v)
		assert.Equal(t, 1000, sends)
		assert.Len(t, errs, 0)
	}

	for i := 0; i < len(expectedMessages1)+len(expectedMessages2); i++ {
		select {
		case <-time.After(500 * time.Millisecond):
			t.Error("timeout")
		case <-done:
		}
	}
}

func TestNotificationHubConnectionWithUnregister(t *testing.T) {
	hub := NewNotificationHub()
	expectedMessages := []string{"test1", "test2", "test3", "test4"}
	domain := "testDomain"
	done := make(chan bool)

	channel := make(chan interface{})
	connGUID := hub.Register(domain, channel)

	go func() {
		for i := 0; i < len(expectedMessages); i++ {
			item := <-channel
			assert.Equal(t, expectedMessages[i], item)
			done <- true
		}
	}()

	go func() {
		for _, v := range expectedMessages {
			sends, errs := hub.Notify(domain, v)
			assert.Equal(t, 1, sends)
			assert.Len(t, errs, 0)
		}
	}()

	for i := 0; i < len(expectedMessages); i++ {
		select {
		case <-time.After(500 * time.Millisecond):
			t.Error("timeout")
		case <-done:
		}
	}

	hub.Unregister(connGUID, nil)

	go func() {
		for i := 0; i < len(expectedMessages); i++ {
			item := <-channel
			assert.Equal(t, expectedMessages[i], item)
			done <- true
		}
	}()

	sendSuccess := make(chan bool)
	go func() {
		for _, v := range expectedMessages {
			sends, errs := hub.Notify(domain, v)
			assert.Equal(t, 0, sends)
			assert.Len(t, errs, 0)
		}
		sendSuccess <- true
	}()

	select {
	case <-time.After(200 * time.Millisecond):
		t.Error("sendSuccess blocked, but should not have. sending to no registered connections should not block")
	case <-sendSuccess:
	}

	for i := 0; i < len(expectedMessages); i++ {
		select {
		case <-time.After(200 * time.Millisecond):
		case <-done:
			t.Error("a message was consumed after unregistering")
		}
	}
}

func TestNotificationHubConnectionNoTimeout(t *testing.T) {
	hub := NewNotificationHub()
	hub.Register("test", make(chan interface{}))

	done := make(chan bool)
	go func() {
		sends, errs := hub.Notify("test", "test")
		assert.Equal(t, 0, sends, "should not have sent to anyone")
		assert.Len(t, errs, 1, "should not have sent to anyone")
		done <- true
	}()

	select {
	case <-time.After(200 * time.Millisecond):
	case <-done:
		t.Error("notify did not block forever")
	}

}

func TestNotificationHubConnectionTimeout(t *testing.T) {
	hub := NewNotificationHub(NotificationHubOptions{SendTimeout: uhelpers.PtrToDuration(100 * time.Millisecond)})
	hub.Register("test", make(chan interface{}))

	done := make(chan bool)
	go func() {
		sends, errs := hub.Notify("test", "test")
		assert.Equal(t, 0, sends, "should not have sent to anyone (timeout was too short)")
		assert.Len(t, errs, 1, "should not have sent to anyone (timeout was too short)")
		done <- true
	}()

	select {
	case <-time.After(200 * time.Millisecond):
		t.Error("notify did not run into timeout, but should have")
	case <-done:
	}
}

func TestNotificationHubConnectionWithBuffering1(t *testing.T) {
	hub := NewNotificationHub(NotificationHubOptions{SendBuffer: uhelpers.PtrToInt(1)})

	hub.Register("test", make(chan interface{}))
	done := make(chan bool)
	go func() {
		sends, errs := hub.Notify("test", "test")
		assert.Equal(t, 1, sends, "should have sent 1 (in the buffer)")
		assert.Len(t, errs, 0, "should have sent successfully (with buffering)")
		done <- true
	}()

	select {
	case <-time.After(200 * time.Millisecond):
		t.Error("did block, but should not have (buffer was supposed to be 1 item)")
	case <-done:
	}

}

func TestNotificationHubConnectionWithBuffering2(t *testing.T) {
	hub := NewNotificationHub(NotificationHubOptions{SendBuffer: uhelpers.PtrToInt(1)})

	hub.Register("test", make(chan interface{}))
	done := make(chan bool)
	go func() {
		sends, errs := hub.Notify("test", "test")
		assert.Equal(t, 1, sends, "should have sent 1 (in the buffer)")
		assert.Len(t, errs, 0, "should have sent successfully (with buffering)")
		hub.Notify("test", "test")
		t.Error("hub.Notify should have blocked, buffer is 1 and no one is consuming")
		done <- true
	}()

	select {
	case <-time.After(200 * time.Millisecond):
	case <-done:
		t.Error("reached done but should have blocked (buffer was supposed to be 1 item but we sent 2)")
	}
}

func TestNotificationHubConnectionWithBuffering3(t *testing.T) {
	hub := NewNotificationHub(NotificationHubOptions{SendBuffer: uhelpers.PtrToInt(2)})

	hub.Register("test", make(chan interface{}))
	done := make(chan bool)
	go func() {
		sends, errs := hub.Notify("test", "test")
		assert.Equal(t, 1, sends, "should have sent 1 (in the buffer)")
		assert.Len(t, errs, 0, "should have sent successfully (with buffering)")
		sends, errs = hub.Notify("test", "test")
		assert.Equal(t, 1, sends, "should have sent 1 (in the buffer)")
		assert.Len(t, errs, 0, "should have sent successfully (with buffering)")
		done <- true
	}()

	select {
	case <-time.After(200 * time.Millisecond):
		t.Error("blocked, but should have buffered (buffer was supposed to be 2 and we sent 2)")
	case <-done:
	}

}

func TestNotificationHubConnectionWithBuffering4(t *testing.T) {
	hub := NewNotificationHub(NotificationHubOptions{SendBuffer: uhelpers.PtrToInt(4)})

	hub.Register("test", make(chan interface{}))
	done := make(chan bool)
	go func() {
		sends, errs := hub.Notify("test", "test")
		assert.Equal(t, 1, sends, "should have sent 1 (in the buffer)")
		assert.Len(t, errs, 0, "should have sent successfully (with buffering)")
		sends, errs = hub.Notify("test", "test")
		assert.Equal(t, 1, sends, "should have sent 1 (in the buffer)")
		assert.Len(t, errs, 0, "should have sent successfully (with buffering)")
		sends, errs = hub.Notify("test", "test")
		assert.Equal(t, 1, sends, "should have sent 1 (in the buffer)")
		assert.Len(t, errs, 0, "should have sent successfully (with buffering)")
		sends, errs = hub.Notify("test", "test")
		assert.Equal(t, 1, sends, "should have sent 1 (in the buffer)")
		assert.Len(t, errs, 0, "should have sent successfully (with buffering)")
		done <- true
	}()

	select {
	case <-time.After(200 * time.Millisecond):
		t.Error("blocked, but should have buffered (buffer was supposed to be 2 and we sent 2)")
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
