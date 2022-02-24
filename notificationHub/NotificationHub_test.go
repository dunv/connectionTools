package notificationHub

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/dunv/ulog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNotificationHubConnection_OneDomain(t *testing.T) {
	hub := NewNotificationHub()
	expectedMessages := []string{"test1", "test2", "test3", "test4"}
	domain := "testDomain"
	done := make(chan bool)

	receiveRoutines := 100

	for i := 0; i < receiveRoutines; i++ {
		startReceiving(t, hub, domain, expectedMessages, done)
	}

	for _, v := range expectedMessages {
		sends, err := hub.Notify(domain, v)
		assert.Equal(t, receiveRoutines, sends)
		assert.NoError(t, err)
	}

	for i := 0; i < receiveRoutines; i++ {
		select {
		case <-time.After(500 * time.Millisecond):
			t.Error("timeout")
		case <-done:
		}
	}

	status, err := hub.Status(context.Background())
	assert.NoError(t, err)
	assert.Len(t, status.Connections, 0, "expected no connections to be left")
	assert.Len(t, status.Registry, 0)
}

func BenchmarkNotificationHubRegisterUnregister(b *testing.B) {
	hub := NewNotificationHub()
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		registrationGUID := hub.Register("testDomain", make(chan<- interface{}))
		err := hub.Unregister(registrationGUID, nil, context.Background())
		if err != nil {
			b.Error(err)
			b.FailNow()
		}
	}
	b.StopTimer()
}

func BenchmarkNotificationHubNotify10(b *testing.B) {
	runBenchmarkNotify(b, 10)
}

func BenchmarkNotificationHubNotify100(b *testing.B) {
	runBenchmarkNotify(b, 100)
}

func BenchmarkNotificationHubNotify1000(b *testing.B) {
	runBenchmarkNotify(b, 1000)
}

func BenchmarkNotificationHubBroadcast10(b *testing.B) {
	runBenchmarkBroadcast(b, 10)
}

func BenchmarkNotificationHubBroadcast100(b *testing.B) {
	runBenchmarkBroadcast(b, 100)
}

func BenchmarkNotificationHubBroadcast1000(b *testing.B) {
	runBenchmarkBroadcast(b, 1000)
}

func TestNotificationHubConnection_TwoDomains(t *testing.T) {
	hub := NewNotificationHub()
	expectedMessages1 := []string{"test11", "test12", "test13", "test14"}
	expectedMessages2 := []string{"test21", "test22", "test23", "test24"}
	domain1 := "testDomain1"
	domain2 := "testDomain2"
	done := make(chan bool)

	receiveRoutines1 := 1000
	receiveRoutines2 := 1000

	for i := 0; i < receiveRoutines1; i++ {
		startReceiving(t, hub, domain1, expectedMessages1, done)
	}

	for i := 0; i < receiveRoutines2; i++ {
		startReceiving(t, hub, domain2, expectedMessages2, done)
	}

	for _, v := range expectedMessages1 {
		sends, err := hub.Notify(domain1, v)
		assert.Equal(t, receiveRoutines1, sends)
		assert.NoError(t, err, 0)
	}

	for _, v := range expectedMessages2 {
		sends, err := hub.Notify(domain2, v)
		assert.Equal(t, receiveRoutines2, sends)
		assert.NoError(t, err, 0)
	}

	for i := 0; i < receiveRoutines1+receiveRoutines2; i++ {
		select {
		case <-time.After(100 * time.Millisecond):
			t.Error("timeout")
		case <-done:
		}
	}

	status, err := hub.Status(context.Background())
	assert.NoError(t, err)
	assert.Len(t, status.Connections, 0, "expected no connections to be left")
	assert.Len(t, status.Registry, 0)
}

