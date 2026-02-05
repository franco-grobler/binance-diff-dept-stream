package benchmark

import (
	"sort"
	"strconv"
	"sync"
	"testing"

	"github.com/franco-grobler/binance-diff-stream/internal/orderbook"
	"github.com/franco-grobler/binance-diff-stream/internal/parsing"
)

// ============================================================================
// OPTIMIZATION 6: WebSocket Buffer Pooling
// The README mentions "Zero-Allocation Logic" but the WebSocket client
// allocates a new buffer for each message. This tests pooling.
// ============================================================================

var wsBufferPool = sync.Pool{
	New: func() any {
		// Typical Binance depth message is 500-2000 bytes
		buf := make([]byte, 0, 4096)
		return &buf
	},
}

// Simulates current WebSocket read behavior (allocates each time)
func simulateWSReadCurrent(data []byte) []byte {
	// Current: each read allocates a new slice
	buf := make([]byte, len(data))
	copy(buf, data)
	return buf
}

// Simulates optimized WebSocket read behavior (uses pool)
func simulateWSReadPooled() []byte {
	bufPtr := wsBufferPool.Get().(*[]byte)
	buf := (*bufPtr)[:0]
	return buf
}

func releaseWSBuffer(buf []byte) {
	wsBufferPool.Put(&buf)
}

// BenchmarkWebSocket_Before_NoPool simulates current WS buffer allocation
func BenchmarkWebSocket_Before_NoPool(b *testing.B) {
	testData := realisticPayload // ~500 bytes

	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		buf := simulateWSReadCurrent(testData)
		// Process...
		_ = buf
	}
}

// BenchmarkWebSocket_After_WithPool simulates pooled WS buffer allocation
func BenchmarkWebSocket_After_WithPool(b *testing.B) {
	testData := realisticPayload

	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		buf := simulateWSReadPooled()
		buf = append(buf, testData...)
		// Process...
		_ = buf
		releaseWSBuffer(buf)
	}
}

// ============================================================================
// OPTIMIZATION 7: Binary Search vs Linear Search for Small Slices
// The README mentions binary search, but for very small slices (<16 elements)
// linear search can be faster due to cache locality and branch prediction.
// ============================================================================

// Linear search implementation for comparison
func linearSearchBids(levels []orderbook.PriceLevel, price float64) int {
	for i, l := range levels {
		if l.Price <= price {
			return i
		}
	}
	return len(levels)
}

// Binary search (current implementation)
func binarySearchBids(levels []orderbook.PriceLevel, price float64) int {
	return sort.Search(len(levels), func(i int) bool {
		return levels[i].Price <= price
	})
}

// Adaptive search: linear for small, binary for large
func adaptiveSearchBids(levels []orderbook.PriceLevel, price float64) int {
	if len(levels) < 16 {
		return linearSearchBids(levels, price)
	}
	return binarySearchBids(levels, price)
}

func generateTestLevels(count int, basePrice float64) []orderbook.PriceLevel {
	levels := make([]orderbook.PriceLevel, count)
	for i := range count {
		levels[i] = orderbook.PriceLevel{
			Price:    basePrice - float64(i)*0.01, // Descending for bids
			Quantity: "1.0",
		}
	}
	return levels
}

// BenchmarkSearch_Small_Linear tests linear search with 10 elements
func BenchmarkSearch_Small_Linear(b *testing.B) {
	levels := generateTestLevels(10, 50000.0)
	searchPrice := 49995.0 // Middle-ish

	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		_ = linearSearchBids(levels, searchPrice)
	}
}

// BenchmarkSearch_Small_Binary tests binary search with 10 elements
func BenchmarkSearch_Small_Binary(b *testing.B) {
	levels := generateTestLevels(10, 50000.0)
	searchPrice := 49995.0

	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		_ = binarySearchBids(levels, searchPrice)
	}
}

// BenchmarkSearch_Medium_Linear tests linear search with 100 elements
func BenchmarkSearch_Medium_Linear(b *testing.B) {
	levels := generateTestLevels(100, 50000.0)
	searchPrice := 49950.0 // Middle

	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		_ = linearSearchBids(levels, searchPrice)
	}
}

// BenchmarkSearch_Medium_Binary tests binary search with 100 elements
func BenchmarkSearch_Medium_Binary(b *testing.B) {
	levels := generateTestLevels(100, 50000.0)
	searchPrice := 49950.0

	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		_ = binarySearchBids(levels, searchPrice)
	}
}

// BenchmarkSearch_Large_Linear tests linear search with 500 elements
func BenchmarkSearch_Large_Linear(b *testing.B) {
	levels := generateTestLevels(500, 50000.0)
	searchPrice := 49750.0 // Middle

	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		_ = linearSearchBids(levels, searchPrice)
	}
}

