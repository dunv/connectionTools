[![Build Status](https://travis-ci.org/dunv/connectionTools.svg?branch=master)](https://travis-ci.org/dunv/connectionTools)
[![GoDoc](https://godoc.org/github.com/dunv/connectionTools?status.svg)](https://godoc.org/github.com/dunv/connectionTools)
[![codecov](https://codecov.io/gh/dunv/connectionTools/branch/master/graph/badge.svg)](https://codecov.io/gh/dunv/connectionTools)

# connectionTools

Trying to move all recurring connection patterns into one importable library.

## `NotificationHub` 
a container where multiple domain-bound subscribers can be added. A notify on the container will notify all subscribers of the notify-domain. See tests for complete code examples.
```go
    import ct "github.com/dunv/connectionTools"

    // create container
    hub := ct.NewNotificationHub()
    
    // this channel will receive all notifications
    channel := make(chan interface{})
    id := hub.Register(domain, channel)
    defer hub.Unregister(id)

    // consume channel
    go func() {
        for {
            item := <-channel
        }
    }()

    // notify will block until consumed (unless we specify buffering on the hub)
    n, err := hub.Notify(domain, "test")
```

## `RequestResponder` 
a container which consumes multiple input-channels concurrently. Requesting on this container will search all following consumed objects on the input channel for a match to the request. See tests for complete code examples.
```go
    import ct "github.com/dunv/connectionTools"

    // create container
    requestResponder := ct.NewRequestResponder()

    // incoming stream should publish on this channel
	var incoming chan interface{}
    
    // have the container start consuming the incoming stream
	cancel := requestResponder.StartConsuming("testDomain", incoming)
	defer cancel()

    // wait for the response to arrive (if needed we can pass a context for timeout and cancel)
    res := <-requestResponder.Request("testDomain", &ct.BaseRequest{GUID: "stringGUID"})
    
    // ExtractErr is a wrapper around switch res.(type) and can be omitted
	response, err := ct.ExtractErr(res)
```