func TestNotificationHubConnection_TwoDomainsBroadcast(t *testing.T) {
	hub := NewNotificationHub()
	expectedMessages := []string{"test11", "test12", "test13", "test14"}
	domain1 := "testDomain1"
	domain2 := "testDomain2"
	done := make(chan bool)

	receiveRoutines1 := 1000
	receiveRoutines2 := 1000

	for i := 0; i < receiveRoutines1; i++ {
		startReceiving(t, hub, domain1, expectedMessages, done)
	}

	for i := 0; i < receiveRoutines2; i++ {
		startReceiving(t, hub, domain2, expectedMessages, done)
	}

	for _, v := range expectedMessages {
		sends, err := hub.Broadcast(v)
		assert.Equal(t, receiveRoutines1+receiveRoutines2, sends)
		assert.NoError(t, err, 0)
	}

	for i := 0; i < receiveRoutines1+receiveRoutines2; i++ {
		select {
		case <-time.After(100 * time.Millisecond):
			t.Error("timeout")
		case <-done:
		}
	}

	status, err := hub.Status(context.Background())
	assert.NoError(t, err)
	assert.Len(t, status.Connections, 0, "expected no connections to be left")
	assert.Len(t, status.Registry, 0)
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
			require.Equal(t, expectedMessages[i], item)
			done <- true
		}
	}()

	go func() {
		for _, v := range expectedMessages {
			sends, err := hub.Notify(domain, v)
			require.Equal(t, 1, sends)
			require.NoError(t, err)
		}
	}()

	for i := 0; i < len(expectedMessages); i++ {
		select {
		case <-time.After(100 * time.Millisecond):
			require.FailNow(t, "timeout")
		case <-done:
		}
	}

	err := hub.Unregister(connGUID, nil, context.Background())
	require.NoError(t, err)
	if err != nil {
		return
	}

	go func() {
		for i := 0; i < len(expectedMessages); i++ {
			item := <-channel
			require.Equal(t, expectedMessages[i], item)
			done <- true
		}
	}()

	sendSuccess := make(chan bool)
	go func() {
		for _, v := range expectedMessages {
			sends, err := hub.Notify(domain, v)
			require.Equal(t, 0, sends)
			require.NoError(t, err)
		}
		sendSuccess <- true
	}()

	select {
	case <-time.After(time.Millisecond):
		require.FailNow(t, "sendSuccess blocked, but should not have. sending to no registered connections should not block")
	case <-sendSuccess:
	}

	select {
	case <-time.After(10 * time.Millisecond):
	case <-done:
		require.FailNow(t, "a message was consumed after unregistering")
	}

	status, err := hub.Status(context.Background())
	require.NoError(t, err)
	require.Len(t, status.Connections, 0, "expected no connections to be left")
	require.Len(t, status.Registry, 0)
}

func TestNotificationHubConnection_NoTimeoutConfigured(t *testing.T) {
	hub := NewNotificationHub()
	hub.Register("test", make(chan interface{}))

	done := make(chan bool)
	go func() {
		_, _ = hub.Notify("test", "test")
		t.Error("should never reach this")
		done <- true
	}()

	select {
	case <-time.After(time.Millisecond):
	case <-done:
		t.Error("notify did not block forever")
	}

	ctx, cancelFunc := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancelFunc()
	_, err := hub.Status(ctx)
	assert.EqualError(t, err, ErrHubLocked.Error())
}

func TestNotificationHubConnection_WithTimeout1(t *testing.T) {
	hub := NewNotificationHub(WithSendTimeout(10 * time.Millisecond))
	guid := hub.Register("test", make(chan interface{}))

	done := make(chan bool)
	go func() {
		sends, err := hub.Notify("test", "test")
		assert.Equal(t, 0, sends, "should not have sent to anyone (timeout was too short)")
		assert.Error(t, err)
		assert.Len(t, err.(ErrNotAllReachable).ErrMap, 1, "should not have sent to anyone (timeout was too short)")
		done <- true
	}()

	select {
	case <-time.After(100 * time.Millisecond):
		t.Error("notify did not run into timeout, but should have")
	case <-done:
	}

	err := hub.Unregister(guid, errors.New("my reason"), context.Background())
	assert.NoError(t, err)
	if err != nil {
		return
	}

	status, err := hub.Status(context.Background())
	assert.NoError(t, err)
	if err != nil {
		return
	}
	assert.Len(t, status.Connections, 0, "expected no connections to be left")
	assert.Len(t, status.Registry, 0)
}

