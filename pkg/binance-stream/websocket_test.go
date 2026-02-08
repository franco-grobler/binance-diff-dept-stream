package binancestream

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/coder/websocket"
)

func TestNewCoderClient(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		ctx  context.Context
	}{
		{
			name: "with background context",
			ctx:  context.Background(),
		},
		{
			name: "with TODO context",
			ctx:  context.TODO(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := NewCoderClient(tt.ctx)

			if client == nil {
				t.Fatal("expected non-nil CoderClient")
			}
			if client.Context != tt.ctx {
				t.Error("context not set correctly")
			}
		})
	}
}

func TestCoderClient_GetContext(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		ctx  context.Context
	}{
		{
			name: "returns background context",
			ctx:  context.Background(),
		},
		{
			name: "returns TODO context",
			ctx:  context.TODO(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := &CoderClient{Context: tt.ctx}

			got := client.GetContext()

			if got != tt.ctx {
				t.Errorf("GetContext() returned different context")
			}
		})
	}
}

func TestCoderClient_GetContext_WithCancel(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	client := &CoderClient{Context: ctx}

	got := client.GetContext()
	if got != ctx {
		t.Error("GetContext() should return the exact context")
	}

	cancel()

	// Context should be cancelled
	select {
	case <-got.Done():
		// Expected
	default:
		t.Error("context should be cancelled")
	}
}

func TestCoderClient_ImplementsWSClient(t *testing.T) {
	t.Parallel()

	// Compile-time check that CoderClient implements WSClient
	var _ WSClient = (*CoderClient)(nil)
}

func TestWSClient_Interface(t *testing.T) {
	t.Parallel()

	// Test that the interface methods are correctly defined
	type wsClientChecker interface {
		GetContext() context.Context
	}

	var _ wsClientChecker = (*CoderClient)(nil)
}

func TestWSConnection_Interface(t *testing.T) {
	t.Parallel()

	// Compile-time check that *websocket.Conn implements WSConnection
	var _ WSConnection = (*websocket.Conn)(nil)
}

// MockWSClientForDialTests is a mock WSClient for testing dial behaviour.
type MockWSClientForDialTests struct {
	ctx      context.Context
	dialFunc func(url string, opts *websocket.DialOptions) (WSConnection, *http.Response, error)
}

func (m *MockWSClientForDialTests) GetContext() context.Context {
	return m.ctx
}

func (m *MockWSClientForDialTests) Dial(
	url string,
	opts *websocket.DialOptions,
) (WSConnection, *http.Response, error) {
	return m.dialFunc(url, opts)
}

func TestWSClient_Dial_InvalidURL(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	expectedErr := errors.New("invalid URL: missing scheme")

	mockClient := &MockWSClientForDialTests{
		ctx: ctx,
		dialFunc: func(url string, _ *websocket.DialOptions) (WSConnection, *http.Response, error) {
			// Simulate behaviour for invalid URL
			if url == "invalid-url-without-scheme" {
				return nil, nil, expectedErr
			}
			return nil, nil, nil
		},
	}

	conn, resp, err := mockClient.Dial("invalid-url-without-scheme", nil)

	if err == nil {
		t.Error("expected error for invalid URL")
	}
	if err != expectedErr {
		t.Errorf("error = %v, want %v", err, expectedErr)
	}
	if conn != nil {
		t.Error("expected nil connection on error")
	}
	if resp != nil {
		t.Error("expected nil response on error")
	}
}

func TestWSClient_Dial_ConnectionRefused(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	expectedErr := errors.New("connection refused")

	mockClient := &MockWSClientForDialTests{
		ctx: ctx,
		dialFunc: func(_ string, _ *websocket.DialOptions) (WSConnection, *http.Response, error) {
			return nil, nil, expectedErr
		},
	}

	conn, resp, err := mockClient.Dial("wss://localhost:59999/ws", nil)

	if err == nil {
		t.Error("expected error for unreachable host")
	}
	if err != expectedErr {
		t.Errorf("error = %v, want %v", err, expectedErr)
	}
	if conn != nil {
		t.Error("expected nil connection on error")
	}
	_ = resp
}

func TestWSClient_Dial_ContextCancelled(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	mockClient := &MockWSClientForDialTests{
		ctx: ctx,
		dialFunc: func(_ string, _ *websocket.DialOptions) (WSConnection, *http.Response, error) {
			// Check if context is cancelled
			if ctx.Err() != nil {
				return nil, nil, ctx.Err()
			}
			return nil, nil, nil
		},
	}

	conn, resp, err := mockClient.Dial("wss://stream.binance.com:9443/ws", nil)

	if err == nil {
		t.Error("expected error for cancelled context")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("error = %v, want context.Canceled", err)
	}
	if conn != nil {
		t.Error("expected nil connection on error")
	}
	_ = resp
}

func TestWSClient_Dial_Success(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	mockConn := &MockWSConnection{}
	mockResp := &http.Response{StatusCode: http.StatusSwitchingProtocols}

	mockClient := &MockWSClientForDialTests{
		ctx: ctx,
		dialFunc: func(_ string, _ *websocket.DialOptions) (WSConnection, *http.Response, error) {
			return mockConn, mockResp, nil
		},
	}

	conn, resp, err := mockClient.Dial("wss://stream.binance.com:9443/ws", nil)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if conn != mockConn {
		t.Error("expected mock connection to be returned")
	}
	if resp != mockResp {
		t.Error("expected mock response to be returned")
	}
}

func TestWSClient_Dial_WithOptions(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	var receivedOpts *websocket.DialOptions

	mockClient := &MockWSClientForDialTests{
		ctx: ctx,
		dialFunc: func(_ string, opts *websocket.DialOptions) (WSConnection, *http.Response, error) {
			receivedOpts = opts
			return &MockWSConnection{}, nil, nil
		},
	}

	opts := &websocket.DialOptions{
		Subprotocols: []string{"test-protocol"},
	}

	_, _, err := mockClient.Dial("wss://example.com/ws", opts)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if receivedOpts != opts {
		t.Error("options not passed through correctly")
	}
}
