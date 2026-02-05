package binanacestream

import (
	"context"
	"testing"
	"time"
)

// MockClient for testing
type MockClient struct {
	Data [][]byte
}

func (m *MockClient) SubscribeDepthSimple(ctx context.Context, symbols []string, outCh chan<- []byte) error {
	go func() {
		for _, d := range m.Data {
			outCh <- d
		}
		close(outCh) // Close to signal done in tests
	}()
	return nil
}

func TestSubscribeDepthSimple(t *testing.T) {
	// Mock Data
	mockData := [][]byte{
		[]byte(`{"e":"depthUpdate","s":"BNBBTC"}`),
		[]byte(`{"e":"depthUpdate","s":"ETHBTC"}`),
	}

	client := &MockClient{Data: mockData}
	outCh := make(chan []byte, 10)
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	err := client.SubscribeDepthSimple(ctx, []string{"BNBBTC"}, outCh)
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	// Read from channel
	count := 0
	for range outCh {
		count++
		if count == 2 {
			break
		}
	}

	if count != 2 {
		t.Errorf("Expected 2 messages, got %d", count)
	}
}