func TestNotificationHubConnection_WithTimeout2(t *testing.T) {
	hub := NewNotificationHub(WithSendTimeout(100 * time.Millisecond))
	expectedMessages := []string{"test1", "test2", "test3", "test4"}
	domain := "testDomain"
	done := make(chan bool)
	receiveRoutines := 100

	for i := 0; i < receiveRoutines; i++ {
		startReceiving(t, hub, domain, expectedMessages, done)
	}

	for _, v := range expectedMessages {
		sends, err := hub.Notify(domain, v)
		assert.Equal(t, receiveRoutines, sends)
		assert.NoError(t, err)
	}

	for i := 0; i < receiveRoutines; i++ {
		select {
		case <-time.After(100 * time.Millisecond):
			t.Error("timeout")
		case <-done:
		}
	}

	status, err := hub.Status(context.Background())
	assert.NoError(t, err)
	assert.Len(t, status.Connections, 0, "expected no connections to be left")
	assert.Len(t, status.Registry, 0)
}

func TestNotificationHubConnection_WithBuffering1(t *testing.T) {
	hub := NewNotificationHub(WithSendBuffer(100))
	expectedMessages1 := []string{"test11", "test12", "test13", "test14"}
	expectedMessages2 := []string{"test21", "test22", "test23", "test24"}
	domain1 := "testDomain1"
	domain2 := "testDomain2"
	done := make(chan bool)

	receiveRoutines1 := 100
	receiveRoutines2 := 100

	for i := 0; i < receiveRoutines1; i++ {
		startReceiving(t, hub, domain1, expectedMessages1, done)
	}

	for i := 0; i < receiveRoutines2; i++ {
		startReceiving(t, hub, domain2, expectedMessages2, done)
	}

	for _, v := range expectedMessages1 {
		sends, err := hub.Notify(domain1, v)
		assert.Equal(t, receiveRoutines1, sends)
		assert.NoError(t, err, 0)
	}

	for _, v := range expectedMessages2 {
		sends, err := hub.Notify(domain2, v)
		assert.Equal(t, receiveRoutines2, sends)
		assert.NoError(t, err, 0)
	}

	for i := 0; i < receiveRoutines1+receiveRoutines2; i++ {
		select {
		case <-time.After(500 * time.Millisecond):
			t.Error("timeout")
		case <-done:
		}
	}

	status, err := hub.Status(context.Background())
	assert.NoError(t, err)
	assert.Len(t, status.Connections, 0, "expected no connections to be left")
	assert.Len(t, status.Registry, 0)

}

func TestNotificationHubConnection_WithBuffering2(t *testing.T) {
	hub := NewNotificationHub(WithSendBuffer(1))

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

	status, err := hub.Status(context.Background())
	assert.NoError(t, err)
	assert.Len(t, status.Connections, 1, "expected no connections to be left")
	assert.Len(t, status.Registry, 1)
}

func TestNotificationHubConnection_WithBuffering3(t *testing.T) {
	hub := NewNotificationHub(WithSendBuffer(1))

	hub.Register("test", make(chan interface{}))
	done := make(chan bool)
	go func() {
		sends, err := hub.Notify("test", "test")
		assert.Equal(t, 1, sends, "should have sent 1 (in the buffer)")
		assert.NoError(t, err, "should have sent successfully (with buffering)")
		_, _ = hub.Notify("test", "test")
		t.Error("this should never be reached")
		done <- true
	}()

	select {
	case <-time.After(time.Millisecond):
	case <-done:
		t.Error("reached done but should have blocked (buffer was supposed to be 1 item but we sent 2)")
	}

	ctx, cancelFunc := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancelFunc()
	_, err := hub.Status(ctx)
	assert.EqualError(t, err, ErrHubLocked.Error())
}

