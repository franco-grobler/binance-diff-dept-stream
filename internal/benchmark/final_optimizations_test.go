package benchmark

import (
	"sort"
	"strconv"
	"sync"
	"testing"
	"unsafe"

	"github.com/franco-grobler/binance-diff-stream/internal/orderbook"
	"github.com/franco-grobler/binance-diff-stream/internal/parsing"
)

// ============================================================================
// FINAL OPTIMIZATION BENCHMARKS - COMPREHENSIVE BEFORE/AFTER COMPARISON
// These benchmarks measure specific optimizations identified from matching
// todo.md tasks against README.md features.
// ============================================================================

// Realistic payload matching Binance depth stream format
var finalPayload = []byte(`{"stream":"btcusdt@depth","data":{"e":"depthUpdate","E":1672531200000,"s":"BTCUSDT","U":1000001,"u":1000010,"b":[["50000.00","1.50000000"],["49999.00","2.00000000"],["49998.00","0.50000000"],["49997.00","1.25000000"],["49996.00","0.75000000"],["49995.00","0.00000000"],["49994.00","1.10000000"],["49993.00","2.30000000"],["49992.00","0.90000000"],["49991.00","1.60000000"]],"a":[["50001.00","1.00000000"],["50002.00","2.50000000"],["50003.00","0.75000000"],["50004.00","1.10000000"],["50005.00","0.90000000"],["50006.00","0.00000000"],["50007.00","1.40000000"],["50008.00","2.10000000"],["50009.00","0.80000000"],["50010.00","1.30000000"]]}}`)

// ============================================================================
// OPTIMIZATION A: ADAPTIVE BINARY/LINEAR SEARCH
// README claims binary search, but for small slices linear can be faster
// ============================================================================

func adaptiveSearch(levels []orderbook.PriceLevel, price float64, desc bool) int {
	n := len(levels)
	// Use linear search for small slices (better cache locality)
	if n < 16 {
		for i, l := range levels {
			if desc {
				if l.Price <= price {
					return i
				}
			} else {
				if l.Price >= price {
					return i
				}
			}
		}
		return n
	}
	// Binary search for larger slices
	return sort.Search(n, func(i int) bool {
		if desc {
			return levels[i].Price <= price
		}
		return levels[i].Price >= price
	})
}

func standardBinarySearch(levels []orderbook.PriceLevel, price float64, desc bool) int {
	return sort.Search(len(levels), func(i int) bool {
		if desc {
			return levels[i].Price <= price
		}
		return levels[i].Price >= price
	})
}

// BenchmarkAdaptiveSearch_Before_Small tests binary search on 8 elements
func BenchmarkAdaptiveSearch_Before_Small(b *testing.B) {
	levels := make([]orderbook.PriceLevel, 8)
	for i := range levels {
		levels[i] = orderbook.PriceLevel{Price: 50000.0 - float64(i)*0.01, Quantity: "1.0"}
	}
	price := 49996.0

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = standardBinarySearch(levels, price, true)
	}
}

// BenchmarkAdaptiveSearch_After_Small tests adaptive search on 8 elements
func BenchmarkAdaptiveSearch_After_Small(b *testing.B) {
	levels := make([]orderbook.PriceLevel, 8)
	for i := range levels {
		levels[i] = orderbook.PriceLevel{Price: 50000.0 - float64(i)*0.01, Quantity: "1.0"}
	}
	price := 49996.0

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = adaptiveSearch(levels, price, true)
	}
}

// ============================================================================
// OPTIMIZATION B: FAST FLOAT PARSING FOR BINANCE FORMAT
// Binance uses consistent decimal format without scientific notation
// ============================================================================

// fastParseBinanceFloat parses Binance-specific decimal format
// Optimized for format like "50000.12345678"
func fastParseBinanceFloat(s string) (float64, bool) {
	if len(s) == 0 {
		return 0, false
	}

	var integer, fraction float64
	var divider float64 = 1
	var inFraction bool

	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '.' {
			if inFraction {
				return 0, false
			}
			inFraction = true
			continue
		}
		if c < '0' || c > '9' {
			return 0, false
		}
		digit := float64(c - '0')
		if inFraction {
			divider *= 10
			fraction = fraction*10 + digit
		} else {
			integer = integer*10 + digit
		}
	}

	return integer + fraction/divider, true
}

// BenchmarkFinal_FloatParse_Before_Strconv tests standard library ParseFloat
func BenchmarkFinal_FloatParse_Before_Strconv(b *testing.B) {
	prices := []string{
		"50000.12345678",
		"49999.87654321",
		"0.00000001",
		"100000.00000000",
		"12345.67890123",
	}

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		for _, p := range prices {
			f, _ := strconv.ParseFloat(p, 64)
			_ = f
		}
	}
}

