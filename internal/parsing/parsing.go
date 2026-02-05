package parsing

import (
	"encoding/json"
	"sync"
	"unsafe"

	"github.com/valyala/fastjson"
)

// parserPool is a pool of fastjson.Parser instances to reduce allocations.
// Each Parser is reusable and thread-safe when used from a single goroutine.
var parserPool = sync.Pool{
	New: func() interface{} {
		return &fastjson.Parser{}
	},
}

// depthUpdatePool is a pool of DepthUpdate structs to reduce allocations.
var depthUpdatePool = sync.Pool{
	New: func() interface{} {
		return &DepthUpdate{
			Bids: make([][2]string, 0, 20),
			Asks: make([][2]string, 0, 20),
		}
	},
}

// DepthUpdate represents the incoming JSON structure from Binance.
// defined here to match the input format for benchmarking.
type DepthUpdate struct {
	Event         string      `json:"e"`
	EventTime     int64       `json:"E"`
	Symbol        string      `json:"s"`
	FirstUpdateID int64       `json:"U"`
	FinalUpdateID int64       `json:"u"`
	Bids          [][2]string `json:"b"`
	Asks          [][2]string `json:"a"`
}

// CombinedStreamWrapper wraps the DepthUpdate when using combined streams.
// Combined stream format: {"stream":"btcusdt@depth","data":{...}}
type CombinedStreamWrapper struct {
	Stream string          `json:"stream"`
	Data   json.RawMessage `json:"data"`
}

// ParseStandard uses encoding/json to parse the byte slice.
// Handles both direct event format and combined stream format.
func ParseStandard(data []byte) (*DepthUpdate, error) {
	// First try to detect if this is a combined stream wrapper
	var wrapper CombinedStreamWrapper
	if err := json.Unmarshal(data, &wrapper); err != nil {
		return nil, err
	}

	// If it has stream/data, parse the inner data
	if wrapper.Stream != "" && len(wrapper.Data) > 0 {
		var update DepthUpdate
		if err := json.Unmarshal(wrapper.Data, &update); err != nil {
			return nil, err
		}
		return &update, nil
	}

	// Otherwise, parse as direct event
	var update DepthUpdate
	if err := json.Unmarshal(data, &update); err != nil {
		return nil, err
	}
	return &update, nil
}

// ParseStandardDirect parses only the direct event format (for benchmarking).
func ParseStandardDirect(data []byte) (*DepthUpdate, error) {
	var update DepthUpdate
	if err := json.Unmarshal(data, &update); err != nil {
		return nil, err
	}
	return &update, nil
}

// ParseFastJSON uses valyala/fastjson to parse the byte slice.
// It returns the struct to keep the interface consistent for comparison,
// though in a real hot path we might pass the values directly to the OrderBook
// to avoid the struct allocation entirely.
//
// This function handles both:
// 1. Direct event format: {"e":"depthUpdate", ...}
// 2. Combined stream format: {"stream":"btcusdt@depth", "data":{...}}
func ParseFastJSON(data []byte) (*DepthUpdate, error) {
	var p fastjson.Parser
	v, err := p.ParseBytes(data)
	if err != nil {
		return nil, err
	}

	// Check if this is a combined stream wrapper (has "stream" and "data" keys)
	// Combined stream format: {"stream":"btcusdt@depth","data":{...actual event...}}
	if v.Exists("stream") && v.Exists("data") {
		v = v.Get("data")
	}

	update := &DepthUpdate{}
	update.Event = string(v.GetStringBytes("e"))
	update.EventTime = v.GetInt64("E")
	update.Symbol = string(v.GetStringBytes("s"))
	update.FirstUpdateID = v.GetInt64("U")
	update.FinalUpdateID = v.GetInt64("u")

	// Parse Bids
	bidsValue := v.Get("b")
	if bidsValue != nil {
		bidsArray, _ := bidsValue.Array()
		update.Bids = make([][2]string, len(bidsArray))
		for i, b := range bidsArray {
			pair, _ := b.Array()
			if len(pair) >= 2 {
				update.Bids[i][0] = string(pair[0].GetStringBytes())
				update.Bids[i][1] = string(pair[1].GetStringBytes())
			}
		}
	}

	// Parse Asks
	asksValue := v.Get("a")
	if asksValue != nil {
		asksArray, _ := asksValue.Array()
		update.Asks = make([][2]string, len(asksArray))
		for i, a := range asksArray {
			pair, _ := a.Array()
			if len(pair) >= 2 {
				update.Asks[i][0] = string(pair[0].GetStringBytes())
				update.Asks[i][1] = string(pair[1].GetStringBytes())
			}
		}
	}

	return update, nil
}

