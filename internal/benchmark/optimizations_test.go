package benchmark

import (
	"strconv"
	"sync"
	"testing"

	"github.com/franco-grobler/binance-diff-stream/internal/orderbook"
	"github.com/franco-grobler/binance-diff-stream/internal/parsing"
)

// ============================================================================
// OPTIMIZATION BENCHMARKS - BEFORE vs AFTER
// ============================================================================

// Test data - realistic Binance payload
var realisticPayload = []byte(`{
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
      ["49996.00", "0.5"],
      ["49995.00", "0.25"],
      ["49994.00", "1.75"],
      ["49993.00", "0.0"],
      ["49992.00", "2.5"],
      ["49991.00", "3.25"]
    ],
    "a": [
      ["50001.00", "1.0"],
      ["50002.00", "2.0"],
      ["50003.00", "1.5"],
      ["50004.00", "3.0"],
      ["50005.00", "0.5"],
      ["50006.00", "0.0"],
      ["50007.00", "1.25"],
      ["50008.00", "2.75"],
      ["50009.00", "0.75"],
      ["50010.00", "1.0"]
    ]
  }
}`)

// ============================================================================
// OPTIMIZATION 1: JSON Parsing Improvements
// ============================================================================

// BenchmarkParsing_Before tests the baseline FastJSON parser
func BenchmarkParsing_Before(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		update, err := parsing.ParseFastJSON(realisticPayload)
		if err != nil {
			b.Fatal(err)
		}
		_ = update.Symbol
	}
}

// BenchmarkParsing_After_Pooled tests the pooled FastJSON parser
func BenchmarkParsing_After_Pooled(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		update, err := parsing.ParseFastJSONPooled(realisticPayload)
		if err != nil {
			b.Fatal(err)
		}
		_ = update.Symbol
	}
}

// BenchmarkParsing_After_ZeroAlloc tests the zero-allocation parser
func BenchmarkParsing_After_ZeroAlloc(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		update, err := parsing.ParseFastJSONZeroAlloc(realisticPayload)
		if err != nil {
			b.Fatal(err)
		}
		_ = update.Symbol
		parsing.ReleaseDepthUpdate(update)
	}
}

// BenchmarkParsing_After_Unsafe tests the unsafe string conversion parser
func BenchmarkParsing_After_Unsafe(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		update, err := parsing.ParseFastJSONUnsafe(realisticPayload)
		if err != nil {
			b.Fatal(err)
		}
		_ = update.Symbol
		parsing.ReleaseDepthUpdate(update)
	}
}

// ============================================================================
// OPTIMIZATION 2: OrderBook Update Improvements
// ============================================================================