// BenchmarkFinal_FloatParse_After_Custom tests optimized Binance-format parser
func BenchmarkFinal_FloatParse_After_Custom(b *testing.B) {
	prices := []string{
		"50000.12345678",
		"49999.87654321",
		"0.00000001",
		"100000.00000000",
		"12345.67890123",
	}

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		for _, p := range prices {
			f, _ := fastParseBinanceFloat(p)
			_ = f
		}
	}
}

// ============================================================================
// OPTIMIZATION C: ZERO-COPY QUANTITY CHECK
// README mentions "isZeroQuantity" - test inline vs function call
// ============================================================================

// isZeroQtyInline checks if quantity is zero (inlined version)
func isZeroQtyInline(qty string) bool {
	for i := 0; i < len(qty); i++ {
		c := qty[i]
		if c != '0' && c != '.' {
			return false
		}
	}
	return len(qty) > 0
}

// isZeroQtyParse checks by parsing float (slower)
func isZeroQtyParse(qty string) bool {
	f, err := strconv.ParseFloat(qty, 64)
	if err != nil {
		return false
	}
	return f == 0
}

// BenchmarkZeroCheck_Before_Parse tests float parsing for zero check
func BenchmarkZeroCheck_Before_Parse(b *testing.B) {
	quantities := []string{"0.00000000", "1.50000000", "0.0", "0", "2.00000000"}

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		for _, q := range quantities {
			_ = isZeroQtyParse(q)
		}
	}
}

// BenchmarkZeroCheck_After_Inline tests string scan for zero check
func BenchmarkZeroCheck_After_Inline(b *testing.B) {
	quantities := []string{"0.00000000", "1.50000000", "0.0", "0", "2.00000000"}

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		for _, q := range quantities {
			_ = isZeroQtyInline(q)
		}
	}
}

// ============================================================================
// OPTIMIZATION D: BUFFERED I/O FOR OUTPUT
// README mentions stdout printing - buffered vs unbuffered
// ============================================================================

var outputPool = sync.Pool{
	New: func() interface{} {
		buf := make([]byte, 0, 128)
		return &buf
	},
}

type OutputData struct {
	Symbol string
	Ask    float64
	Bid    float64
	TS     int64
}

// formatOutputUnbuffered creates output with new allocation each time
func formatOutputUnbuffered(d OutputData) []byte {
	buf := make([]byte, 0, 128)
	buf = append(buf, `{"symbol":"`...)
	buf = append(buf, d.Symbol...)
	buf = append(buf, `","ask":`...)
	buf = strconv.AppendFloat(buf, d.Ask, 'f', 2, 64)
	buf = append(buf, `,"bid":`...)
	buf = strconv.AppendFloat(buf, d.Bid, 'f', 2, 64)
	buf = append(buf, `,"ts":`...)
	buf = strconv.AppendInt(buf, d.TS, 10)
	buf = append(buf, "}\n"...)
	return buf
}

// formatOutputPooled creates output with pooled buffer
func formatOutputPooled(d OutputData) []byte {
	bufPtr := outputPool.Get().(*[]byte)
	buf := (*bufPtr)[:0]
	buf = append(buf, `{"symbol":"`...)
	buf = append(buf, d.Symbol...)
	buf = append(buf, `","ask":`...)
	buf = strconv.AppendFloat(buf, d.Ask, 'f', 2, 64)
	buf = append(buf, `,"bid":`...)
	buf = strconv.AppendFloat(buf, d.Bid, 'f', 2, 64)
	buf = append(buf, `,"ts":`...)
	buf = strconv.AppendInt(buf, d.TS, 10)
	buf = append(buf, "}\n"...)
	return buf
}

func releaseOutputBuf(buf []byte) {
	outputPool.Put(&buf)
}

// BenchmarkOutput_Before_Unbuffered tests output with fresh allocation
func BenchmarkOutput_Before_Unbuffered(b *testing.B) {
	data := OutputData{Symbol: "BTCUSDT", Ask: 50001.50, Bid: 50000.25, TS: 1672531200000}

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		buf := formatOutputUnbuffered(data)
		_ = buf
	}
}

// BenchmarkOutput_After_Pooled tests output with pooled buffer
func BenchmarkOutput_After_Pooled(b *testing.B) {
	data := OutputData{Symbol: "BTCUSDT", Ask: 50001.50, Bid: 50000.25, TS: 1672531200000}

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		buf := formatOutputPooled(data)
		releaseOutputBuf(buf)
	}
}

// ============================================================================
// OPTIMIZATION E: UNSAFE STRING CONVERSION
// Converting []byte to string without allocation using unsafe
// ============================================================================

// unsafeByteToString converts bytes to string without allocation
// WARNING: The resulting string must not outlive the byte slice
func unsafeByteToString(b []byte) string {
	return unsafe.String(unsafe.SliceData(b), len(b))
}

