package benchmark

import (
	"strconv"
	"sync"
	"testing"

	"github.com/franco-grobler/binance-diff-stream/internal/orderbook"
	"github.com/franco-grobler/binance-diff-stream/internal/parsing"
)

// ============================================================================
// TEST DATA
// ============================================================================

// Combined stream payload (real-world format from Binance)
var combinedPayload = []byte(`{
  "stream": "btcusdt@depth",
  "data": {
    "e": "depthUpdate",
    "E": 1672531200000,
    "s": "BTCUSDT",
    "U": 157,
    "u": 160,
    "b": [
      ["50000.00", "1.5"],
      ["49999.00", "2.0"],
      ["49998.00", "3.0"],
      ["49997.00", "1.0"],
      ["49996.00", "0.5"]
    ],
    "a": [
      ["50001.00", "1.0"],
      ["50002.00", "2.0"],
      ["50003.00", "1.5"],
      ["50004.00", "3.0"],
      ["50005.00", "0.5"]
    ]
  }
}`)

// TopLevel represents the output format (copied from main.go for benchmarking)
type TopLevel struct {
	Symbol string
	Ask    float64
	Bid    float64
	TS     int64
}

// ============================================================================
// OPTIMIZATION 1: Parser Selection Benchmarks
// ============================================================================

// BenchmarkDispatcher_ParseFastJSON benchmarks the CURRENT implementation
// (uses ParseFastJSON - non-pooled)
func BenchmarkDispatcher_ParseFastJSON(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		update, err := parsing.ParseFastJSON(combinedPayload)
		if err != nil {
			b.Fatal(err)
		}
		// Simulate using the parsed data
		_ = update.Symbol
		_ = update.FinalUpdateID
	}
}

// BenchmarkDispatcher_ParseFastJSONPooled benchmarks OPTIMIZED implementation
// (uses ParseFastJSONPooled - pooled parser)
func BenchmarkDispatcher_ParseFastJSONPooled(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		update, err := parsing.ParseFastJSONPooled(combinedPayload)
		if err != nil {
			b.Fatal(err)
		}
		// Simulate using the parsed data
		_ = update.Symbol
		_ = update.FinalUpdateID
	}
}

// BenchmarkDispatcher_ParseFastJSONZeroAlloc benchmarks MOST OPTIMIZED implementation
// (uses ParseFastJSONZeroAlloc - pooled parser + pooled struct)
func BenchmarkDispatcher_ParseFastJSONZeroAlloc(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		update, err := parsing.ParseFastJSONZeroAlloc(combinedPayload)
		if err != nil {
			b.Fatal(err)
		}
		// Simulate using the parsed data
		_ = update.Symbol
		_ = update.FinalUpdateID
		// IMPORTANT: Release back to pool
		parsing.ReleaseDepthUpdate(update)
	}
}

// ============================================================================
// OPTIMIZATION 2: Printer Output Formatting Benchmarks
// ============================================================================

// BenchmarkPrinter_Sprintf benchmarks CURRENT implementation using fmt.Sprintf
func BenchmarkPrinter_Sprintf(b *testing.B) {
	top := TopLevel{
		Symbol: "BTCUSDT",
		Ask:    50001.00,
		Bid:    50000.00,
		TS:     1672531200000,
	}

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		// Current implementation: fmt.Printf (Sprintf for benchmark capture)
		_ = sprintfFormat(top)
	}
}

// BenchmarkPrinter_StringConcat benchmarks OPTIMIZED implementation using strconv
func BenchmarkPrinter_StringConcat(b *testing.B) {
	top := TopLevel{
		Symbol: "BTCUSDT",
		Ask:    50001.00,
		Bid:    50000.00,
		TS:     1672531200000,
	}

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = optimizedFormat(top)
	}
}

// BenchmarkPrinter_BufferPool benchmarks MOST OPTIMIZED with sync.Pool
func BenchmarkPrinter_BufferPool(b *testing.B) {
	top := TopLevel{
		Symbol: "BTCUSDT",
		Ask:    50001.00,
		Bid:    50000.00,
		TS:     1672531200000,
	}

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		buf := bufferPoolFormat(top)
		releaseBuffer(buf)
	}
}

// Printer format functions

