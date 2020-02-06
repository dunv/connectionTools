package requestResponder

import (
	"context"
	"fmt"
	"testing"
	"time"

	nh "github.com/dunv/connectionTools/notificationHub"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestRequestResponse_NoOneListeningError(t *testing.T) {
	requestResponder := NewRequestResponder()
	domain := "testDomain"

	responseChannel := requestResponder.Request(domain, &BaseRequest{GUID: uuid.New().String()})
	_, err := ExtractErr(<-responseChannel)
	assert.EqualError(t, err, nh.ErrNoOneListeningToRequest.Error())
}

func TestRequestResponse_Success(t *testing.T) {
	requestResponder := NewRequestResponder()
	domain := "testDomain"

	requestsChannel := make(chan interface{})
	cancelRequests := requestResponder.AddRequestChannel(domain, requestsChannel)
	defer cancelRequests()

	responsesChannel := make(chan interface{})
	cancelResponse := requestResponder.AddResponseChannel(domain, responsesChannel)
	defer cancelResponse()

	// Inject traffic
	go publishRandomResponses("no match", 99, 100000, responsesChannel)

	requestGUID := uuid.New().String()
	requestData := "injectedResponse"
	go func() {
		// Publish correct response after a while
		<-time.After(1 * time.Second)
		go publishResponse(requestGUID, requestsChannel, responsesChannel, requestData)
	}()

	responseChannel := requestResponder.Request(domain, &BaseRequest{GUID: requestGUID})

	res, err := ExtractErr(<-responseChannel)
	assert.NoError(t, err, "should have been successful")
	assert.IsType(t, &BaseResponse{}, res)
	assert.IsType(t, "", res.(*BaseResponse).Data)
	assert.Equal(t, requestData, res.(*BaseResponse).Data.(string))
}

func TestRequestResponse_RequestTimeout(t *testing.T) {
	requestResponder := NewRequestResponder()
	domain := "testDomain"

	requestsChannel := make(chan interface{})
	cancelRequests := requestResponder.AddRequestChannel(domain, requestsChannel)
	defer cancelRequests()

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	responseChannel := requestResponder.Request(domain, &BaseRequest{GUID: uuid.New().String()}, ctx)

	_, err := ExtractErr(<-responseChannel)
	assert.EqualError(t, err, context.DeadlineExceeded.Error())
}

func TestRequestResponse_RequestCancelled(t *testing.T) {
	requestResponder := NewRequestResponder()
	domain := "testDomain"

	requestsChannel := make(chan interface{})
	cancelRequests := requestResponder.AddRequestChannel(domain, requestsChannel)
	defer cancelRequests()

	ctx, cancel := context.WithCancel(context.Background())
	responseChannel := requestResponder.Request(domain, &BaseRequest{GUID: uuid.New().String()}, ctx)
	cancel()

	_, err := ExtractErr(<-responseChannel)
	assert.EqualError(t, err, context.Canceled.Error())
}

func TestRequestResponse_SuccessMultipleInput(t *testing.T) {
	requestResponder := NewRequestResponder()
	domain := "testDomain"

	requestsChannel := make(chan interface{})
	cancelRequests := requestResponder.AddRequestChannel(domain, requestsChannel)

	allResponsesChannel1 := make(chan interface{})
	stopConsuming1 := requestResponder.AddResponseChannel(domain, allResponsesChannel1)

	allResponsesChannel2 := make(chan interface{})
	stopConsuming2 := requestResponder.AddResponseChannel(domain, allResponsesChannel2)

	allResponsesChannel3 := make(chan interface{})
	stopConsuming3 := requestResponder.AddResponseChannel(domain, allResponsesChannel3)

	allResponsesChannel4 := make(chan interface{})
	stopConsuming4 := requestResponder.AddResponseChannel(domain, allResponsesChannel4)

	requestGUID := uuid.New().String()
	responseChannel := requestResponder.Request(domain, &BaseRequest{GUID: requestGUID, Data: "request"})
	requestData := "injectedPayload"

	go publishRandomResponses(requestGUID, 10000, 10000, allResponsesChannel1)
	go publishRandomResponses(requestGUID, 10000, 10000, allResponsesChannel2)
	go publishRandomResponses(requestGUID, 10000, 10000, allResponsesChannel3)
	go publishRandomResponses(requestGUID, 10000, 10000, allResponsesChannel4)

	go publishResponse(requestGUID, requestsChannel, allResponsesChannel3, requestData)

	res, err := ExtractErr(<-responseChannel)
	assert.NoError(t, err, "should have been successful")
	assert.IsType(t, &BaseResponse{}, res)
	assert.IsType(t, "", res.(*BaseResponse).Data)
	assert.Equal(t, requestData, res.(*BaseResponse).Data.(string))

	cancelRequests()
	stopConsuming1()
	stopConsuming2()
	stopConsuming3()
	stopConsuming4()
}

func publishRandomResponses(correctGUID string, correctIndex int, length int, channel chan interface{}) {
	for i := 0; i < length; i++ {
		var guid string
		if i == correctIndex {
			guid = correctGUID
			// fmt.Println("---> ", i, "CORRECT")
		} else {
			guid = uuid.New().String()
			// fmt.Println("---> ", i, "wrong")
		}
		channel <- &BaseResponse{
			CorrelationGUID: guid,
			Data:            fmt.Sprintf("%d", i),
		}
		// fmt.Println("---> ", i, "done")
	}
}

func publishResponse(correctGUID string, requestChannel chan interface{}, responseChannel chan interface{}, requestData string) {
	for request := range requestChannel {
		if request.(*BaseRequest).GUID == correctGUID {
			responseChannel <- &BaseResponse{CorrelationGUID: correctGUID, Data: requestData}
		}
	}
}
