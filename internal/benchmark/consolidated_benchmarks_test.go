package benchmark

import (
	"strconv"
	"sync"
	"testing"

	"github.com/franco-grobler/binance-diff-stream/internal/orderbook"
	"github.com/franco-grobler/binance-diff-stream/internal/parsing"
)

// ============================================================================
// CONSOLIDATED OPTIMIZATION BENCHMARKS
// Each benchmark clearly shows BEFORE and AFTER states for the README claims.
//
// Run with: go test -bench=BenchmarkConsolidated -benchmem ./internal/benchmark/
// ============================================================================

// Realistic Binance combined stream payload
var consolidatedPayload = []byte(`{"stream":"btcusdt@depth","data":{"e":"depthUpdate","E":1672531200000,"s":"BTCUSDT","U":1000001,"u":1000010,"b":[["50000.00","1.50000000"],["49999.00","2.00000000"],["49998.00","0.50000000"],["49997.00","1.25000000"],["49996.00","0.75000000"],["49995.00","0.00000000"],["49994.00","1.10000000"],["49993.00","2.30000000"],["49992.00","0.90000000"],["49991.00","1.60000000"]],"a":[["50001.00","1.00000000"],["50002.00","2.50000000"],["50003.00","0.75000000"],["50004.00","1.10000000"],["50005.00","0.90000000"],["50006.00","0.00000000"],["50007.00","1.40000000"],["50008.00","2.10000000"],["50009.00","0.80000000"],["50010.00","1.30000000"]]}}`)

// ============================================================================
// 1. JSON PARSING OPTIMIZATION
// README Claim: "Fast JSON: ~42% Faster with fastjson"
// ============================================================================

// BenchmarkConsolidated_JSON_1_StandardLib - BEFORE (encoding/json)
func BenchmarkConsolidated_JSON_1_StandardLib(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		update, err := parsing.ParseStandard(consolidatedPayload)
		if err != nil {
			b.Fatal(err)
		}
		_ = update.Symbol
	}
}

// BenchmarkConsolidated_JSON_2_FastJSON - AFTER Level 1 (valyala/fastjson)
func BenchmarkConsolidated_JSON_2_FastJSON(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		update, err := parsing.ParseFastJSON(consolidatedPayload)
		if err != nil {
			b.Fatal(err)
		}
		_ = update.Symbol
	}
}

// BenchmarkConsolidated_JSON_3_Pooled - AFTER Level 2 (sync.Pool for parser)
func BenchmarkConsolidated_JSON_3_Pooled(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		update, err := parsing.ParseFastJSONPooled(consolidatedPayload)
		if err != nil {
			b.Fatal(err)
		}
		_ = update.Symbol
	}
}

// BenchmarkConsolidated_JSON_4_ZeroAlloc - AFTER Level 3 (pooled struct)
func BenchmarkConsolidated_JSON_4_ZeroAlloc(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		update, err := parsing.ParseFastJSONZeroAlloc(consolidatedPayload)
		if err != nil {
			b.Fatal(err)
		}
		_ = update.Symbol
		parsing.ReleaseDepthUpdate(update)
	}
}

// BenchmarkConsolidated_JSON_5_Unsafe - AFTER Level 4 (unsafe string conversion)
func BenchmarkConsolidated_JSON_5_Unsafe(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		update, err := parsing.ParseFastJSONUnsafe(consolidatedPayload)
		if err != nil {
			b.Fatal(err)
		}
		_ = update.Symbol
		parsing.ReleaseDepthUpdate(update)
	}
}

// ============================================================================
// 2. WEBSOCKET BUFFER ALLOCATION
// README Claim: "Zero-Allocation Logic"
// ============================================================================

var wsBufferPoolConsolidated = sync.Pool{
	New: func() interface{} {
		buf := make([]byte, 0, 4096)
		return &buf
	},
}

// BenchmarkConsolidated_WSBuffer_Before - BEFORE (allocate each read)
func BenchmarkConsolidated_WSBuffer_Before(b *testing.B) {
	data := consolidatedPayload
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		// Current behavior: make() for each message
		buf := make([]byte, len(data))
		copy(buf, data)
		_ = buf
	}
}

