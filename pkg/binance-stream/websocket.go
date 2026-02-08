package binancestream

import (
	"context"
	"net/http"

	"github.com/coder/websocket"
)

// WSConnection abstracts websocket connection operations for testing.
// This interface allows mocking the connection's Read, Write, and Close methods.
type WSConnection interface {
	Read(ctx context.Context) (websocket.MessageType, []byte, error)
	Write(ctx context.Context, typ websocket.MessageType, p []byte) error
	Close(code websocket.StatusCode, reason string) error
}

// Verify *websocket.Conn implements WSConnection at compile time.
var _ WSConnection = (*websocket.Conn)(nil)

// WSClient handles WebSocket dialling and returns a mockable connection.
// Golang has no standard websocket implementation, so we use coder/websocket.
type WSClient interface {
	GetContext() context.Context
	Dial(url string, opts *websocket.DialOptions) (WSConnection, *http.Response, error)
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
// Returns WSConnection interface which *websocket.Conn satisfies.
func (c *CoderClient) Dial(
	url string, opts *websocket.DialOptions,
) (WSConnection, *http.Response, error) {
	return websocket.Dial(c.Context, url, opts)
}

// GetContext implements [WSClient].
func (c *CoderClient) GetContext() context.Context {
	return c.Context
}
