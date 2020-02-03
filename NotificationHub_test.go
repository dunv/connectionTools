package connectionTools

import (
	"testing"
	"time"

	"github.com/dunv/uhelpers"
	"github.com/stretchr/testify/assert"
)

func TestNotificationHubConnection_OneDomain(t *testing.T) {
	hub := NewNotificationHub()
	expectedMessages := []string{"test1", "test2", "test3", "test4"}
	domain := "testDomain"
	done := make(chan bool)

	for i := 0; i < 100; i++ {
		startReceiving(t, hub, domain, expectedMessages, done)
	}

	for _, v := range expectedMessages {
		sends, err := hub.Notify(domain, v)
		assert.Equal(t, 100, sends)
		assert.NoError(t, err)
	}

	for i := 0; i < len(expectedMessages); i++ {
		select {
		case <-time.After(500 * time.Millisecond):
			t.Error("timeout")
		case <-done:
		}
	}
}

func TestNotificationHubConnection_TwoDomains(t *testing.T) {
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
		sends, err := hub.Notify(domain1, v)
		assert.Equal(t, 1000, sends)
		assert.NoError(t, err, 0)
	}

	for _, v := range expectedMessages2 {
		sends, err := hub.Notify(domain2, v)
		assert.Equal(t, 1000, sends)
		assert.NoError(t, err, 0)
	}

	for i := 0; i < len(expectedMessages1)+len(expectedMessages2); i++ {
		select {
		case <-time.After(500 * time.Millisecond):
			t.Error("timeout")
		case <-done:
		}
	}
}

func TestNotificationHubConnection_WithBuffer(t *testing.T) {
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
		sends, err := hub.Notify(domain1, v)
		assert.Equal(t, 1000, sends)
		assert.NoError(t, err, 0)
	}

	for _, v := range expectedMessages2 {
		sends, err := hub.Notify(domain2, v)
		assert.Equal(t, 1000, sends)
		assert.NoError(t, err, 0)
	}

	for i := 0; i < len(expectedMessages1)+len(expectedMessages2); i++ {
		select {
		case <-time.After(500 * time.Millisecond):
			t.Error("timeout")
		case <-done:
		}
	}
}

func TestNotificationHubConnection_WithUnregister(t *testing.T) {
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
			sends, err := hub.Notify(domain, v)
			assert.Equal(t, 1, sends)
			assert.NoError(t, err)
		}
	}()

	for i := 0; i < len(expectedMessages); i++ {
		select {
		case <-time.After(500 * time.Millisecond):
			t.Error("timeout")
		case <-done:
		}
	}

	// Use blocking -> we know what we are doing
	hub.UnregisterBlocking(connGUID, nil)

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
			sends, err := hub.Notify(domain, v)
			assert.Equal(t, 0, sends)
			assert.NoError(t, err)
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

func TestNotificationHubConnection_NoTimeout(t *testing.T) {
	hub := NewNotificationHub()
	hub.Register("test", make(chan interface{}))

	done := make(chan bool)
	go func() {
		sends, err := hub.Notify("test", "test")
		assert.Equal(t, 0, sends, "should not have sent to anyone")
		assert.Error(t, err)
		assert.Len(t, err.(ErrNotAllReachable).ErrMap, 1, "should not have sent to anyone")
		done <- true
	}()

	select {
	case <-time.After(200 * time.Millisecond):
	case <-done:
		t.Error("notify did not block forever")
	}

}

func TestNotificationHubConnection_WithTimeout1(t *testing.T) {
	hub := NewNotificationHub(NotificationHubOptions{SendTimeout: uhelpers.PtrToDuration(100 * time.Millisecond)})
	hub.Register("test", make(chan interface{}))

	done := make(chan bool)
	go func() {
		sends, err := hub.Notify("test", "test")
		assert.Equal(t, 0, sends, "should not have sent to anyone (timeout was too short)")
		assert.Error(t, err)
		assert.Len(t, err.(ErrNotAllReachable).ErrMap, 1, "should not have sent to anyone (timeout was too short)")
		done <- true
	}()

	select {
	case <-time.After(200 * time.Millisecond):
		t.Error("notify did not run into timeout, but should have")
	case <-done:
	}
}

