package connectionManager

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestConnectTwo(t *testing.T) {
	events := make(chan ConnectionChange)
	m := NewConnectionManager([]string{"conn1", "conn2"}, nil, &events)

	wait := make(chan struct{})
	go func() {
		first := <-events
		assert.True(t, first.Connected)
		wait <- struct{}{}
	}()

	assert.NoError(t, m.Connect("group1", "conn1"))
	status := m.Status()
	assert.False(t, status["group1"].Connected)
	assert.NoError(t, m.Connect("group1", "conn2"))
	status = m.Status()
	assert.True(t, status["group1"].Connected)

	select {
	case <-wait:
	case <-time.After(time.Second):
		t.Error("did not receive required events")
	}
}

func TestUnknownConnection(t *testing.T) {
	m := NewConnectionManager([]string{"conn1", "conn2"}, nil, nil)
	assert.Error(t, m.Connect("group1", "conn3"))
}

func TestUnknownMetadata(t *testing.T) {
	m := NewConnectionManager([]string{"conn1", "conn2"}, nil, nil)
	assert.Error(t, m.SetMetadata("group1", "conn3", "test"))
}

func TestConnectTwoAndDisconnectOne(t *testing.T) {
	events := make(chan ConnectionChange)
	m := NewConnectionManager([]string{"conn1", "conn2"}, nil, &events)

	wait := make(chan struct{})
	go func() {
		first := <-events
		assert.True(t, first.Connected)
		second := <-events
		assert.False(t, second.Connected)
		wait <- struct{}{}
	}()

	assert.NoError(t, m.Connect("group1", "conn1"))
	status := m.Status()
	assert.False(t, status["group1"].Connected)

	assert.NoError(t, m.Connect("group1", "conn2"))
	status = m.Status()
	assert.True(t, status["group1"].Connected)

	assert.NoError(t, m.Disconnect("group1", "conn2"))
	status = m.Status()
	assert.False(t, status["group1"].Connected)

	select {
	case <-wait:
	case <-time.After(time.Second):
		t.Error("did not receive required events")
	}
}

func TestConnectOneAndMetadata(t *testing.T) {
	events := make(chan ConnectionChange)
	m := NewConnectionManager([]string{"conn1"}, []string{"meta1"}, &events)

	wait := make(chan struct{})
	go func() {
		first := <-events
		assert.True(t, first.Connected)
		wait <- struct{}{}
	}()

	assert.NoError(t, m.Connect("group1", "conn1"))
	status := m.Status()
	assert.False(t, status["group1"].Connected)

	assert.NoError(t, m.SetMetadata("group1", "meta1", "test"))
	status = m.Status()
	assert.True(t, status["group1"].Connected)

	select {
	case <-wait:
	case <-time.After(time.Second):
		t.Error("did not receive required events")
	}
}