// BenchmarkStringConvert_Before_Safe tests safe string conversion
func BenchmarkStringConvert_Before_Safe(b *testing.B) {
	data := []byte("BTCUSDT")

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		s := string(data)
		_ = s
	}
}

// BenchmarkStringConvert_After_Unsafe tests unsafe string conversion
func BenchmarkStringConvert_After_Unsafe(b *testing.B) {
	data := []byte("BTCUSDT")

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		s := unsafeByteToString(data)
		_ = s
	}
}

// ============================================================================
// OPTIMIZATION F: ORDERBOOK CAPACITY MANAGEMENT
// README mentions DefaultCapacity=500 - test impact of pre-allocation
// ============================================================================

// BenchmarkCapacity_Before_Default tests default initial capacity
func BenchmarkCapacity_Before_Default(b *testing.B) {
	bids := make([][2]string, 500)
	asks := make([][2]string, 500)
	for i := 0; i < 500; i++ {
		bids[i] = [2]string{strconv.FormatFloat(50000.0-float64(i)*0.01, 'f', 8, 64), "1.0"}
		asks[i] = [2]string{strconv.FormatFloat(50000.0+float64(i)*0.01, 'f', 8, 64), "1.0"}
	}

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		// Simulate old behavior with smaller initial capacity
		ob := &orderbook.OrderBook{
			Symbol: "BTCUSDT",
			Bids:   make([]orderbook.PriceLevel, 0, 100), // Old default
			Asks:   make([]orderbook.PriceLevel, 0, 100),
		}
		ob.Update(bids, asks, int64(i))
	}
}

// BenchmarkCapacity_After_Optimized tests optimized initial capacity
func BenchmarkCapacity_After_Optimized(b *testing.B) {
	bids := make([][2]string, 500)
	asks := make([][2]string, 500)
	for i := 0; i < 500; i++ {
		bids[i] = [2]string{strconv.FormatFloat(50000.0-float64(i)*0.01, 'f', 8, 64), "1.0"}
		asks[i] = [2]string{strconv.FormatFloat(50000.0+float64(i)*0.01, 'f', 8, 64), "1.0"}
	}

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		ob := orderbook.NewOrderBook("BTCUSDT") // Uses DefaultCapacity=500
		ob.Update(bids, asks, int64(i))
	}
}

// ============================================================================
// OPTIMIZATION G: PARSER POOL REUSE
// README mentions "fastjson" - test parser pooling impact
// ============================================================================

// BenchmarkParserPool_Before_NoPool tests parsing without pooling
func BenchmarkParserPool_Before_NoPool(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		update, err := parsing.ParseFastJSON(finalPayload)
		if err != nil {
			b.Fatal(err)
		}
		_ = update.Symbol
	}
}

// BenchmarkParserPool_After_WithPool tests parsing with pooling
func BenchmarkParserPool_After_WithPool(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		update, err := parsing.ParseFastJSONPooled(finalPayload)
		if err != nil {
			b.Fatal(err)
		}
		_ = update.Symbol
	}
}

// ============================================================================
// OPTIMIZATION H: COMPLETE END-TO-END PIPELINE
// Measures the full path from JSON to output
// ============================================================================

// BenchmarkE2E_Before_Baseline tests original full pipeline
func BenchmarkE2E_Before_Baseline(b *testing.B) {
	ob := orderbook.NewOrderBook("BTCUSDT")

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		// 1. Parse JSON (non-pooled)
		update, err := parsing.ParseFastJSON(finalPayload)
		if err != nil {
			b.Fatal(err)
		}

		// 2. Update orderbook
		ob.Update(update.Bids, update.Asks, update.FinalUpdateID)

		// 3. Get top level
		var bestBid, bestAsk float64
		if len(ob.Bids) > 0 {
			bestBid = ob.Bids[0].Price
		}
		if len(ob.Asks) > 0 {
			bestAsk = ob.Asks[0].Price
		}

		// 4. Format output (unbuffered)
		buf := formatOutputUnbuffered(OutputData{
			Symbol: ob.Symbol,
			Ask:    bestAsk,
			Bid:    bestBid,
			TS:     update.EventTime,
		})
		_ = buf
	}
}

// BenchmarkE2E_After_Optimized tests fully optimized pipeline
func BenchmarkE2E_After_Optimized(b *testing.B) {
	ob := orderbook.NewOrderBook("BTCUSDT")

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		// 1. Parse JSON (pooled + unsafe strings)
		update, err := parsing.ParseFastJSONUnsafe(finalPayload)
		if err != nil {
			b.Fatal(err)
		}

		// 2. Update orderbook (batch optimized)
		ob.UpdateBatch(update.Bids, update.Asks, update.FinalUpdateID)

		// 3. Get top level (O(1) - same as before)
		var bestBid, bestAsk float64
		if len(ob.Bids) > 0 {
			bestBid = ob.Bids[0].Price
		}
		if len(ob.Asks) > 0 {
			bestAsk = ob.Asks[0].Price
		}

		// 4. Format output (pooled buffer)
		buf := formatOutputPooled(OutputData{
			Symbol: ob.Symbol,
			Ask:    bestAsk,
			Bid:    bestBid,
			TS:     update.EventTime,
		})
		releaseOutputBuf(buf)

		// 5. Release parsed update
		parsing.ReleaseDepthUpdate(update)
	}
}