// BenchmarkSearch_Large_Binary tests binary search with 500 elements
func BenchmarkSearch_Large_Binary(b *testing.B) {
	levels := generateTestLevels(500, 50000.0)
	searchPrice := 49750.0

	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		_ = binarySearchBids(levels, searchPrice)
	}
}

// BenchmarkSearch_Large_Adaptive tests adaptive search with 500 elements
func BenchmarkSearch_Large_Adaptive(b *testing.B) {
	levels := generateTestLevels(500, 50000.0)
	searchPrice := 49750.0

	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		_ = adaptiveSearchBids(levels, searchPrice)
	}
}

// ============================================================================
// OPTIMIZATION 8: strconv.ParseFloat vs Custom Float Parser
// Binance sends prices with predictable format (e.g., "50000.12345678")
// A custom parser could be faster for this specific format.
// ============================================================================

// customParseFloat parses a simple decimal float string without exponents
// This is faster for Binance's consistent price format
func customParseFloat(s string) (float64, error) {
	if len(s) == 0 {
		return 0, strconv.ErrSyntax
	}

	var result float64
	var decimalFound bool
	var decimalPlaces float64 = 1

	negative := false
	start := 0
	switch s[0] {
	case '-':
		negative = true
		start = 1
	case '+':
		start = 1
	}

	for i := start; i < len(s); i++ {
		c := s[i]
		if c == '.' {
			if decimalFound {
				return 0, strconv.ErrSyntax
			}
			decimalFound = true
			continue
		}
		if c < '0' || c > '9' {
			return 0, strconv.ErrSyntax
		}
		digit := float64(c - '0')
		if decimalFound {
			decimalPlaces *= 10
			result = result + digit/decimalPlaces
		} else {
			result = result*10 + digit
		}
	}

	if negative {
		result = -result
	}
	return result, nil
}

// BenchmarkParseFloat_Standard tests standard library ParseFloat
func BenchmarkParseFloat_Standard(b *testing.B) {
	prices := []string{"50000.12345678", "49999.87654321", "0.00000001", "100000.0"}

	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		for _, p := range prices {
			_, _ = strconv.ParseFloat(p, 64)
		}
	}
}

// BenchmarkParseFloat_Custom tests custom float parser
func BenchmarkParseFloat_Custom(b *testing.B) {
	prices := []string{"50000.12345678", "49999.87654321", "0.00000001", "100000.0"}

	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		for _, p := range prices {
			_, _ = customParseFloat(p)
		}
	}
}

// ============================================================================
// OPTIMIZATION 9: Struct Field Ordering for Cache Efficiency
// Reordering struct fields to minimize padding can improve cache performance.
// ============================================================================

// Original struct layout (may have padding)
type PriceLevelOriginal struct {
	Price    float64 // 8 bytes
	Quantity string  // 16 bytes (string header)
}

// Optimized struct layout (same fields, potentially better alignment)
type PriceLevelOptimized struct {
	Price    float64 // 8 bytes
	Quantity string  // 16 bytes
	// No change needed - already optimal alignment
}

// BenchmarkStructLayout_Original tests original struct access patterns
func BenchmarkStructLayout_Original(b *testing.B) {
	levels := make([]PriceLevelOriginal, 1000)
	for i := range levels {
		levels[i] = PriceLevelOriginal{Price: float64(i), Quantity: "1.0"}
	}

	b.ReportAllocs()
	var sum float64
	for i := 0; b.Loop(); i++ {
		for j := range levels {
			sum += levels[j].Price
		}
	}
	_ = sum
}

// ============================================================================
// OPTIMIZATION 10: Slice Pre-growth Strategy
// Growing slices in larger chunks can reduce allocation overhead.
// ============================================================================

// BenchmarkSliceGrowth_OneByOne simulates current growth pattern
func BenchmarkSliceGrowth_OneByOne(b *testing.B) {
	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		slice := make([]orderbook.PriceLevel, 0)
		for j := range 500 {
			slice = append(slice, orderbook.PriceLevel{Price: float64(j)})
		}
	}
}

// BenchmarkSliceGrowth_PreAllocated simulates pre-allocated growth
func BenchmarkSliceGrowth_PreAllocated(b *testing.B) {
	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		slice := make([]orderbook.PriceLevel, 0, 500)
		for j := range 500 {
			slice = append(slice, orderbook.PriceLevel{Price: float64(j)})
		}
	}
}

