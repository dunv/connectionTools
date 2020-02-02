package connectionTools

type Request interface{ Match(interface{}) bool }

type Response interface{ Identifier() string }

type BaseRequest struct {
	GUID string      `json:"guid"`
	Data interface{} `json:"data"`
}

func (b *BaseRequest) Match(r interface{}) bool { return b.GUID == r.(*BaseResponse).CorrelationGUID }

type BaseResponse struct {
	CorrelationGUID string      `json:"correlationGuid"`
	Data            interface{} `json:"data"`
}

func ExtractErr(response interface{}) (interface{}, error) {
	switch typed := response.(type) {
	case error:
		return nil, typed
	default:
		return response, nil
	}
}
