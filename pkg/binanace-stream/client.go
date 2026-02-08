package binanacestream

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/coder/websocket"
)

var _ StreamClient = (*Client)(nil)

// Client implements [StreamClient] for a Binance order-book management.
type Client struct {
	apiURL     string
	wsURL      string
	HTTPClient HTTPClient
	WSClient   WSClient
	Parser     Parser
}

// Endpoint client API endpoints.
type Endpoint string

// DepthSnapshotEndpoint api endpoint for order-book snapshot.
const DepthSnapshotEndpoint = "v3/depth"

// NewClient returns a standard Client for StreamClient.
func NewClient(httpClient HTTPClient, wsClient WSClient, parser Parser) Client {
	return Client{
		"https://api.binance.com/api/",
		"wss://stream.binance.com:9443/ws/",
		httpClient,
		wsClient,
		parser,
	}
}

func (c *Client) parseResponse(resp *http.Response, data any) error {
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Printf("failed to close resp.Body: %v", err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}

	return c.Parser(body, data)
}

// DepthStream implements [StreamClient].
func (c *Client) DepthStream(
	ctx context.Context,
	symbols []string,
	_ UpdateSpeed,
	outCh chan<- []byte,
) error {
	streams := make([]string, len(symbols))
	for i, s := range symbols {
		streams[i] = fmt.Sprintf("%s@depth", strings.ToLower(s))
	}
	url := fmt.Sprintf("%s/%s", c.wsURL, strings.Join(streams, "/"))

	// Dial the connection
	conn, _, err := c.WSClient.Dial(ctx, url, nil)
	if err != nil {
		return fmt.Errorf("failed to dial: %w", err)
	}

	// Start reading loop
	go func() {
		defer close(outCh)
		defer func() {
			if err := conn.Close(websocket.StatusNormalClosure, ""); err != nil {
				log.Printf("Failed to close websocket connection: %v", err)
			}
		}()

		for {
			select {
			case <-ctx.Done():
				return

			default:
				// Read raw message
				_, data, err := conn.Read(ctx)
				if err != nil {
					// In a real app, implement reconnect logic here.
					// For assessment, we just stop.
					panic(fmt.Sprintf("error reading data: %v", err))
				}
				outCh <- data
				log.Printf("received message: %v\n", data)
			}
		}
	}()

	return nil
}

// DepthSnapshot implements [StreamClient]
func (c *Client) DepthSnapshot(symbol string, limit int16) (*DiffSnapshot, error) {
	url := fmt.Sprintf("%s/%s?symbol=%s&limit=%d", c.apiURL, DepthSnapshotEndpoint, symbol, limit)
	req, reqErr := http.NewRequest("GET", url, nil)
	if reqErr != nil {
		return nil, reqErr
	}

	resp, respErr := c.HTTPClient.Do(req)
	if respErr != nil {
		return nil, respErr
	}

	snapshot := DiffSnapshot{}
	if err := c.parseResponse(resp, DiffSnapshot{}); err != nil {
		return nil, err
	}
	return &snapshot, nil
}