// BenchmarkConsolidated_WSBuffer_After - AFTER (sync.Pool reuse)
func BenchmarkConsolidated_WSBuffer_After(b *testing.B) {
	data := consolidatedPayload
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		bufPtr := wsBufferPoolConsolidated.Get().(*[]byte)
		buf := (*bufPtr)[:0]
		buf = append(buf, data...)
		*bufPtr = buf
		wsBufferPoolConsolidated.Put(bufPtr)
	}
}

// ============================================================================
// 3. FLOAT PARSING OPTIMIZATION
// Used in orderbook price parsing
// ============================================================================

// fastParseFloatConsolidated parses Binance's decimal format efficiently
func fastParseFloatConsolidated(s string) (float64, bool) {
	if len(s) == 0 {
		return 0, false
	}
	var result float64
	var decimalPlace float64
	var isDecimal bool
	var negative bool
	start := 0

	if s[0] == '-' {
		negative = true
		start = 1
	}

	for i := start; i < len(s); i++ {
		c := s[i]
		if c == '.' {
			if isDecimal {
				return 0, false
			}
			isDecimal = true
			decimalPlace = 0.1
			continue
		}
		if c < '0' || c > '9' {
			return 0, false
		}
		digit := float64(c - '0')
		if isDecimal {
			result += digit * decimalPlace
			decimalPlace *= 0.1
		} else {
			result = result*10 + digit
		}
	}
	if negative {
		result = -result
	}
	return result, true
}

// BenchmarkConsolidated_Float_Before - BEFORE (strconv.ParseFloat)
func BenchmarkConsolidated_Float_Before(b *testing.B) {
	prices := []string{"50000.12345678", "49999.87654321", "0.00000001", "100000.00000000"}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		for _, p := range prices {
			f, _ := strconv.ParseFloat(p, 64)
			_ = f
		}
	}
}

// BenchmarkConsolidated_Float_After - AFTER (custom parser)
func BenchmarkConsolidated_Float_After(b *testing.B) {
	prices := []string{"50000.12345678", "49999.87654321", "0.00000001", "100000.00000000"}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		for _, p := range prices {
			f, _ := fastParseFloatConsolidated(p)
			_ = f
		}
	}
}

// ============================================================================
// 4. SLICE PRE-ALLOCATION
// README: "DefaultCapacity = 500" for order book slices
// ============================================================================

// BenchmarkConsolidated_Slice_Before - BEFORE (no pre-allocation)
func BenchmarkConsolidated_Slice_Before(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		slice := make([]orderbook.PriceLevel, 0)
		for j := 0; j < 500; j++ {
			slice = append(slice, orderbook.PriceLevel{Price: float64(j), Quantity: "1.0"})
		}
		_ = slice
	}
}

// BenchmarkConsolidated_Slice_After - AFTER (pre-allocated capacity)
func BenchmarkConsolidated_Slice_After(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		slice := make([]orderbook.PriceLevel, 0, 500)
		for j := 0; j < 500; j++ {
			slice = append(slice, orderbook.PriceLevel{Price: float64(j), Quantity: "1.0"})
		}
		_ = slice
	}
}

// ============================================================================
// 5. OUTPUT FORMATTING
// README: JSON output {"symbol":..., "ask":..., "bid":...}
// ============================================================================

type consolidatedTopLevel struct {
	Symbol string
	Ask    float64
	Bid    float64
	TS     int64
}

var outputPoolConsolidated = sync.Pool{
	New: func() interface{} {
		buf := make([]byte, 0, 128)
		return &buf
	},
}

// BenchmarkConsolidated_Output_Before - BEFORE (string concatenation)
func BenchmarkConsolidated_Output_Before(b *testing.B) {
	top := consolidatedTopLevel{Symbol: "BTCUSDT", Ask: 50001.50, Bid: 50000.25, TS: 1672531200000}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		out := `{"symbol":"` + top.Symbol + `","ask":` +
			strconv.FormatFloat(top.Ask, 'f', 2, 64) + `,"bid":` +
			strconv.FormatFloat(top.Bid, 'f', 2, 64) + `,"ts":` +
			strconv.FormatInt(top.TS, 10) + "}\n"
		_ = out
	}
}