func TestNotificationHubConnection_WithBuffering4(t *testing.T) {
	hub := NewNotificationHub(WithSendBuffer(2))

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

	status, err := hub.Status(context.Background())
	assert.NoError(t, err)
	assert.Len(t, status.Connections, 1, "expected no connections to be left")
	assert.Len(t, status.Registry, 1)
}

func TestNotificationHubConnection_WithBuffering5(t *testing.T) {
	hub := NewNotificationHub(WithSendBuffer(4))

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

	status, err := hub.Status(context.Background())
	assert.NoError(t, err)
	assert.Len(t, status.Connections, 1, "expected no connections to be left")
	assert.Len(t, status.Registry, 1)
}

func TestNotificationHub_Metrics(t *testing.T) {
	hub := NewNotificationHub()

	dummyChannel1 := make(chan interface{})
	guid1 := hub.Register("all", dummyChannel1)

	dummyChannel2 := make(chan interface{})
	guid2 := hub.Register("all", dummyChannel2)

	status, err := hub.Status(context.Background())
	assert.NoError(t, err)
	assert.Len(t, status.Connections, 2)

	found1 := false
	found2 := false
	for _, conn := range status.Connections {
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

	status, err := hub.Status(context.Background())
	assert.NoError(t, err)
	assert.Len(t, status.Registry, 1)

	for domain, regItems := range status.Registry {
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

	err = hub.Unregister(guid1, nil, context.Background())
	assert.NoError(t, err)
	if err != nil {
		return
	}

	status, err = hub.Status(context.Background())
	assert.NoError(t, err)
	assert.Len(t, status.Registry, 1)

	for domain, regItems := range status.Registry {
		assert.Equal(t, "all", domain)
		assert.Len(t, regItems, 1)
		for i, regItem := range regItems {
			if i == 0 {
				assert.Equal(t, guid2, regItem)
			}
		}
	}

	guid1 = hub.Register("all", dummyChannel)

	status, err = hub.Status(context.Background())
	assert.NoError(t, err)
	assert.Len(t, status.Registry, 1)

	for domain, regItems := range status.Registry {
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

	status, err := hub.Status(context.Background())
	assert.NoError(t, err)
	assert.Len(t, status.Registry, 2)

	for domain, regItems := range status.Registry {
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

	err = hub.Unregister(guid1, nil, context.Background())
	assert.NoError(t, err)
	if err != nil {
		return
	}

	status, err = hub.Status(context.Background())
	assert.NoError(t, err)
	assert.Len(t, status.Registry, 1)

	for domain, regItems := range status.Registry {
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
				err := hub.Unregister(guid, nil, context.Background())
				assert.NoError(t, err)

				done <- true
				return
			}
		}
	}()
}

// runs until context expires consuming and discarding a channel
func runBenchmarkReceiver(ctx context.Context) chan interface{} {
	channel := make(chan interface{})
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-channel:
			}
		}
	}()
	return channel
}

func runBenchmarkNotify(b *testing.B, receivers int) {
	hub := NewNotificationHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// create consumers
	for i := 0; i < receivers; i++ {
		registrationGUID := hub.Register("testDomain", runBenchmarkReceiver(ctx))
		defer func() {
			ulog.LogIfError(hub.Unregister(registrationGUID, nil, context.Background()))
		}()
	}

	// run benchmark
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		_, _ = hub.Notify("testDomain", "boringData")
	}
	b.StopTimer()
}

func runBenchmarkBroadcast(b *testing.B, receivers int) {
	hub := NewNotificationHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// create consumers
	for i := 0; i < receivers; i++ {
		registrationGUID := hub.Register("testDomain", runBenchmarkReceiver(ctx))
		defer func() {
			ulog.LogIfError(hub.Unregister(registrationGUID, nil, context.Background()))
		}()
	}

	// run benchmark
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		_, _ = hub.Broadcast("boringData")
	}
	b.StopTimer()
}
