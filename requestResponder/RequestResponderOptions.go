package requestResponder

type RequestResponderOption interface {
	apply(*requestResponderOptions)
}

type requestResponderOptions struct {
	debug bool
}

type funcrequestResponderOption struct {
	f func(*requestResponderOptions)
}

func (fdo *funcrequestResponderOption) apply(do *requestResponderOptions) {
	fdo.f(do)
}

func newFuncrequestResponderOption(f func(*requestResponderOptions)) *funcrequestResponderOption {
	return &funcrequestResponderOption{f: f}
}

func WithDebug() RequestResponderOption {
	return newFuncrequestResponderOption(func(o *requestResponderOptions) {
		o.debug = false
	})
}