// BenchmarkConsolidated_Output_After - AFTER (pooled buffer with strconv.Append*)
func BenchmarkConsolidated_Output_After(b *testing.B) {
	top := consolidatedTopLevel{Symbol: "BTCUSDT", Ask: 50001.50, Bid: 50000.25, TS: 1672531200000}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		bufPtr := outputPoolConsolidated.Get().(*[]byte)
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
		*bufPtr = buf
		outputPoolConsolidated.Put(bufPtr)
	}
}

// ============================================================================
// 6. BINARY SEARCH PERFORMANCE
// README: "Binary Search for price level lookups"
// ============================================================================

func createConsolidatedLevels(n int) []orderbook.PriceLevel {
	levels := make([]orderbook.PriceLevel, n)
	for i := 0; i < n; i++ {
		levels[i] = orderbook.PriceLevel{Price: 50000.0 - float64(i)*0.01, Quantity: "1.0"}
	}
	return levels
}

func linearFindConsolidated(levels []orderbook.PriceLevel, price float64) int {
	for i, l := range levels {
		if l.Price <= price {
			return i
		}
	}
	return len(levels)
}

// BenchmarkConsolidated_Search_100_Before - BEFORE (linear search O(n))
func BenchmarkConsolidated_Search_100_Before(b *testing.B) {
	levels := createConsolidatedLevels(100)
	price := 49950.0
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = linearFindConsolidated(levels, price)
	}
}

// BenchmarkConsolidated_Search_100_After - AFTER (binary search O(log n))
func BenchmarkConsolidated_Search_100_After(b *testing.B) {
	levels := createConsolidatedLevels(100)
	price := 49950.0
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = binarySearchBids(levels, price)
	}
}

// BenchmarkConsolidated_Search_500_Before - BEFORE (linear search O(n))
func BenchmarkConsolidated_Search_500_Before(b *testing.B) {
	levels := createConsolidatedLevels(500)
	price := 49750.0
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = linearFindConsolidated(levels, price)
	}
}

// BenchmarkConsolidated_Search_500_After - AFTER (binary search O(log n))
func BenchmarkConsolidated_Search_500_After(b *testing.B) {
	levels := createConsolidatedLevels(500)
	price := 49750.0
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = binarySearchBids(levels, price)
	}
}

// ============================================================================
// 7. ORDERBOOK CAPACITY OPTIMIZATION
// README: "DefaultCapacity = 500" to reduce slice reallocation
// ============================================================================

func genConsolidatedLevels(count int, base float64) [][2]string {
	levels := make([][2]string, count)
	for i := 0; i < count; i++ {
		levels[i] = [2]string{
			strconv.FormatFloat(base+float64(i)*0.01, 'f', 8, 64),
			"1.00000000",
		}
	}
	return levels
}

// BenchmarkConsolidated_OrderBook_Before - BEFORE (capacity 100)
func BenchmarkConsolidated_OrderBook_Before(b *testing.B) {
	bids := genConsolidatedLevels(300, 50000.0)
	asks := genConsolidatedLevels(300, 50300.0)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		ob := &orderbook.OrderBook{
			Symbol: "BTCUSDT",
			Bids:   make([]orderbook.PriceLevel, 0, 100), // Small capacity
			Asks:   make([]orderbook.PriceLevel, 0, 100),
		}
		ob.Update(bids, asks, int64(i))
	}
}

// BenchmarkConsolidated_OrderBook_After - AFTER (capacity 500)
func BenchmarkConsolidated_OrderBook_After(b *testing.B) {
	bids := genConsolidatedLevels(300, 50000.0)
	asks := genConsolidatedLevels(300, 50300.0)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		ob := orderbook.NewOrderBook("BTCUSDT") // Uses DefaultCapacity=500
		ob.Update(bids, asks, int64(i))
	}
}

// ============================================================================
// 8. FULL PIPELINE COMPARISON
// End-to-end comparison showing cumulative optimization impact
// ============================================================================

