package binancestream

import "net/http"

// HTTPClient Define the interface for the HTTP client's behaviour
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

var _ HTTPClient = (*http.Client)(nil)

// NewHTTPClient creates a basic new HTTPClient.
func NewHTTPClient() *http.Client {
	return &http.Client{}
}