// ParseFastJSONDirect parses only the direct event format (for benchmarking).
// This is the original implementation without combined stream handling.
func ParseFastJSONDirect(data []byte) (*DepthUpdate, error) {
	var p fastjson.Parser
	v, err := p.ParseBytes(data)
	if err != nil {
		return nil, err
	}

	update := &DepthUpdate{}
	update.Event = string(v.GetStringBytes("e"))
	update.EventTime = v.GetInt64("E")
	update.Symbol = string(v.GetStringBytes("s"))
	update.FirstUpdateID = v.GetInt64("U")
	update.FinalUpdateID = v.GetInt64("u")

	// Parse Bids
	bidsValue := v.Get("b")
	if bidsValue != nil {
		bidsArray, _ := bidsValue.Array()
		update.Bids = make([][2]string, len(bidsArray))
		for i, b := range bidsArray {
			pair, _ := b.Array()
			if len(pair) >= 2 {
				update.Bids[i][0] = string(pair[0].GetStringBytes())
				update.Bids[i][1] = string(pair[1].GetStringBytes())
			}
		}
	}

	// Parse Asks
	asksValue := v.Get("a")
	if asksValue != nil {
		asksArray, _ := asksValue.Array()
		update.Asks = make([][2]string, len(asksArray))
		for i, a := range asksArray {
			pair, _ := a.Array()
			if len(pair) >= 2 {
				update.Asks[i][0] = string(pair[0].GetStringBytes())
				update.Asks[i][1] = string(pair[1].GetStringBytes())
			}
		}
	}

	return update, nil
}

// ============================================================================
// OPTIMIZED FUNCTIONS (using sync.Pool)
// ============================================================================

// ParseFastJSONPooled uses sync.Pool to reuse the fastjson.Parser.
// This reduces allocations in hot paths.
func ParseFastJSONPooled(data []byte) (*DepthUpdate, error) {
	p := parserPool.Get().(*fastjson.Parser)
	defer parserPool.Put(p)

	v, err := p.ParseBytes(data)
	if err != nil {
		return nil, err
	}

	// Check if this is a combined stream wrapper
	if v.Exists("stream") && v.Exists("data") {
		v = v.Get("data")
	}

	update := &DepthUpdate{}
	update.Event = string(v.GetStringBytes("e"))
	update.EventTime = v.GetInt64("E")
	update.Symbol = string(v.GetStringBytes("s"))
	update.FirstUpdateID = v.GetInt64("U")
	update.FinalUpdateID = v.GetInt64("u")

	// Parse Bids
	bidsValue := v.Get("b")
	if bidsValue != nil {
		bidsArray, _ := bidsValue.Array()
		update.Bids = make([][2]string, len(bidsArray))
		for i, b := range bidsArray {
			pair, _ := b.Array()
			if len(pair) >= 2 {
				update.Bids[i][0] = string(pair[0].GetStringBytes())
				update.Bids[i][1] = string(pair[1].GetStringBytes())
			}
		}
	}

	// Parse Asks
	asksValue := v.Get("a")
	if asksValue != nil {
		asksArray, _ := asksValue.Array()
		update.Asks = make([][2]string, len(asksArray))
		for i, a := range asksArray {
			pair, _ := a.Array()
			if len(pair) >= 2 {
				update.Asks[i][0] = string(pair[0].GetStringBytes())
				update.Asks[i][1] = string(pair[1].GetStringBytes())
			}
		}
	}

	return update, nil
}

// ParseFastJSONPooledDirect parses only direct format using pooled parser.
func ParseFastJSONPooledDirect(data []byte) (*DepthUpdate, error) {
	p := parserPool.Get().(*fastjson.Parser)
	defer parserPool.Put(p)

	v, err := p.ParseBytes(data)
	if err != nil {
		return nil, err
	}

	update := &DepthUpdate{}
	update.Event = string(v.GetStringBytes("e"))
	update.EventTime = v.GetInt64("E")
	update.Symbol = string(v.GetStringBytes("s"))
	update.FirstUpdateID = v.GetInt64("U")
	update.FinalUpdateID = v.GetInt64("u")

	// Parse Bids
	bidsValue := v.Get("b")
	if bidsValue != nil {
		bidsArray, _ := bidsValue.Array()
		update.Bids = make([][2]string, len(bidsArray))
		for i, b := range bidsArray {
			pair, _ := b.Array()
			if len(pair) >= 2 {
				update.Bids[i][0] = string(pair[0].GetStringBytes())
				update.Bids[i][1] = string(pair[1].GetStringBytes())
			}
		}
	}

	// Parse Asks
	asksValue := v.Get("a")
	if asksValue != nil {
		asksArray, _ := asksValue.Array()
		update.Asks = make([][2]string, len(asksArray))
		for i, a := range asksArray {
			pair, _ := a.Array()
			if len(pair) >= 2 {
				update.Asks[i][0] = string(pair[0].GetStringBytes())
				update.Asks[i][1] = string(pair[1].GetStringBytes())
			}
		}
	}

	return update, nil
}

// GetDepthUpdate returns a DepthUpdate from the pool.
// Caller must call ReleaseDepthUpdate when done.
func GetDepthUpdate() *DepthUpdate {
	return depthUpdatePool.Get().(*DepthUpdate)
}

// ReleaseDepthUpdate returns a DepthUpdate to the pool.
func ReleaseDepthUpdate(d *DepthUpdate) {
	// Reset fields to avoid data leaks
	d.Event = ""
	d.EventTime = 0
	d.Symbol = ""
	d.FirstUpdateID = 0
	d.FinalUpdateID = 0
	d.Bids = d.Bids[:0]
	d.Asks = d.Asks[:0]
	depthUpdatePool.Put(d)
}