func TestNotificationHubConnection_WithTimeout2(t *testing.T) {
	hub := NewNotificationHub(NotificationHubOptions{SendTimeout: uhelpers.PtrToDuration(100 * time.Millisecond)})
	expectedMessages := []string{"test1", "test2", "test3", "test4"}
	domain := "testDomain"
	done := make(chan bool)

	for i := 0; i < 100; i++ {
		startReceiving(t, hub, domain, expectedMessages, done)
	}

	for _, v := range expectedMessages {
		sends, err := hub.Notify(domain, v)
		assert.Equal(t, 100, sends)
		assert.NoError(t, err)
	}

	for i := 0; i < len(expectedMessages); i++ {
		select {
		case <-time.After(500 * time.Millisecond):
			t.Error("timeout")
		case <-done:
		}
	}
}

func TestNotificationHubConnection_WithBuffering1(t *testing.T) {
	hub := NewNotificationHub(NotificationHubOptions{SendBuffer: uhelpers.PtrToInt(1)})

	hub.Register("test", make(chan interface{}))
	done := make(chan bool)
	go func() {
		sends, err := hub.Notify("test", "test")
		assert.Equal(t, 1, sends, "should have sent 1 (in the buffer)")
		assert.NoError(t, err, "should have sent successfully (with buffering)")
		done <- true
	}()

	select {
	case <-time.After(200 * time.Millisecond):
		t.Error("did block, but should not have (buffer was supposed to be 1 item)")
	case <-done:
	}

}

func TestNotificationHubConnection_WithBuffering2(t *testing.T) {
	hub := NewNotificationHub(NotificationHubOptions{SendBuffer: uhelpers.PtrToInt(1)})

	hub.Register("test", make(chan interface{}))
	done := make(chan bool)
	go func() {
		sends, err := hub.Notify("test", "test")
		assert.Equal(t, 1, sends, "should have sent 1 (in the buffer)")
		assert.NoError(t, err, "should have sent successfully (with buffering)")
		sends, err = hub.Notify("test", "test")
		assert.Equal(t, 0, sends, "should have sent 0 (buffer full)")
		assert.Error(t, err)
		t.Error("hub.Notify should have blocked, buffer is 1 and no one is consuming")
		done <- true
	}()

	select {
	case <-time.After(200 * time.Millisecond):
	case <-done:
		t.Error("reached done but should have blocked (buffer was supposed to be 1 item but we sent 2)")
	}
}

func TestNotificationHubConnection_WithBuffering3(t *testing.T) {
	hub := NewNotificationHub(NotificationHubOptions{SendBuffer: uhelpers.PtrToInt(2)})

	hub.Register("test", make(chan interface{}))
	done := make(chan bool)
	go func() {
		sends, err := hub.Notify("test", "test")
		assert.Equal(t, 1, sends, "should have sent 1 (in the buffer)")
		assert.NoError(t, err, "should have sent successfully (with buffering)")
		sends, err = hub.Notify("test", "test")
		assert.Equal(t, 1, sends, "should have sent 1 (in the buffer)")
		assert.NoError(t, err, "should have sent successfully (with buffering)")
		done <- true
	}()

	select {
	case <-time.After(200 * time.Millisecond):
		t.Error("blocked, but should have buffered (buffer was supposed to be 2 and we sent 2)")
	case <-done:
	}

}

func TestNotificationHubConnection_WithBuffering4(t *testing.T) {
	hub := NewNotificationHub(NotificationHubOptions{SendBuffer: uhelpers.PtrToInt(4)})

	hub.Register("test", make(chan interface{}))
	done := make(chan bool)
	go func() {
		sends, err := hub.Notify("test", "test")
		assert.Equal(t, 1, sends, "should have sent 1 (in the buffer)")
		assert.NoError(t, err, "should have sent successfully (with buffering)")
		sends, err = hub.Notify("test", "test")
		assert.Equal(t, 1, sends, "should have sent 1 (in the buffer)")
		assert.NoError(t, err, "should have sent successfully (with buffering)")
		sends, err = hub.Notify("test", "test")
		assert.Equal(t, 1, sends, "should have sent 1 (in the buffer)")
		assert.NoError(t, err, "should have sent successfully (with buffering)")
		sends, err = hub.Notify("test", "test")
		assert.Equal(t, 1, sends, "should have sent 1 (in the buffer)")
		assert.NoError(t, err, "should have sent successfully (with buffering)")
		done <- true
	}()

	select {
	case <-time.After(200 * time.Millisecond):
		t.Error("blocked, but should have buffered (buffer was supposed to be 2 and we sent 2)")
	case <-done:
	}

}