func sprintfFormat(top TopLevel) string {
	return "{\"symbol\":\"" + top.Symbol + "\",\"ask\":" +
		strconv.FormatFloat(top.Ask, 'f', 2, 64) + ",\"bid\":" +
		strconv.FormatFloat(top.Bid, 'f', 2, 64) + ",\"ts\":" +
		strconv.FormatInt(top.TS, 10) + "}\n"
}

func optimizedFormat(top TopLevel) []byte {
	// Pre-allocate estimated size: ~70 bytes typical
	buf := make([]byte, 0, 80)
	buf = append(buf, `{"symbol":"`...)
	buf = append(buf, top.Symbol...)
	buf = append(buf, `","ask":`...)
	buf = strconv.AppendFloat(buf, top.Ask, 'f', 2, 64)
	buf = append(buf, `,"bid":`...)
	buf = strconv.AppendFloat(buf, top.Bid, 'f', 2, 64)
	buf = append(buf, `,"ts":`...)
	buf = strconv.AppendInt(buf, top.TS, 10)
	buf = append(buf, "}\n"...)
	return buf
}

var bufferPool = sync.Pool{
	New: func() interface{} {
		// Pre-allocate buffer with typical message size
		buf := make([]byte, 0, 128)
		return &buf
	},
}

func bufferPoolFormat(top TopLevel) []byte {
	bufPtr := bufferPool.Get().(*[]byte)
	buf := (*bufPtr)[:0] // Reset length, keep capacity

	buf = append(buf, `{"symbol":"`...)
	buf = append(buf, top.Symbol...)
	buf = append(buf, `","ask":`...)
	buf = strconv.AppendFloat(buf, top.Ask, 'f', 2, 64)
	buf = append(buf, `,"bid":`...)
	buf = strconv.AppendFloat(buf, top.Bid, 'f', 2, 64)
	buf = append(buf, `,"ts":`...)
	buf = strconv.AppendInt(buf, top.TS, 10)
	buf = append(buf, "}\n"...)

	return buf
}

func releaseBuffer(buf []byte) {
	bufferPool.Put(&buf)
}

// ============================================================================
// OPTIMIZATION 3: OrderBook Pre-allocation Benchmarks
// ============================================================================

// generatePriceLevels creates test data for benchmarking
func generatePriceLevels(count int, basePrice float64) [][2]string {
	levels := make([][2]string, count)
	for i := 0; i < count; i++ {
		price := basePrice + float64(i)*0.01
		levels[i] = [2]string{
			strconv.FormatFloat(price, 'f', 8, 64),
			"1.00000000",
		}
	}
	return levels
}

// BenchmarkOrderBook_DefaultCapacity benchmarks CURRENT default capacity (100)
func BenchmarkOrderBook_DefaultCapacity(b *testing.B) {
	bids := generatePriceLevels(200, 100.0)
	asks := generatePriceLevels(200, 300.0)

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		ob := orderbook.NewOrderBook("BTCUSDT")
		_ = ob.Update(bids, asks, int64(i))
	}
}

// BenchmarkOrderBook_OptimizedCapacity benchmarks OPTIMIZED capacity (500)
func BenchmarkOrderBook_OptimizedCapacity(b *testing.B) {
	bids := generatePriceLevels(200, 100.0)
	asks := generatePriceLevels(200, 300.0)

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		ob := NewOrderBookOptimized("BTCUSDT")
		_ = ob.Update(bids, asks, int64(i))
	}
}

// OrderBookOptimized is a copy with higher initial capacity
type OrderBookOptimized struct {
	Symbol       string
	Bids         []orderbook.PriceLevel
	Asks         []orderbook.PriceLevel
	LastUpdateID int64
}

func NewOrderBookOptimized(symbol string) *OrderBookOptimized {
	return &OrderBookOptimized{
		Symbol: symbol,
		Bids:   make([]orderbook.PriceLevel, 0, 500), // Increased from 100
		Asks:   make([]orderbook.PriceLevel, 0, 500), // Increased from 100
	}
}

func (ob *OrderBookOptimized) Update(bidsRaw, asksRaw [][2]string, u int64) error {
	ob.LastUpdateID = u

	for _, raw := range bidsRaw {
		price, err := strconv.ParseFloat(raw[0], 64)
		if err != nil {
			return err
		}
		ob.Bids = append(ob.Bids, orderbook.PriceLevel{Price: price, Quantity: raw[1]})
	}

	for _, raw := range asksRaw {
		price, err := strconv.ParseFloat(raw[0], 64)
		if err != nil {
			return err
		}
		ob.Asks = append(ob.Asks, orderbook.PriceLevel{Price: price, Quantity: raw[1]})
	}

	return nil
}

