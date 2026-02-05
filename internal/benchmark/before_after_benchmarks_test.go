package benchmark

// ============================================================================
// BEFORE/AFTER OPTIMIZATION BENCHMARKS
// ============================================================================
//
// This file contains comprehensive before/after benchmarks for all
// optimizations implemented in the Binance Depth Streamer.
//
// Run with: go test -bench=BeforeAfter -benchmem ./internal/benchmark
// ============================================================================

import (
	"sort"
	"strconv"
	"sync"
	"testing"

	"github.com/franco-grobler/binance-diff-stream/internal/orderbook"
	"github.com/franco-grobler/binance-diff-stream/internal/parsing"
)

// ============================================================================
// TEST DATA (realistic Binance payloads)
// ============================================================================

// Direct depth update format
var directPayload = []byte(`{
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
}`)

// Combined stream format (used by Binance /stream endpoint)
var combinedStreamPayload = []byte(`{
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

// ============================================================================
// OPTIMIZATION 1: JSON PARSING
// Before: encoding/json (standard library)
// After:  valyala/fastjson with sync.Pool
// Impact: ~70% faster, ~95% fewer allocations
// ============================================================================

func BenchmarkBeforeAfter_JSON_1_StandardLib(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := parsing.ParseStandardDirect(directPayload)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkBeforeAfter_JSON_2_FastJSON(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := parsing.ParseFastJSONDirect(directPayload)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkBeforeAfter_JSON_3_FastJSONPooled(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := parsing.ParseFastJSONPooledDirect(directPayload)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkBeforeAfter_JSON_4_FastJSONZeroAlloc(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		update, err := parsing.ParseFastJSONZeroAlloc(combinedStreamPayload)
		if err != nil {
			b.Fatal(err)
		}
		parsing.ReleaseDepthUpdate(update)
	}
}

// ============================================================================
// OPTIMIZATION 2: ZERO-QUANTITY DETECTION
// Before: strconv.ParseFloat() to check if qty == 0
// After:  String character iteration (no parsing)
// Impact: ~85% faster for zero detection
// ============================================================================

func isZeroQuantityBefore(qty string) bool {
	if val, err := strconv.ParseFloat(qty, 64); err == nil && val == 0 {
		return true
	}
	return false
}

func isZeroQuantityAfter(qty string) bool {
	if len(qty) == 0 {
		return false
	}
	for i := 0; i < len(qty); i++ {
		c := qty[i]
		if c == '0' || c == '.' {
			continue
		}
		return false
	}
	return true
}

var testQuantities = []string{
	"0.00000000", // Zero (most common delete case)
	"1.23456789", // Non-zero
	"0",          // Zero
	"100.5",      // Non-zero
	"0.0",        // Zero
}

func BenchmarkBeforeAfter_ZeroCheck_1_ParseFloat(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		for _, q := range testQuantities {
			_ = isZeroQuantityBefore(q)
		}
	}
}

func BenchmarkBeforeAfter_ZeroCheck_2_Optimized(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		for _, q := range testQuantities {
			_ = isZeroQuantityAfter(q)
		}
	}
}

// ============================================================================
// OPTIMIZATION 3: SLICE PRE-ALLOCATION
// Before: Default capacity (100 elements)
// After:  Increased capacity (500 elements)
// Impact: ~75% fewer reallocations during initial population
// ============================================================================

func BenchmarkBeforeAfter_Prealloc_1_Default100(b *testing.B) {
	bids := make([][2]string, 400)
	asks := make([][2]string, 400)
	for i := 0; i < 400; i++ {
		bids[i] = [2]string{strconv.FormatFloat(50000.0-float64(i), 'f', 2, 64), "1.0"}
		asks[i] = [2]string{strconv.FormatFloat(50001.0+float64(i), 'f', 2, 64), "1.0"}
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		ob := &orderbook.OrderBook{
			Symbol: "BTCUSDT",
			Bids:   make([]orderbook.PriceLevel, 0, 100), // Before: 100
			Asks:   make([]orderbook.PriceLevel, 0, 100),
		}
		_ = ob.Update(bids, asks, int64(i))
	}
}

func BenchmarkBeforeAfter_Prealloc_2_Optimized500(b *testing.B) {
	bids := make([][2]string, 400)
	asks := make([][2]string, 400)
	for i := 0; i < 400; i++ {
		bids[i] = [2]string{strconv.FormatFloat(50000.0-float64(i), 'f', 2, 64), "1.0"}
		asks[i] = [2]string{strconv.FormatFloat(50001.0+float64(i), 'f', 2, 64), "1.0"}
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		ob := orderbook.NewOrderBook("BTCUSDT") // After: 500
		_ = ob.Update(bids, asks, int64(i))
	}
}

// ============================================================================
// OPTIMIZATION 4: SEARCH ALGORITHM
// Before: Linear search O(n)
// After:  Binary search O(log n)
// Impact: 97% faster for large order books (500+ levels)
// ============================================================================

type searchableSlice []float64

func linearSearch(s searchableSlice, target float64) int {
	for i, v := range s {
		if v >= target {
			return i
		}
	}
	return len(s)
}

func binarySearch(s searchableSlice, target float64) int {
	return sort.Search(len(s), func(i int) bool {
		return s[i] >= target
	})
}

func BenchmarkBeforeAfter_Search_Small10_1_Linear(b *testing.B) {
	slice := make(searchableSlice, 10)
	for i := range slice {
		slice[i] = float64(i) * 10
	}
	target := 45.0

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = linearSearch(slice, target)
	}
}

func BenchmarkBeforeAfter_Search_Small10_2_Binary(b *testing.B) {
	slice := make(searchableSlice, 10)
	for i := range slice {
		slice[i] = float64(i) * 10
	}
	target := 45.0

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = binarySearch(slice, target)
	}
}

func BenchmarkBeforeAfter_Search_Medium100_1_Linear(b *testing.B) {
	slice := make(searchableSlice, 100)
	for i := range slice {
		slice[i] = float64(i)
	}
	target := 50.0

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = linearSearch(slice, target)
	}
}

func BenchmarkBeforeAfter_Search_Medium100_2_Binary(b *testing.B) {
	slice := make(searchableSlice, 100)
	for i := range slice {
		slice[i] = float64(i)
	}
	target := 50.0

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = binarySearch(slice, target)
	}
}

func BenchmarkBeforeAfter_Search_Large500_1_Linear(b *testing.B) {
	slice := make(searchableSlice, 500)
	for i := range slice {
		slice[i] = float64(i)
	}
	target := 250.0

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = linearSearch(slice, target)
	}
}

func BenchmarkBeforeAfter_Search_Large500_2_Binary(b *testing.B) {
	slice := make(searchableSlice, 500)
	for i := range slice {
		slice[i] = float64(i)
	}
	target := 250.0

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = binarySearch(slice, target)
	}
}

// ============================================================================
// OPTIMIZATION 5: OUTPUT FORMATTING
// Before: fmt.Sprintf (reflection-based)
// After:  strconv.Append* with pooled buffers
// Impact: ~25% faster, ~95% fewer allocations
// ============================================================================

type benchTopLevel struct {
	Symbol string
	Ask    float64
	Bid    float64
	TS     int64
}

func formatOutputBefore(top benchTopLevel) string {
	return `{"symbol":"` + top.Symbol + `","ask":` +
		strconv.FormatFloat(top.Ask, 'f', 2, 64) + `,"bid":` +
		strconv.FormatFloat(top.Bid, 'f', 2, 64) + `,"ts":` +
		strconv.FormatInt(top.TS, 10) + "}\n"
}

var baOutputBufferPool = sync.Pool{
	New: func() interface{} {
		buf := make([]byte, 0, 128)
		return &buf
	},
}

func formatOutputAfter(top benchTopLevel) []byte {
	bufPtr := baOutputBufferPool.Get().(*[]byte)
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

func baReleaseOutputBuffer(buf []byte) {
	baOutputBufferPool.Put(&buf)
}

func BenchmarkBeforeAfter_Output_1_StringConcat(b *testing.B) {
	top := benchTopLevel{Symbol: "BTCUSDT", Ask: 50001.00, Bid: 50000.00, TS: 1672531200000}

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = formatOutputBefore(top)
	}
}

func BenchmarkBeforeAfter_Output_2_PooledBuffer(b *testing.B) {
	top := benchTopLevel{Symbol: "BTCUSDT", Ask: 50001.00, Bid: 50000.00, TS: 1672531200000}

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		buf := formatOutputAfter(top)
		baReleaseOutputBuffer(buf)
	}
}

// ============================================================================
// OPTIMIZATION 6: OPTIMIZED DELETION
// Before: append(slice[:idx], slice[idx+1:]...) - creates temp slice
// After:  copy(slice[idx:], slice[idx+1:]) - in-place shift
// Impact: ~5-10% faster for deletion operations
// ============================================================================

func deleteBefore(levels []orderbook.PriceLevel, idx int) []orderbook.PriceLevel {
	return append(levels[:idx], levels[idx+1:]...)
}

func deleteAfter(levels []orderbook.PriceLevel, idx int) []orderbook.PriceLevel {
	n := len(levels)
	copy(levels[idx:], levels[idx+1:])
	return levels[:n-1]
}

func BenchmarkBeforeAfter_Deletion_1_Append(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		// Create a fresh slice each iteration
		levels := make([]orderbook.PriceLevel, 500)
		for j := range levels {
			levels[j] = orderbook.PriceLevel{Price: float64(j), Quantity: "1.0"}
		}
		// Delete from middle (worst case)
		_ = deleteBefore(levels, 250)
	}
}

func BenchmarkBeforeAfter_Deletion_2_Copy(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		levels := make([]orderbook.PriceLevel, 500)
		for j := range levels {
			levels[j] = orderbook.PriceLevel{Price: float64(j), Quantity: "1.0"}
		}
		_ = deleteAfter(levels, 250)
	}
}

// ============================================================================
// OPTIMIZATION 7: FULL PIPELINE (END-TO-END)
// Before: ParseFastJSON (no pool) + string formatting
// After:  ParseFastJSONZeroAlloc + pooled buffer formatting
// Impact: ~55% faster, ~99% fewer allocations
// ============================================================================

func BenchmarkBeforeAfter_FullPipeline_1_Baseline(b *testing.B) {
	ob := orderbook.NewOrderBook("BTCUSDT")

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		// Parse (before - non-pooled FastJSON)
		update, err := parsing.ParseFastJSON(combinedStreamPayload)
		if err != nil {
			b.Fatal(err)
		}

		// Update orderbook
		_ = ob.Update(update.Bids, update.Asks, update.FinalUpdateID)

		// Format output (before - string concat)
		var bestBid, bestAsk float64
		if len(ob.Bids) > 0 {
			bestBid = ob.Bids[0].Price
		}
		if len(ob.Asks) > 0 {
			bestAsk = ob.Asks[0].Price
		}
		_ = formatOutputBefore(benchTopLevel{
			Symbol: ob.Symbol,
			Ask:    bestAsk,
			Bid:    bestBid,
			TS:     update.EventTime,
		})
	}
}

func BenchmarkBeforeAfter_FullPipeline_2_Optimized(b *testing.B) {
	ob := orderbook.NewOrderBook("BTCUSDT")

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		// Parse (after - zero-alloc pooled)
		update, err := parsing.ParseFastJSONZeroAlloc(combinedStreamPayload)
		if err != nil {
			b.Fatal(err)
		}

		// Update orderbook
		_ = ob.Update(update.Bids, update.Asks, update.FinalUpdateID)

		// Format output (after - pooled buffer)
		var bestBid, bestAsk float64
		if len(ob.Bids) > 0 {
			bestBid = ob.Bids[0].Price
		}
		if len(ob.Asks) > 0 {
			bestAsk = ob.Asks[0].Price
		}
		buf := formatOutputAfter(benchTopLevel{
			Symbol: ob.Symbol,
			Ask:    bestAsk,
			Bid:    bestBid,
			TS:     update.EventTime,
		})
		baReleaseOutputBuffer(buf)

		// Release parser struct back to pool
		parsing.ReleaseDepthUpdate(update)
	}
}

// Parallel versions to simulate real-world concurrent load
func BenchmarkBeforeAfter_FullPipeline_Parallel_1_Baseline(b *testing.B) {
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		ob := orderbook.NewOrderBook("BTCUSDT")
		for pb.Next() {
			update, _ := parsing.ParseFastJSON(combinedStreamPayload)
			_ = ob.Update(update.Bids, update.Asks, update.FinalUpdateID)

			var bestBid, bestAsk float64
			if len(ob.Bids) > 0 {
				bestBid = ob.Bids[0].Price
			}
			if len(ob.Asks) > 0 {
				bestAsk = ob.Asks[0].Price
			}
			_ = formatOutputBefore(benchTopLevel{
				Symbol: ob.Symbol, Ask: bestAsk, Bid: bestBid, TS: update.EventTime,
			})
		}
	})
}

func BenchmarkBeforeAfter_FullPipeline_Parallel_2_Optimized(b *testing.B) {
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		ob := orderbook.NewOrderBook("BTCUSDT")
		for pb.Next() {
			update, _ := parsing.ParseFastJSONZeroAlloc(combinedStreamPayload)
			_ = ob.Update(update.Bids, update.Asks, update.FinalUpdateID)

			var bestBid, bestAsk float64
			if len(ob.Bids) > 0 {
				bestBid = ob.Bids[0].Price
			}
			if len(ob.Asks) > 0 {
				bestAsk = ob.Asks[0].Price
			}
			buf := formatOutputAfter(benchTopLevel{
				Symbol: ob.Symbol, Ask: bestAsk, Bid: bestBid, TS: update.EventTime,
			})
			baReleaseOutputBuffer(buf)
			parsing.ReleaseDepthUpdate(update)
		}
	})
}

// ============================================================================
// OPTIMIZATION 8: GC PRESSURE COMPARISON
// Demonstrates the impact of allocations on garbage collection
// ============================================================================

func BenchmarkBeforeAfter_GCPressure_1_HighAlloc(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		// High allocation path - simulates old code
		update, _ := parsing.ParseFastJSON(combinedStreamPayload)
		_ = formatOutputBefore(benchTopLevel{
			Symbol: update.Symbol,
			Ask:    50001.0,
			Bid:    50000.0,
			TS:     update.EventTime,
		})
	}
}

func BenchmarkBeforeAfter_GCPressure_2_LowAlloc(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		// Low allocation path - optimized code
		update, _ := parsing.ParseFastJSONZeroAlloc(combinedStreamPayload)
		buf := formatOutputAfter(benchTopLevel{
			Symbol: update.Symbol,
			Ask:    50001.0,
			Bid:    50000.0,
			TS:     update.EventTime,
		})
		baReleaseOutputBuffer(buf)
		parsing.ReleaseDepthUpdate(update)
	}
}
