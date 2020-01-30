package connectionTools

type Request interface{ Match(Response) bool }

type Response interface{ Identifier() string }

type BaseRequest struct {
	GUID string      `json:"guid"`
	Data interface{} `json:"data"`
}

func (b *BaseRequest) Match(r Response) bool { return b.GUID == r.Identifier() }

type BaseResponse struct {
	CorrelationGUID string      `json:"correlationGuid"`
	Data            interface{} `json:"data"`
}

func (b *BaseResponse) Identifier() string { return b.CorrelationGUID }

func ExtractErr(response interface{}) (interface{}, error) {
	switch typed := response.(type) {
	case error:
		return nil, typed
	default:
		return response, nil
	}
}