// ============================================================================
// ULTRA-OPTIMIZED FUNCTIONS (using unsafe for zero-copy string conversion)
// ============================================================================

// unsafeString converts a byte slice to a string without allocation.
// WARNING: The returned string shares memory with the byte slice.
// Do not modify the byte slice while the string is in use.
func unsafeString(b []byte) string {
	return unsafe.String(unsafe.SliceData(b), len(b))
}

// ParseFastJSONUnsafe is the most aggressive optimization.
// It uses unsafe string conversion to avoid ALL string allocations.
// WARNING: The returned DepthUpdate's string fields share memory with
// the input byte slice. The input MUST remain valid and unmodified
// while the DepthUpdate is in use.
func ParseFastJSONUnsafe(data []byte) (*DepthUpdate, error) {
	p := parserPool.Get().(*fastjson.Parser)
	defer parserPool.Put(p)

	v, err := p.ParseBytes(data)
	if err != nil {
		return nil, err
	}

	// Check if this is a combined stream wrapper
	if v.Exists("stream") && v.Exists("data") {
		v = v.Get("data")
	}

	update := GetDepthUpdate()
	update.Event = unsafeString(v.GetStringBytes("e"))
	update.EventTime = v.GetInt64("E")
	update.Symbol = unsafeString(v.GetStringBytes("s"))
	update.FirstUpdateID = v.GetInt64("U")
	update.FinalUpdateID = v.GetInt64("u")

	// Parse Bids - reuse existing slice capacity with unsafe strings
	bidsValue := v.Get("b")
	if bidsValue != nil {
		bidsArray, _ := bidsValue.Array()
		if cap(update.Bids) < len(bidsArray) {
			update.Bids = make([][2]string, len(bidsArray))
		} else {
			update.Bids = update.Bids[:len(bidsArray)]
		}
		for i, b := range bidsArray {
			pair, _ := b.Array()
			if len(pair) >= 2 {
				update.Bids[i][0] = unsafeString(pair[0].GetStringBytes())
				update.Bids[i][1] = unsafeString(pair[1].GetStringBytes())
			}
		}
	}

	// Parse Asks - reuse existing slice capacity with unsafe strings
	asksValue := v.Get("a")
	if asksValue != nil {
		asksArray, _ := asksValue.Array()
		if cap(update.Asks) < len(asksArray) {
			update.Asks = make([][2]string, len(asksArray))
		} else {
			update.Asks = update.Asks[:len(asksArray)]
		}
		for i, a := range asksArray {
			pair, _ := a.Array()
			if len(pair) >= 2 {
				update.Asks[i][0] = unsafeString(pair[0].GetStringBytes())
				update.Asks[i][1] = unsafeString(pair[1].GetStringBytes())
			}
		}
	}

	return update, nil
}

// ParseFastJSONZeroAlloc is the most optimized version.
// It uses pooled parser AND pooled DepthUpdate struct.
// IMPORTANT: Caller must call ReleaseDepthUpdate when done with the result.
func ParseFastJSONZeroAlloc(data []byte) (*DepthUpdate, error) {
	p := parserPool.Get().(*fastjson.Parser)
	defer parserPool.Put(p)

	v, err := p.ParseBytes(data)
	if err != nil {
		return nil, err
	}

	// Check if this is a combined stream wrapper
	if v.Exists("stream") && v.Exists("data") {
		v = v.Get("data")
	}

	update := GetDepthUpdate()
	update.Event = string(v.GetStringBytes("e"))
	update.EventTime = v.GetInt64("E")
	update.Symbol = string(v.GetStringBytes("s"))
	update.FirstUpdateID = v.GetInt64("U")
	update.FinalUpdateID = v.GetInt64("u")

	// Parse Bids - reuse existing slice capacity
	bidsValue := v.Get("b")
	if bidsValue != nil {
		bidsArray, _ := bidsValue.Array()
		// Ensure capacity, but reuse underlying array
		if cap(update.Bids) < len(bidsArray) {
			update.Bids = make([][2]string, len(bidsArray))
		} else {
			update.Bids = update.Bids[:len(bidsArray)]
		}
		for i, b := range bidsArray {
			pair, _ := b.Array()
			if len(pair) >= 2 {
				update.Bids[i][0] = string(pair[0].GetStringBytes())
				update.Bids[i][1] = string(pair[1].GetStringBytes())
			}
		}
	}

	// Parse Asks - reuse existing slice capacity
	asksValue := v.Get("a")
	if asksValue != nil {
		asksArray, _ := asksValue.Array()
		if cap(update.Asks) < len(asksArray) {
			update.Asks = make([][2]string, len(asksArray))
		} else {
			update.Asks = update.Asks[:len(asksArray)]
		}
		for i, a := range asksArray {
			pair, _ := a.Array()
			if len(pair) >= 2 {
				update.Asks[i][0] = string(pair[0].GetStringBytes())
				update.Asks[i][1] = string(pair[1].GetStringBytes())
			}
		}
	}

	return update, nil
}