// ============================================================================
// OPTIMIZATION 4: Full Pipeline Benchmarks (End-to-End)
// ============================================================================

// BenchmarkFullPipeline_Current simulates the current full pipeline
func BenchmarkFullPipeline_Current(b *testing.B) {
	ob := orderbook.NewOrderBook("BTCUSDT")

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		// Step 1: Parse (current - non-pooled)
		update, err := parsing.ParseFastJSON(combinedPayload)
		if err != nil {
			b.Fatal(err)
		}

		// Step 2: Update OrderBook
		_ = ob.Update(update.Bids, update.Asks, update.FinalUpdateID)

		// Step 3: Format output (current - string concat)
		var bestBid, bestAsk float64
		if len(ob.Bids) > 0 {
			bestBid = ob.Bids[0].Price
		}
		if len(ob.Asks) > 0 {
			bestAsk = ob.Asks[0].Price
		}
		_ = sprintfFormat(TopLevel{
			Symbol: ob.Symbol,
			Ask:    bestAsk,
			Bid:    bestBid,
			TS:     update.EventTime,
		})
	}
}

// BenchmarkFullPipeline_Optimized simulates the optimized full pipeline
func BenchmarkFullPipeline_Optimized(b *testing.B) {
	ob := orderbook.NewOrderBook("BTCUSDT")

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		// Step 1: Parse (optimized - zero alloc with pool)
		update, err := parsing.ParseFastJSONZeroAlloc(combinedPayload)
		if err != nil {
			b.Fatal(err)
		}

		// Step 2: Update OrderBook
		_ = ob.Update(update.Bids, update.Asks, update.FinalUpdateID)

		// Step 3: Format output (optimized - buffer pool)
		var bestBid, bestAsk float64
		if len(ob.Bids) > 0 {
			bestBid = ob.Bids[0].Price
		}
		if len(ob.Asks) > 0 {
			bestAsk = ob.Asks[0].Price
		}
		buf := bufferPoolFormat(TopLevel{
			Symbol: ob.Symbol,
			Ask:    bestAsk,
			Bid:    bestBid,
			TS:     update.EventTime,
		})
		releaseBuffer(buf)

		// Release parser struct back to pool
		parsing.ReleaseDepthUpdate(update)
	}
}

// ============================================================================
// PARALLEL BENCHMARKS (Simulating Real Workload)
// ============================================================================

// BenchmarkFullPipeline_CurrentParallel simulates concurrent current pipeline
func BenchmarkFullPipeline_CurrentParallel(b *testing.B) {
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		ob := orderbook.NewOrderBook("BTCUSDT")
		for pb.Next() {
			update, err := parsing.ParseFastJSON(combinedPayload)
			if err != nil {
				b.Fatal(err)
			}
			_ = ob.Update(update.Bids, update.Asks, update.FinalUpdateID)

			var bestBid, bestAsk float64
			if len(ob.Bids) > 0 {
				bestBid = ob.Bids[0].Price
			}
			if len(ob.Asks) > 0 {
				bestAsk = ob.Asks[0].Price
			}
			_ = sprintfFormat(TopLevel{
				Symbol: ob.Symbol,
				Ask:    bestAsk,
				Bid:    bestBid,
				TS:     update.EventTime,
			})
		}
	})
}

// BenchmarkFullPipeline_OptimizedParallel simulates concurrent optimized pipeline
func BenchmarkFullPipeline_OptimizedParallel(b *testing.B) {
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		ob := orderbook.NewOrderBook("BTCUSDT")
		for pb.Next() {
			update, err := parsing.ParseFastJSONZeroAlloc(combinedPayload)
			if err != nil {
				b.Fatal(err)
			}
			_ = ob.Update(update.Bids, update.Asks, update.FinalUpdateID)

			var bestBid, bestAsk float64
			if len(ob.Bids) > 0 {
				bestBid = ob.Bids[0].Price
			}
			if len(ob.Asks) > 0 {
				bestAsk = ob.Asks[0].Price
			}
			buf := bufferPoolFormat(TopLevel{
				Symbol: ob.Symbol,
				Ask:    bestAsk,
				Bid:    bestBid,
				TS:     update.EventTime,
			})
			releaseBuffer(buf)
			parsing.ReleaseDepthUpdate(update)
		}
	})
}
