package binancestream

import (
	"context"
	"net/http"

	"github.com/coder/websocket"
)

// WSClient piggybacks on coder/websocket. Golang has no standard websocket implementation.
type WSClient interface {
	GetContext() context.Context
	Dial(
		url string,
		opts *websocket.DialOptions,
	) (*websocket.Conn, *http.Response, error)
}

var _ WSClient = (*CoderClient)(nil)

// CoderClient implements WSClient using coder/websocket.
type CoderClient struct {
	Context context.Context
}

// NewCoderClient creates a new CoderClient.
func NewCoderClient(ctx context.Context) *CoderClient {
	return &CoderClient{
		ctx,
	}
}

// Dial implements [WSClient].
func (c *CoderClient) Dial(
	url string, opts *websocket.DialOptions,
) (*websocket.Conn, *http.Response, error) {
	return websocket.Dial(c.Context, url, opts)
}

// GetContext implements [WSClient].
func (c *CoderClient) GetContext() context.Context {
	return c.Context
}