// BenchmarkSliceGrowth_ChunkedGrowth simulates chunked growth strategy
func BenchmarkSliceGrowth_ChunkedGrowth(b *testing.B) {
	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		slice := make([]orderbook.PriceLevel, 0, 100) // Start smaller
		for j := range 500 {
			if len(slice) == cap(slice) {
				// Grow by 2x when full
				newSlice := make([]orderbook.PriceLevel, len(slice), cap(slice)*2)
				copy(newSlice, slice)
				slice = newSlice
			}
			slice = append(slice, orderbook.PriceLevel{Price: float64(j)})
		}
	}
}

// ============================================================================
// OPTIMIZATION 11: Avoiding String Allocation in Hot Path
// The README mentions "Zero-Allocation Logic" - test string interning
// ============================================================================

var symbolIntern = map[string]string{
	"BTCUSDT": "BTCUSDT",
	"ETHUSDT": "ETHUSDT",
	"BNBUSDT": "BNBUSDT",
}

// BenchmarkStringAlloc_NewString tests creating new strings
func BenchmarkStringAlloc_NewString(b *testing.B) {
	data := []byte("BTCUSDT")

	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		s := string(data)
		_ = s
	}
}

// BenchmarkStringAlloc_Interned tests interned strings
func BenchmarkStringAlloc_Interned(b *testing.B) {
	data := []byte("BTCUSDT")

	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		s := string(data)
		if interned, ok := symbolIntern[s]; ok {
			s = interned
		}
		_ = s
	}
}

// ============================================================================
// OPTIMIZATION 12: Complete Pipeline with All Optimizations
// ============================================================================

// BenchmarkCompletePipeline_Baseline tests the baseline implementation
func BenchmarkCompletePipeline_Baseline(b *testing.B) {
	ob := orderbook.NewOrderBook("BTCUSDT")

	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		// Parse with standard fastjson
		update, err := parsing.ParseFastJSON(realisticPayload)
		if err != nil {
			b.Fatal(err)
		}

		// Update orderbook with standard method
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
		buf := make([]byte, 0, 128)
		buf = append(buf, `{"symbol":"`...)
		buf = append(buf, ob.Symbol...)
		buf = append(buf, `","ask":`...)
		buf = strconv.AppendFloat(buf, bestAsk, 'f', 2, 64)
		buf = append(buf, `,"bid":`...)
		buf = strconv.AppendFloat(buf, bestBid, 'f', 2, 64)
		buf = append(buf, `,"ts":`...)
		buf = strconv.AppendInt(buf, update.EventTime, 10)
		buf = append(buf, "}\n"...)
		_ = buf
	}
}

// BenchmarkCompletePipeline_FullyOptimized tests all optimizations combined
func BenchmarkCompletePipeline_FullyOptimized(b *testing.B) {
	ob := orderbook.NewOrderBook("BTCUSDT")

	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		// Parse with unsafe zero-alloc parser
		update, err := parsing.ParseFastJSONUnsafe(realisticPayload)
		if err != nil {
			b.Fatal(err)
		}

		// Update orderbook with batch optimization
		_ = ob.UpdateBatch(update.Bids, update.Asks, update.FinalUpdateID)

		// Get top level (O(1) - no change needed)
		var bestBid, bestAsk float64
		if len(ob.Bids) > 0 {
			bestBid = ob.Bids[0].Price
		}
		if len(ob.Asks) > 0 {
			bestAsk = ob.Asks[0].Price
		}

		// Format output with pooled buffer
		bufPtr := bufPool.Get().(*[]byte)
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
		bufPool.Put(bufPtr)

		// Release parser struct
		parsing.ReleaseDepthUpdate(update)
	}
}

// ============================================================================
// OPTIMIZATION 13: Memory Pressure - Concurrent GC Impact
// ============================================================================

// BenchmarkGCPressure_HighAlloc tests GC pressure with many allocations
func BenchmarkGCPressure_HighAlloc(b *testing.B) {
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			// High allocation path
			update, _ := parsing.ParseFastJSON(realisticPayload)
			buf := make([]byte, 0, 128)
			buf = append(buf, update.Symbol...)
			_ = buf
		}
	})
}

// BenchmarkGCPressure_LowAlloc tests GC pressure with pooled allocations
func BenchmarkGCPressure_LowAlloc(b *testing.B) {
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			// Low allocation path
			update, _ := parsing.ParseFastJSONUnsafe(realisticPayload)
			bufPtr := bufPool.Get().(*[]byte)
			buf := (*bufPtr)[:0]
			buf = append(buf, update.Symbol...)
			*bufPtr = buf
			bufPool.Put(bufPtr)
			parsing.ReleaseDepthUpdate(update)
		}
	})
}