// BenchmarkE2E_Before_Parallel tests baseline under concurrent load
func BenchmarkE2E_Before_Parallel(b *testing.B) {
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		ob := orderbook.NewOrderBook("BTCUSDT")
		for pb.Next() {
			update, _ := parsing.ParseFastJSON(finalPayload)
			ob.Update(update.Bids, update.Asks, update.FinalUpdateID)

			var bestBid, bestAsk float64
			if len(ob.Bids) > 0 {
				bestBid = ob.Bids[0].Price
			}
			if len(ob.Asks) > 0 {
				bestAsk = ob.Asks[0].Price
			}

			buf := formatOutputUnbuffered(OutputData{
				Symbol: ob.Symbol,
				Ask:    bestAsk,
				Bid:    bestBid,
				TS:     update.EventTime,
			})
			_ = buf
		}
	})
}

// BenchmarkE2E_After_Parallel tests optimized under concurrent load
func BenchmarkE2E_After_Parallel(b *testing.B) {
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		ob := orderbook.NewOrderBook("BTCUSDT")
		for pb.Next() {
			update, _ := parsing.ParseFastJSONUnsafe(finalPayload)
			ob.UpdateBatch(update.Bids, update.Asks, update.FinalUpdateID)

			var bestBid, bestAsk float64
			if len(ob.Bids) > 0 {
				bestBid = ob.Bids[0].Price
			}
			if len(ob.Asks) > 0 {
				bestAsk = ob.Asks[0].Price
			}

			buf := formatOutputPooled(OutputData{
				Symbol: ob.Symbol,
				Ask:    bestAsk,
				Bid:    bestBid,
				TS:     update.EventTime,
			})
			releaseOutputBuf(buf)
			parsing.ReleaseDepthUpdate(update)
		}
	})
}

// ============================================================================
// OPTIMIZATION I: MEMORY TRIMMING IMPACT
// README mentions unbounded memory growth as a limitation
// ============================================================================

// BenchmarkTrimming_Before_NoTrim tests without memory trimming
func BenchmarkTrimming_Before_NoTrim(b *testing.B) {
	bids := make([][2]string, 2000)
	asks := make([][2]string, 2000)
	for i := 0; i < 2000; i++ {
		bids[i] = [2]string{strconv.FormatFloat(50000.0-float64(i)*0.01, 'f', 8, 64), "1.0"}
		asks[i] = [2]string{strconv.FormatFloat(50000.0+float64(i)*0.01, 'f', 8, 64), "1.0"}
	}

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		ob := orderbook.NewOrderBook("BTCUSDT")
		ob.Update(bids, asks, int64(i))
		// Memory grows unbounded
	}
}

// BenchmarkTrimming_After_WithTrim tests with memory trimming
func BenchmarkTrimming_After_WithTrim(b *testing.B) {
	bids := make([][2]string, 2000)
	asks := make([][2]string, 2000)
	for i := 0; i < 2000; i++ {
		bids[i] = [2]string{strconv.FormatFloat(50000.0-float64(i)*0.01, 'f', 8, 64), "1.0"}
		asks[i] = [2]string{strconv.FormatFloat(50000.0+float64(i)*0.01, 'f', 8, 64), "1.0"}
	}

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		ob := orderbook.NewOrderBook("BTCUSDT")
		ob.Update(bids, asks, int64(i))
		ob.Trim(1000) // Limit to 1000 levels
	}
}

// ============================================================================
// OPTIMIZATION J: CHANNEL BUFFER SIZING
// README mentions sharded worker pattern with channels
// ============================================================================

// BenchmarkChannel_Before_SmallBuffer tests small channel buffer
func BenchmarkChannel_Before_SmallBuffer(b *testing.B) {
	ch := make(chan int, 10) // Small buffer
	done := make(chan struct{})

	go func() {
		for range ch {
		}
		close(done)
	}()

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		ch <- i
	}
	close(ch)
	<-done
}

// BenchmarkChannel_After_LargeBuffer tests larger channel buffer
func BenchmarkChannel_After_LargeBuffer(b *testing.B) {
	ch := make(chan int, 1000) // Larger buffer matching main.go
	done := make(chan struct{})

	go func() {
		for range ch {
		}
		close(done)
	}()

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		ch <- i
	}
	close(ch)
	<-done
}
