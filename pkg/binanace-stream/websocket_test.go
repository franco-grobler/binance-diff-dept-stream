package binanacestream

import (
	"context"
	"testing"
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

func TestCoderClient_Dial_InvalidURL(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	client := NewCoderClient(ctx)

	// Test with an invalid URL - this should fail
	conn, resp, err := client.Dial("invalid-url-without-scheme", nil)

	if err == nil {
		t.Error("expected error for invalid URL")
		if conn != nil {
			conn.Close(1000, "test cleanup")
		}
	}
	if conn != nil {
		t.Error("expected nil connection on error")
	}
	// resp may or may not be nil depending on how far the dial got
	_ = resp
}

func TestCoderClient_Dial_ConnectionRefused(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	client := NewCoderClient(ctx)

	// Test with a valid URL format but unreachable host
	conn, resp, err := client.Dial("wss://localhost:59999/ws", nil)

	if err == nil {
		t.Error("expected error for unreachable host")
		if conn != nil {
			conn.Close(1000, "test cleanup")
		}
	}
	if conn != nil {
		t.Error("expected nil connection on error")
	}
	_ = resp
}

func TestCoderClient_Dial_ContextCancelled(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	client := NewCoderClient(ctx)

	conn, resp, err := client.Dial("wss://stream.binance.com:9443/ws", nil)

	if err == nil {
		t.Error("expected error for cancelled context")
		if conn != nil {
			conn.Close(1000, "test cleanup")
		}
	}
	_ = resp
}
