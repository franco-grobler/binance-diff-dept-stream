package binanacestream

import "net/http"

type StreamClient interface {
	DepthStream(symbol string, updateSpeed UpdateSpeed) (OrderBook, error)
}

// HTTPClient Define the interface for the HTTP client's behavior
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

type WSClient interface {
	Stream(req *http.Request) error
}

type Client struct {
	HTTPClient HTTPClient
}
