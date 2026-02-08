package binanacestream

import (
	"context"
	"net/http"

	"github.com/coder/websocket"
)

// UpdateSpeed allowed values for depth when streaming from websocket.
type UpdateSpeed int

// 'Enum' for UpdateSpeed
const (
	Second     UpdateSpeed = 1000
	DeciSecond UpdateSpeed = 100
)

// Parser serves as an alias for unmarshalling functions
type Parser func(data []byte, v any) error

// DiffStream reads Diff. Depth Stream responses.
// Example:
//
//	{
//	    "e": "depthUpdate",     // Event type
//	    "E": 1672515782136,     // Event time
//	    "s": "BNBBTC",          // Symbol
//	    "U": 157,               // First update ID in event
//	    "u": 160,               // Final update ID in event
//	    "b": [                  // Bids to be updated
//	        [
//	            "0.0024",       // Price level to be updated
//	            "10"            // Quantity
//	        ]
//	    ],
//	    "a": [                  // Asks to be updated
//	        [
//	            "0.0026",       // Price level to be updated
//	            "100"           // Quantity
//	        ]
//	    ]
//	}
type DiffStream struct {
	EventType     string      `json:"e"`
	EventTime     string      `json:"E"`
	Symbol        string      `json:"s"`
	FirstUpdateID uint        `json:"U"`
	FinalUpdateID uint        `json:"u"`
	UpdateBids    [][2]string `json:"b"`
	UpdateAsks    [][2]string `json:"a"`
}

// DiffSnapshot reads a snapshot of the last N
type DiffSnapshot struct {
	LastUpdateID int32       `json:"lastUpdateId"`
	Bids         [][2]string `json:"bids"`
}

// StreamClient defines required methods for a client to manage a local order book.
type StreamClient interface {
	DepthStream(
		ctx context.Context,
		symbols []string,
		updateSpeed UpdateSpeed,
		outCh chan<- []byte,
	) error
	DepthSnapshot(symbol string, limit int16) (*DiffSnapshot, error)
}

// HTTPClient Define the interface for the HTTP client's behaviour
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// WSClient piggybacks on coder/websocket. Golang has no standard websocket implementation.
type WSClient interface {
	Dial(
		ctx context.Context,
		url string,
		opts *websocket.DialOptions,
	) (*websocket.Conn, *http.Response, error)
}