// BenchmarkConsolidated_Pipeline_Before - BEFORE (baseline implementation)
func BenchmarkConsolidated_Pipeline_Before(b *testing.B) {
	ob := orderbook.NewOrderBook("BTCUSDT")
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		// Standard JSON parsing (no pooling)
		update, err := parsing.ParseFastJSON(consolidatedPayload)
		if err != nil {
			b.Fatal(err)
		}

		// Standard orderbook update
		_ = ob.Update(update.Bids, update.Asks, update.FinalUpdateID)

		// Get top level
		var bestBid, bestAsk float64
		if len(ob.Bids) > 0 {
			bestBid = ob.Bids[0].Price
		}
		if len(ob.Asks) > 0 {
			bestAsk = ob.Asks[0].Price
		}

		// String concatenation for output
		out := `{"symbol":"` + ob.Symbol + `","ask":` +
			strconv.FormatFloat(bestAsk, 'f', 2, 64) + `,"bid":` +
			strconv.FormatFloat(bestBid, 'f', 2, 64) + `,"ts":` +
			strconv.FormatInt(update.EventTime, 10) + "}\n"
		_ = out
	}
}

// BenchmarkConsolidated_Pipeline_After - AFTER (all optimizations applied)
func BenchmarkConsolidated_Pipeline_After(b *testing.B) {
	ob := orderbook.NewOrderBook("BTCUSDT")
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		// Zero-alloc JSON parsing with pooled parser and struct
		update, err := parsing.ParseFastJSONZeroAlloc(consolidatedPayload)
		if err != nil {
			b.Fatal(err)
		}

		// Batch optimized orderbook update
		_ = ob.UpdateBatch(update.Bids, update.Asks, update.FinalUpdateID)

		// Get top level (O(1) - no optimization needed)
		var bestBid, bestAsk float64
		if len(ob.Bids) > 0 {
			bestBid = ob.Bids[0].Price
		}
		if len(ob.Asks) > 0 {
			bestAsk = ob.Asks[0].Price
		}

		// Pooled buffer with strconv.Append*
		bufPtr := outputPoolConsolidated.Get().(*[]byte)
		buf := (*bufPtr)[:0]
		buf = append(buf, `{"symbol":"`...)
		buf = append(buf, ob.Symbol...)
		buf = append(buf, `","ask":`...)
		buf = strconv.AppendFloat(buf, bestAsk, 'f', 2, 64)
		buf = append(buf, `,"bid":`...)
		buf = strconv.AppendFloat(buf, bestBid, 'f', 2, 64)
		buf = append(buf, `,"ts":`...)
		buf = strconv.AppendInt(buf, update.EventTime, 10)
		buf = append(buf, "}\n"...)
		*bufPtr = buf
		outputPoolConsolidated.Put(bufPtr)

		// Release parser struct back to pool
		parsing.ReleaseDepthUpdate(update)
	}
}

// BenchmarkConsolidated_Pipeline_UltraOptimized - AFTER (maximum optimization with unsafe)
func BenchmarkConsolidated_Pipeline_UltraOptimized(b *testing.B) {
	ob := orderbook.NewOrderBook("BTCUSDT")
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		// Ultra-optimized parsing with unsafe string conversion
		update, err := parsing.ParseFastJSONUnsafe(consolidatedPayload)
		if err != nil {
			b.Fatal(err)
		}

		// Batch optimized orderbook update
		_ = ob.UpdateBatch(update.Bids, update.Asks, update.FinalUpdateID)

		// Get top level
		var bestBid, bestAsk float64
		if len(ob.Bids) > 0 {
			bestBid = ob.Bids[0].Price
		}
		if len(ob.Asks) > 0 {
			bestAsk = ob.Asks[0].Price
		}

		// Pooled buffer output
		bufPtr := outputPoolConsolidated.Get().(*[]byte)
		buf := (*bufPtr)[:0]
		buf = append(buf, `{"symbol":"`...)
		buf = append(buf, ob.Symbol...)
		buf = append(buf, `","ask":`...)
		buf = strconv.AppendFloat(buf, bestAsk, 'f', 2, 64)
		buf = append(buf, `,"bid":`...)
		buf = strconv.AppendFloat(buf, bestBid, 'f', 2, 64)
		buf = append(buf, `,"ts":`...)
		buf = strconv.AppendInt(buf, update.EventTime, 10)
		buf = append(buf, "}\n"...)
		*bufPtr = buf
		outputPoolConsolidated.Put(bufPtr)

		parsing.ReleaseDepthUpdate(update)
	}
}