func generateLargePriceLevels(count int, basePrice float64) [][2]string {
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

// BenchmarkOrderBook_Before_Update tests the original Update function
func BenchmarkOrderBook_Before_Update(b *testing.B) {
	bids := generateLargePriceLevels(100, 50000.0)
	asks := generateLargePriceLevels(100, 50100.0)

	// Updates with mix of operations
	updateBids := [][2]string{
		{"50050.00000000", "2.0"}, // Update mid
		{"49999.00000000", "1.0"}, // Insert at end
		{"50025.00000000", "0.0"}, // Delete mid
		{"50075.00000000", "1.5"}, // Update
		{"50001.00000000", "0.0"}, // Delete
	}
	updateAsks := [][2]string{
		{"50150.00000000", "2.0"}, // Update mid
		{"50250.00000000", "1.0"}, // Insert at end
		{"50125.00000000", "0.0"}, // Delete mid
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ob := orderbook.NewOrderBook("BTCUSDT")
		ob.Update(bids, asks, 0)
		ob.Update(updateBids, updateAsks, int64(i))
	}
}

// BenchmarkOrderBook_After_UpdateOptimized tests the optimized Update function
func BenchmarkOrderBook_After_UpdateOptimized(b *testing.B) {
	bids := generateLargePriceLevels(100, 50000.0)
	asks := generateLargePriceLevels(100, 50100.0)

	updateBids := [][2]string{
		{"50050.00000000", "2.0"},
		{"49999.00000000", "1.0"},
		{"50025.00000000", "0.0"},
		{"50075.00000000", "1.5"},
		{"50001.00000000", "0.0"},
	}
	updateAsks := [][2]string{
		{"50150.00000000", "2.0"},
		{"50250.00000000", "1.0"},
		{"50125.00000000", "0.0"},
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ob := orderbook.NewOrderBook("BTCUSDT")
		ob.Update(bids, asks, 0)
		ob.UpdateOptimized(updateBids, updateAsks, int64(i))
	}
}

// BenchmarkOrderBook_After_UpdateBatch tests the batch Update function
func BenchmarkOrderBook_After_UpdateBatch(b *testing.B) {
	bids := generateLargePriceLevels(100, 50000.0)
	asks := generateLargePriceLevels(100, 50100.0)

	updateBids := [][2]string{
		{"50050.00000000", "2.0"},
		{"49999.00000000", "1.0"},
		{"50025.00000000", "0.0"},
		{"50075.00000000", "1.5"},
		{"50001.00000000", "0.0"},
	}
	updateAsks := [][2]string{
		{"50150.00000000", "2.0"},
		{"50250.00000000", "1.0"},
		{"50125.00000000", "0.0"},
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ob := orderbook.NewOrderBook("BTCUSDT")
		ob.Update(bids, asks, 0)
		ob.UpdateBatch(updateBids, updateAsks, int64(i))
	}
}

// ============================================================================
// OPTIMIZATION 3: Deletion Operation Improvements
// ============================================================================

// BenchmarkOrderBook_Before_Deletion tests original deletion (using append)
func BenchmarkOrderBook_Before_Deletion(b *testing.B) {
	bids := generateLargePriceLevels(500, 50000.0)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ob := orderbook.NewOrderBook("BTCUSDT")
		ob.Update(bids, nil, 0)
		// Delete from middle
		deleteBids := [][2]string{
			{"50250.00000000", "0.0"},
		}
		ob.Update(deleteBids, nil, int64(i))
	}
}

// BenchmarkOrderBook_After_Deletion tests optimized deletion (using copy)
func BenchmarkOrderBook_After_Deletion(b *testing.B) {
	bids := generateLargePriceLevels(500, 50000.0)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ob := orderbook.NewOrderBook("BTCUSDT")
		ob.Update(bids, nil, 0)
		// Delete from middle
		deleteBids := [][2]string{
			{"50250.00000000", "0.0"},
		}
		ob.UpdateBatch(deleteBids, nil, int64(i))
	}
}

// ============================================================================
// OPTIMIZATION 4: Memory Trimming
// ============================================================================

// BenchmarkOrderBook_Before_NoTrimming tests without memory trimming
func BenchmarkOrderBook_Before_NoTrimming(b *testing.B) {
	// Simulate growing order book
	bids := generateLargePriceLevels(2000, 50000.0)
	asks := generateLargePriceLevels(2000, 52000.0)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ob := orderbook.NewOrderBook("BTCUSDT")
		ob.Update(bids, asks, int64(i))
		// No trimming - memory grows
	}
}

// BenchmarkOrderBook_After_WithTrimming tests with memory trimming
func BenchmarkOrderBook_After_WithTrimming(b *testing.B) {
	bids := generateLargePriceLevels(2000, 50000.0)
	asks := generateLargePriceLevels(2000, 52000.0)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ob := orderbook.NewOrderBook("BTCUSDT")
		ob.Update(bids, asks, int64(i))
		ob.Trim(1000) // Keep only top 1000 levels
	}
}

// ============================================================================
// OPTIMIZATION 5: Full Pipeline Comparison
// ============================================================================

// TopLevel for output formatting
type TopLevelOpt struct {
	Symbol string
	Ask    float64
	Bid    float64
	TS     int64
}

var bufPool = sync.Pool{
	New: func() interface{} {
		buf := make([]byte, 0, 128)
		return &buf
	},
}

