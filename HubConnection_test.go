package connectionTools

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestHubConnection(t *testing.T) {
	hub := NewNotificationHub(5 * time.Second)

	domain := "testDomain"
	data := "test"
	receiver1 := make(chan interface{})
	receiver2 := make(chan interface{})

	hub.Register(domain, receiver1)
	hub.Register(domain, receiver2)

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		test := <-receiver1
		assert.Equal(t, data, test.(string))
		wg.Done()
	}()

	go func() {
		test := <-receiver2
		assert.Equal(t, data, test.(string))
		wg.Done()
	}()

	hub.Notify(domain, data)
	wg.Wait()
}
