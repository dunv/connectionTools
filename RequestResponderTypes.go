package connectionTools

type Request interface {
	Match(Response) bool
	GUID() string
}

type Response interface {
	Match(Request) bool
	CorrelationGUID() string
}

type BaseRequest struct {
	Guid string      `json:"guid"`
	Data interface{} `json:"data"`
}

func (b *BaseRequest) GUID() string {
	return b.Guid
}

func (b *BaseRequest) Match(r Response) bool {
	return b.Guid == r.CorrelationGUID()
}

type BaseResponse struct {
	CorrelationGuid string      `json:"correlationGuid"`
	Data            interface{} `json:"data"`
}

func (b *BaseResponse) CorrelationGUID() string {
	return b.CorrelationGuid
}

func (b *BaseResponse) Match(r Request) bool {
	return b.CorrelationGuid == r.GUID()
}

func ExtractErr(response interface{}) (interface{}, error) {
	switch typed := response.(type) {
	case error:
		return nil, typed
	default:
		return response, nil
	}
}