func TestNotificationHub_Metrics(t *testing.T) {
	hub := NewNotificationHub()

	dummyChannel1 := make(chan interface{})
	guid1 := hub.Register("all", dummyChannel1)

	dummyChannel2 := make(chan interface{})
	guid2 := hub.Register("all", dummyChannel2)

	conns := hub.Status().Connections
	assert.Len(t, conns, 2)

	found1 := false
	found2 := false
	for _, conn := range conns {
		if conn.ConnectionGUID == guid1 {
			found1 = true
		}
		if conn.ConnectionGUID == guid2 {
			found2 = true
		}
	}

	assert.True(t, found1)
	assert.True(t, found2)
}

func TestNotificationHubRegistry_PreservingOrder(t *testing.T) {
	hub := NewNotificationHub()
	dummyChannel := make(chan interface{})

	guid1 := hub.Register("all", dummyChannel)
	guid2 := hub.Register("all", dummyChannel)

	reg := hub.Status().Registry
	assert.Len(t, reg, 1)

	for domain, regItems := range reg {
		assert.Equal(t, "all", domain)
		assert.Len(t, regItems, 2)
		for i, regItem := range regItems {
			if i == 0 {
				assert.Equal(t, guid1, regItem)
			} else if i == 1 {
				assert.Equal(t, guid2, regItem)
			}
		}
	}

	hub.UnregisterBlocking(guid1, nil)

	reg = hub.Status().Registry
	assert.Len(t, reg, 1)

	for domain, regItems := range reg {
		assert.Equal(t, "all", domain)
		assert.Len(t, regItems, 1)
		for i, regItem := range regItems {
			if i == 0 {
				assert.Equal(t, guid2, regItem)
			}
		}
	}

	guid1 = hub.Register("all", dummyChannel)

	reg = hub.Status().Registry
	assert.Len(t, reg, 1)

	for domain, regItems := range reg {
		assert.Equal(t, "all", domain)
		assert.Len(t, regItems, 2)
		for i, regItem := range regItems {
			if i == 0 {
				assert.Equal(t, guid2, regItem)
			} else if i == 1 {
				assert.Equal(t, guid1, regItem)
			}
		}
	}
}

func TestNotificationHubRegistry_MultipleDomains(t *testing.T) {
	hub := NewNotificationHub()
	dummyChannel := make(chan interface{})

	guid1 := hub.Register("domain1", dummyChannel)
	guid2 := hub.Register("domain2", dummyChannel)

	reg := hub.Status().Registry
	assert.Len(t, reg, 2)

	for domain, regItems := range reg {
		if domain == "domain1" {
			assert.Len(t, regItems, 1)
			for _, regItem := range regItems {
				assert.Equal(t, guid1, regItem)
			}
		} else if domain == "domain2" {
			assert.Len(t, regItems, 1)
			for _, regItem := range regItems {
				assert.Equal(t, guid2, regItem)
			}
		}
	}

	hub.UnregisterBlocking(guid1, nil)
	reg = hub.Status().Registry
	assert.Len(t, reg, 1)

	for domain, regItems := range reg {
		assert.Equal(t, "domain2", domain)
		assert.Len(t, regItems, 1)
		for _, regItem := range regItems {
			assert.Equal(t, guid2, regItem)
		}
	}

}

func startReceiving(t *testing.T, hub *NotificationHub, domain string, expectedMessages []string, done chan bool) {
	receiver := make(chan interface{})
	guid := hub.Register(domain, receiver)

	go func() {
		i := 0
		for msg := range receiver {
			assert.Equal(t, expectedMessages[i], msg)
			i++
			if len(expectedMessages) == i {
				hub.UnregisterBlocking(guid, nil)
				done <- true
			}
		}
	}()
}
