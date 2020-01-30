package connectionTools

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestRequestResponse_Success(t *testing.T) {
	requestResponder := NewRequestResponder()
	allResponsesChannel := make(chan interface{})
	domain := "testDomain"
	stopConsuming := requestResponder.StartConsuming(domain, allResponsesChannel)
	defer stopConsuming()

	requestGUID := uuid.New().String()
	responseChannel := requestResponder.Request(domain, &BaseRequest{GUID: requestGUID})

	go publishRandomResponses(requestGUID, 99, 100, allResponsesChannel)

	res, err := ExtractErr(<-responseChannel)
	assert.NoError(t, err, "should have been successful")
	assert.IsType(t, &BaseResponse{}, res)
	assert.IsType(t, "", res.(*BaseResponse).Data)
	assert.Equal(t, "response 99", res.(*BaseResponse).Data.(string))
}

func TestRequestResponse_SuccessMultipleInput(t *testing.T) {
	requestResponder := NewRequestResponder()
	domain := "testDomain"

	allResponsesChannel1 := make(chan interface{})
	stopConsuming1 := requestResponder.StartConsuming(domain, allResponsesChannel1)
	defer stopConsuming1()

	allResponsesChannel2 := make(chan interface{})
	stopConsuming2 := requestResponder.StartConsuming(domain, allResponsesChannel2)
	defer stopConsuming2()

	allResponsesChannel3 := make(chan interface{})
	stopConsuming3 := requestResponder.StartConsuming(domain, allResponsesChannel3)
	defer stopConsuming3()

	allResponsesChannel4 := make(chan interface{})
	stopConsuming4 := requestResponder.StartConsuming(domain, allResponsesChannel4)
	defer stopConsuming4()

	requestGUID := uuid.New().String()
	responseChannel := requestResponder.Request(domain, &BaseRequest{GUID: requestGUID})

	go publishRandomResponses(requestGUID, 1000, 1000, allResponsesChannel1)
	go publishRandomResponses(requestGUID, 1000, 1000, allResponsesChannel2)
	go publishRandomResponses(requestGUID, 1000, 1000, allResponsesChannel3)

	<-time.After(200 * time.Millisecond)
	go publishRandomResponses(requestGUID, 999, 1000, allResponsesChannel4)

	res, err := ExtractErr(<-responseChannel)
	assert.NoError(t, err, "should have been successful")
	assert.IsType(t, &BaseResponse{}, res)
	assert.IsType(t, "", res.(*BaseResponse).Data)
	assert.Equal(t, "response 999", res.(*BaseResponse).Data.(string))
}

func TestRequestResponse_RequestTimeout(t *testing.T) {
	requestResponder := NewRequestResponder()
	allResponsesChannel := make(chan interface{})
	domain := "testDomain"
	stopConsuming := requestResponder.StartConsuming(domain, allResponsesChannel)
	defer stopConsuming()

	requestGUID := uuid.New().String()
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	responseChannel := requestResponder.Request(domain, &BaseRequest{GUID: requestGUID}, ctx)

	go publishRandomResponses(requestGUID, 100, 100, allResponsesChannel) // index out of range -> never inject correct guid

	_, err := ExtractErr(<-responseChannel)
	assert.Error(t, err, "should have received a deadline exceeded")
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
			Data:            fmt.Sprintf("response %d", i),
		}
		// fmt.Println("---> ", i, "done")
	}
}