// ============================================================================
// 9. PARALLEL PERFORMANCE (Simulating Real-World Concurrency)
// ============================================================================

// BenchmarkConsolidated_Parallel_Before - BEFORE (parallel, no pooling)
func BenchmarkConsolidated_Parallel_Before(b *testing.B) {
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		ob := orderbook.NewOrderBook("BTCUSDT")
		for pb.Next() {
			update, _ := parsing.ParseFastJSON(consolidatedPayload)
			ob.Update(update.Bids, update.Asks, update.FinalUpdateID)

			var bestBid, bestAsk float64
			if len(ob.Bids) > 0 {
				bestBid = ob.Bids[0].Price
			}
			if len(ob.Asks) > 0 {
				bestAsk = ob.Asks[0].Price
			}
			out := `{"symbol":"` + ob.Symbol + `","ask":` +
				strconv.FormatFloat(bestAsk, 'f', 2, 64) + `,"bid":` +
				strconv.FormatFloat(bestBid, 'f', 2, 64) + `,"ts":` +
				strconv.FormatInt(update.EventTime, 10) + "}\n"
			_ = out
		}
	})
}

// BenchmarkConsolidated_Parallel_After - AFTER (parallel, fully optimized)
func BenchmarkConsolidated_Parallel_After(b *testing.B) {
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		ob := orderbook.NewOrderBook("BTCUSDT")
		for pb.Next() {
			update, _ := parsing.ParseFastJSONUnsafe(consolidatedPayload)
			ob.UpdateBatch(update.Bids, update.Asks, update.FinalUpdateID)

			var bestBid, bestAsk float64
			if len(ob.Bids) > 0 {
				bestBid = ob.Bids[0].Price
			}
			if len(ob.Asks) > 0 {
				bestAsk = ob.Asks[0].Price
			}
			bufPtr := outputPoolConsolidated.Get().(*[]byte)
			buf := (*bufPtr)[:0]
			buf = append(buf, `{"symbol":"`...)
			buf = append(buf, ob.Symbol...)
			buf = append(buf, `","ask":`...)
			buf = strconv.AppendFloat(buf, bestAsk, 'f', 2, 64)
			buf = append(buf, `,"bid":`...)
			buf = strconv.AppendFloat(buf, bestBid, 'f', 2, 64)
			buf = append(buf, `,"ts":`...)
			buf = strconv.AppendInt(buf, update.EventTime, 10)
			buf = append(buf, "}\n"...)
			*bufPtr = buf
			outputPoolConsolidated.Put(bufPtr)

			parsing.ReleaseDepthUpdate(update)
		}
	})
}

// ============================================================================
// 10. MEMORY ALLOCATION COMPARISON
// Demonstrates GC pressure reduction
// ============================================================================

// BenchmarkConsolidated_Allocs_Before - BEFORE (high allocation count)
func BenchmarkConsolidated_Allocs_Before(b *testing.B) {
	ob := orderbook.NewOrderBook("BTCUSDT")
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		update, _ := parsing.ParseFastJSON(consolidatedPayload)
		ob.Update(update.Bids, update.Asks, update.FinalUpdateID)
	}
}

// BenchmarkConsolidated_Allocs_After - AFTER (minimal allocations)
func BenchmarkConsolidated_Allocs_After(b *testing.B) {
	ob := orderbook.NewOrderBook("BTCUSDT")
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		update, _ := parsing.ParseFastJSONUnsafe(consolidatedPayload)
		ob.UpdateBatch(update.Bids, update.Asks, update.FinalUpdateID)
		parsing.ReleaseDepthUpdate(update)
	}
}
