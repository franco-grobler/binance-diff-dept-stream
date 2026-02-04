package binanacestream

const Endpoint = "wss://stream.binance.com:9443"

type UpdateSpeed int

const (
	Second     UpdateSpeed = 1000
	DeciSecond UpdateSpeed = 100
)

// OrderBook reads Diff. Depth Stream responses.
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
type OrderBook struct {
	EventType     string      `json:"e"`
	EventTime     string      `json:"E"`
	Symbol        string      `json:"s"`
	FirstUpdateID uint        `json:"U"`
	FinalUpdateID uint        `json:"u"`
	UpdateBids    [][2]string `json:"b"`
	UpdateAsks    [][2]string `json:"a"`
}