func formatOutput(top TopLevelOpt) []byte {
	bufPtr := bufPool.Get().(*[]byte)
	buf := (*bufPtr)[:0]

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

func releaseOutputBuffer(buf []byte) {
	bufPool.Put(&buf)
}

// BenchmarkFullPipeline_Before tests the original pipeline
func BenchmarkFullPipeline_Before(b *testing.B) {
	ob := orderbook.NewOrderBook("BTCUSDT")

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		// Parse with non-pooled parser
		update, err := parsing.ParseFastJSON(realisticPayload)
		if err != nil {
			b.Fatal(err)
		}

		// Update orderbook
		_ = ob.Update(update.Bids, update.Asks, update.FinalUpdateID)

		// Get top level
		var bestBid, bestAsk float64
		if len(ob.Bids) > 0 {
			bestBid = ob.Bids[0].Price
		}
		if len(ob.Asks) > 0 {
			bestAsk = ob.Asks[0].Price
		}

		// Format output
		top := TopLevelOpt{
			Symbol: ob.Symbol,
			Ask:    bestAsk,
			Bid:    bestBid,
			TS:     update.EventTime,
		}
		_ = formatOutput(top)
	}
}

// BenchmarkFullPipeline_After tests the optimized pipeline
func BenchmarkFullPipeline_After(b *testing.B) {
	ob := orderbook.NewOrderBook("BTCUSDT")

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		// Parse with unsafe string conversion + pooled struct
		update, err := parsing.ParseFastJSONUnsafe(realisticPayload)
		if err != nil {
			b.Fatal(err)
		}

		// Update orderbook with batch optimization
		_ = ob.UpdateBatch(update.Bids, update.Asks, update.FinalUpdateID)

		// Get top level
		var bestBid, bestAsk float64
		if len(ob.Bids) > 0 {
			bestBid = ob.Bids[0].Price
		}
		if len(ob.Asks) > 0 {
			bestAsk = ob.Asks[0].Price
		}

		// Format output with pooled buffer
		top := TopLevelOpt{
			Symbol: ob.Symbol,
			Ask:    bestAsk,
			Bid:    bestBid,
			TS:     update.EventTime,
		}
		buf := formatOutput(top)
		releaseOutputBuffer(buf)

		// Release parser struct
		parsing.ReleaseDepthUpdate(update)
	}
}

// ============================================================================
// PARALLEL BENCHMARKS - Simulating Real-World Concurrency
// ============================================================================

// BenchmarkFullPipeline_Before_Parallel tests original pipeline under load
func BenchmarkFullPipeline_Before_Parallel(b *testing.B) {
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		ob := orderbook.NewOrderBook("BTCUSDT")
		for pb.Next() {
			update, err := parsing.ParseFastJSON(realisticPayload)
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
			_ = formatOutput(TopLevelOpt{
				Symbol: ob.Symbol,
				Ask:    bestAsk,
				Bid:    bestBid,
				TS:     update.EventTime,
			})
		}
	})
}

// BenchmarkFullPipeline_After_Parallel tests optimized pipeline under load
func BenchmarkFullPipeline_After_Parallel(b *testing.B) {
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		ob := orderbook.NewOrderBook("BTCUSDT")
		for pb.Next() {
			update, err := parsing.ParseFastJSONUnsafe(realisticPayload)
			if err != nil {
				b.Fatal(err)
			}
			_ = ob.UpdateBatch(update.Bids, update.Asks, update.FinalUpdateID)

			var bestBid, bestAsk float64
			if len(ob.Bids) > 0 {
				bestBid = ob.Bids[0].Price
			}
			if len(ob.Asks) > 0 {
				bestAsk = ob.Asks[0].Price
			}
			buf := formatOutput(TopLevelOpt{
				Symbol: ob.Symbol,
				Ask:    bestAsk,
				Bid:    bestBid,
				TS:     update.EventTime,
			})
			releaseOutputBuffer(buf)
			parsing.ReleaseDepthUpdate(update)
		}
	})
}

// ============================================================================
// MEMORY ALLOCATION COMPARISON
// ============================================================================

// BenchmarkAllocations_Before counts allocations in original path
func BenchmarkAllocations_Before(b *testing.B) {
	ob := orderbook.NewOrderBook("BTCUSDT")

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		update, _ := parsing.ParseFastJSON(realisticPayload)
		ob.Update(update.Bids, update.Asks, update.FinalUpdateID)
	}
}

// BenchmarkAllocations_After counts allocations in optimized path
func BenchmarkAllocations_After(b *testing.B) {
	ob := orderbook.NewOrderBook("BTCUSDT")

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		update, _ := parsing.ParseFastJSONUnsafe(realisticPayload)
		ob.UpdateBatch(update.Bids, update.Asks, update.FinalUpdateID)
		parsing.ReleaseDepthUpdate(update)
	}
}
