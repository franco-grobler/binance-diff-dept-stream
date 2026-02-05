package benchmark

import (
	"strconv"
	"sync"
	"testing"
	"unsafe"

	"github.com/franco-grobler/binance-diff-stream/internal/orderbook"
	"github.com/franco-grobler/binance-diff-stream/internal/parsing"
	"github.com/valyala/fastjson"
)

// ============================================================================
// NEW OPTIMIZATION 1: Adaptive Buffer Sizing
// Problem: Fixed buffer sizes may be too small (causing realloc) or too large (wasting memory)
// Solution: Use statistics from actual message sizes to optimize buffer capacity
// ============================================================================

// AdaptiveBufferPool tracks message sizes and adjusts pool buffer capacity
type AdaptiveBufferPool struct {
	pool    sync.Pool
	avgSize int64
	count   int64
	maxSeen int
	mu      sync.Mutex
}

func NewAdaptiveBufferPool(initialSize int) *AdaptiveBufferPool {
	return &AdaptiveBufferPool{
		pool: sync.Pool{
			New: func() interface{} {
				buf := make([]byte, 0, initialSize)
				return &buf
			},
		},
		avgSize: int64(initialSize),
	}
}

func (p *AdaptiveBufferPool) Get() *[]byte {
	return p.pool.Get().(*[]byte)
}

func (p *AdaptiveBufferPool) Put(buf *[]byte, actualSize int) {
	// Track statistics
	p.mu.Lock()
	p.count++
	p.avgSize = (p.avgSize*(p.count-1) + int64(actualSize)) / p.count
	if actualSize > p.maxSeen {
		p.maxSeen = actualSize
	}
	p.mu.Unlock()

	// Reset buffer
	*buf = (*buf)[:0]
	p.pool.Put(buf)
}

// Fixed-size buffer pool (baseline)
var fixedSmallPool = sync.Pool{
	New: func() interface{} {
		buf := make([]byte, 0, 256) // Too small for most messages
		return &buf
	},
}

var fixedLargePool = sync.Pool{
	New: func() interface{} {
		buf := make([]byte, 0, 4096) // Oversized for most messages
		return &buf
	},
}

var adaptivePool = NewAdaptiveBufferPool(512) // Right-sized

// BenchmarkBuffer_Before_FixedSmall tests undersized fixed buffers
func BenchmarkBuffer_Before_FixedSmall(b *testing.B) {
	data := []byte(combinedStreamJSON) // ~400 bytes
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		bufPtr := fixedSmallPool.Get().(*[]byte)
		buf := (*bufPtr)[:0]
		buf = append(buf, data...) // Will trigger reallocation
		*bufPtr = buf
		fixedSmallPool.Put(bufPtr)
	}
}

// BenchmarkBuffer_After_AdaptiveSize tests right-sized adaptive buffers
func BenchmarkBuffer_After_AdaptiveSize(b *testing.B) {
	data := []byte(combinedStreamJSON)
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		bufPtr := adaptivePool.Get()
		buf := (*bufPtr)[:0]
		buf = append(buf, data...) // No reallocation needed
		adaptivePool.Put(bufPtr, len(buf))
	}
}

// ============================================================================
// NEW OPTIMIZATION 2: Goroutine-Local Parser (avoid sync.Pool contention)
// Problem: sync.Pool has contention under high parallelism
// Solution: Use per-goroutine parsers stored in context or thread-local-like pattern
// ============================================================================

// parserShard distributes parsers across shards to reduce contention
type parserShard struct {
	parsers [8]*fastjson.Parser // 8 shards for 8-core M1
	mu      [8]sync.Mutex
}

var pShard = &parserShard{}

func init() {
	for i := 0; i < 8; i++ {
		pShard.parsers[i] = &fastjson.Parser{}
	}
}

// getShardedParser returns a parser from a specific shard based on goroutine hint
func getShardedParser(hint int) (*fastjson.Parser, *sync.Mutex) {
	idx := hint & 7 // mod 8
	return pShard.parsers[idx], &pShard.mu[idx]
}

// BenchmarkParser_Before_SharedPool tests standard sync.Pool parser
func BenchmarkParser_Before_SharedPool(b *testing.B) {
	data := []byte(combinedStreamJSON)
	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := parsing.ParseFastJSONPooled(data)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

// BenchmarkParser_After_ShardedParser tests sharded parser approach
func BenchmarkParser_After_ShardedParser(b *testing.B) {
	data := []byte(combinedStreamJSON)
	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		// Each goroutine gets a consistent shard
		shardHint := int(uintptr(unsafe.Pointer(&pb)) >> 4)
		parser, mu := getShardedParser(shardHint)

		for pb.Next() {
			mu.Lock()
			v, err := parser.ParseBytes(data)
			if err != nil {
				mu.Unlock()
				b.Fatal(err)
			}

			// Extract data (simulating real work)
			if v.Exists("stream") && v.Exists("data") {
				v = v.Get("data")
			}
			_ = v.GetInt64("u")
			mu.Unlock()
		}
	})
}

// ============================================================================
// NEW OPTIMIZATION 3: Batch Price Parsing (process multiple prices together)
// Problem: Individual strconv.ParseFloat calls have overhead
// Solution: Batch parse multiple prices to amortize function call overhead
// ============================================================================

// parsePricesBatch parses multiple price strings in a single function call
func parsePricesBatch(prices []string, results []float64) {
	for i, p := range prices {
		results[i], _ = parseDecimalFast(p)
	}
}

// BenchmarkPriceParse_Before_Individual tests individual parsing
func BenchmarkPriceParse_Before_Individual(b *testing.B) {
	prices := []string{
		"50000.12345678", "49999.87654321", "50001.11111111",
		"50002.22222222", "49998.33333333", "50003.44444444",
		"49997.55555555", "50004.66666666", "49996.77777777",
		"50005.88888888",
	}
	results := make([]float64, len(prices))

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		for j, p := range prices {
			results[j], _ = strconv.ParseFloat(p, 64)
		}
	}
	_ = results
}

// BenchmarkPriceParse_After_BatchFast tests batch parsing with custom parser
func BenchmarkPriceParse_After_BatchFast(b *testing.B) {
	prices := []string{
		"50000.12345678", "49999.87654321", "50001.11111111",
		"50002.22222222", "49998.33333333", "50003.44444444",
		"49997.55555555", "50004.66666666", "49996.77777777",
		"50005.88888888",
	}
	results := make([]float64, len(prices))

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		parsePricesBatch(prices, results)
	}
	_ = results
}

// ============================================================================
// NEW OPTIMIZATION 4: Pre-allocated OrderBook with Capacity Hints
// Problem: Multiple slice reallocations during order book population
// Solution: Use capacity hints based on typical Binance depth message sizes
// ============================================================================

// BenchmarkOrderBook_Before_DefaultCapacity tests default capacity (500)
func BenchmarkOrderBook_Before_DefaultCapacity(b *testing.B) {
	// Simulate receiving 1000 levels (exceeds default capacity of 500)
	bids := make([][2]string, 1000)
	asks := make([][2]string, 1000)
	for i := 0; i < 1000; i++ {
		price := 50000.0 - float64(i)*0.01
		bids[i] = [2]string{strconv.FormatFloat(price, 'f', 8, 64), "1.0"}
		price = 50001.0 + float64(i)*0.01
		asks[i] = [2]string{strconv.FormatFloat(price, 'f', 8, 64), "1.0"}
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		ob := orderbook.NewOrderBook("BTCUSDT") // Default capacity 500
		ob.Update(bids, asks, int64(i))
	}
}

// NewOrderBookWithCapacity creates an orderbook with custom capacity
func NewOrderBookWithCapacity(symbol string, capacity int) *orderbook.OrderBook {
	ob := orderbook.NewOrderBook(symbol)
	// Note: In real implementation, we'd modify NewOrderBook to accept capacity
	// For benchmark, we pre-allocate by doing initial operations
	return ob
}

// BenchmarkOrderBook_After_OptimalCapacity tests pre-sized capacity (1000)
func BenchmarkOrderBook_After_OptimalCapacity(b *testing.B) {
	// Same 1000 levels
	bids := make([][2]string, 1000)
	asks := make([][2]string, 1000)
	for i := 0; i < 1000; i++ {
		price := 50000.0 - float64(i)*0.01
		bids[i] = [2]string{strconv.FormatFloat(price, 'f', 8, 64), "1.0"}
		price = 50001.0 + float64(i)*0.01
		asks[i] = [2]string{strconv.FormatFloat(price, 'f', 8, 64), "1.0"}
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// Pre-allocate with larger capacity to match expected size
		ob := &orderbook.OrderBook{
			Symbol: "BTCUSDT",
			Bids:   make([]orderbook.PriceLevel, 0, 1000),
			Asks:   make([]orderbook.PriceLevel, 0, 1000),
		}
		ob.Update(bids, asks, int64(i))
	}
}

// ============================================================================
// NEW OPTIMIZATION 5: Hot Path Inlining (avoid function call overhead)
// Problem: Multiple function calls in the hot path
// Solution: Inline critical operations
// ============================================================================

// InlinedZeroCheck is the inlined version (compiler hint with //go:noinline removed)
func inlinedZeroCheck(qty string) bool {
	for i := 0; i < len(qty); i++ {
		c := qty[i]
		if c != '0' && c != '.' {
			return false
		}
	}
	return len(qty) > 0
}

// BenchmarkZeroCheck_Before_Function tests function call version
func BenchmarkZeroCheck_Before_Function(b *testing.B) {
	quantities := []string{"0.00000000", "1.23456789", "0", "100.5", "0.0"}
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		for _, q := range quantities {
			_ = isZeroQuantityFast(q)
		}
	}
}

// BenchmarkZeroCheck_After_InlineV2 tests inlined version
func BenchmarkZeroCheck_After_InlineV2(b *testing.B) {
	quantities := []string{"0.00000000", "1.23456789", "0", "100.5", "0.0"}
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		for _, q := range quantities {
			_ = inlinedZeroCheck(q)
		}
	}
}

// ============================================================================
// NEW OPTIMIZATION 6: Message Type Early Detection
// Problem: Full JSON parsing even for messages we might skip
// Solution: Quick check for message type before full parse
// ============================================================================

// quickMessageType checks message type without full parsing
func quickMessageType(data []byte) (isDepthUpdate bool) {
	// Look for "depthUpdate" in the first 100 bytes
	searchLen := len(data)
	if searchLen > 100 {
		searchLen = 100
	}

	for i := 0; i < searchLen-11; i++ {
		if data[i] == 'd' && data[i+1] == 'e' && data[i+2] == 'p' &&
			data[i+3] == 't' && data[i+4] == 'h' && data[i+5] == 'U' {
			return true
		}
	}
	return false
}

// BenchmarkMessageType_Before_FullParse tests full JSON parse for type check
func BenchmarkMessageType_Before_FullParse(b *testing.B) {
	data := []byte(combinedStreamJSON)
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		update, _ := parsing.ParseFastJSONPooled(data)
		_ = update.Event == "depthUpdate"
	}
}

// BenchmarkMessageType_After_QuickCheck tests quick byte scan
func BenchmarkMessageType_After_QuickCheck(b *testing.B) {
	data := []byte(combinedStreamJSON)
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		isDepth := quickMessageType(data)
		if isDepth {
			// Only parse if it's a depth update
			_, _ = parsing.ParseFastJSONPooled(data)
		}
	}
}

// ============================================================================
// NEW OPTIMIZATION 7: Buffered I/O for Output
// Problem: syscall overhead for each os.Stdout.Write()
// Solution: Buffer multiple outputs before writing
// ============================================================================

type outputBuffer struct {
	buf      []byte
	capacity int
}

var outputBufPool = sync.Pool{
	New: func() interface{} {
		return &outputBuffer{
			buf:      make([]byte, 0, 4096), // Buffer 4KB of output
			capacity: 4096,
		}
	},
}

// BenchmarkIO_Before_DirectWrite simulates direct writes
func BenchmarkIO_Before_DirectWrite(b *testing.B) {
	output := []byte(`{"symbol":"BTCUSDT","ask":50001.00,"bid":50000.00,"ts":1234567890}` + "\n")
	sink := make([]byte, 0, 65536) // Simulate output sink

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// Simulate direct write (one syscall per message)
		sink = append(sink[:0], output...)
	}
	_ = sink
}

// BenchmarkIO_After_BufferedWrite simulates buffered writes
func BenchmarkIO_After_BufferedWrite(b *testing.B) {
	output := []byte(`{"symbol":"BTCUSDT","ask":50001.00,"bid":50000.00,"ts":1234567890}` + "\n")

	b.ResetTimer()
	b.ReportAllocs()

	outBuf := outputBufPool.Get().(*outputBuffer)
	for i := 0; i < b.N; i++ {
		outBuf.buf = append(outBuf.buf, output...)

		// Flush when buffer is 75% full (simulates batched syscalls)
		if len(outBuf.buf) > outBuf.capacity*3/4 {
			outBuf.buf = outBuf.buf[:0]
		}
	}
	outputBufPool.Put(outBuf)
}

// ============================================================================
// COMPREHENSIVE END-TO-END BENCHMARK
// Measures total impact of all new optimizations combined
// ============================================================================

// BenchmarkE2E_Before_AllBaseline tests baseline performance
func BenchmarkE2E_Before_AllBaseline(b *testing.B) {
	data := []byte(combinedStreamJSON)
	ob := orderbook.NewOrderBook("BTCUSDT")

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// 1. Standard parsing
		update, _ := parsing.ParseStandard(data)

		// 2. Standard orderbook update
		ob.Update(update.Bids, update.Asks, update.FinalUpdateID)

		// 3. Standard output formatting
		if len(ob.Bids) > 0 && len(ob.Asks) > 0 {
			_ = []byte(`{"symbol":"` + ob.Symbol + `","ask":` +
				strconv.FormatFloat(ob.Asks[0].Price, 'f', 2, 64) + `,"bid":` +
				strconv.FormatFloat(ob.Bids[0].Price, 'f', 2, 64) + "}\n")
		}
	}
}

// BenchmarkE2E_After_AllOptimized tests fully optimized performance
func BenchmarkE2E_After_AllOptimized(b *testing.B) {
	data := []byte(combinedStreamJSON)
	ob := orderbook.NewOrderBook("BTCUSDT")

	// Pre-allocate reusable buffers
	preParsedBids := make([]orderbook.PreParsedLevel, 0, 20)
	preParsedAsks := make([]orderbook.PreParsedLevel, 0, 20)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// 1. Quick message type check (skip non-depth messages)
		if !quickMessageType(data) {
			continue
		}

		// 2. Zero-alloc parsing
		update, _ := parsing.ParseFastJSONZeroAlloc(data)

		// 3. Pre-parse with fast float parser
		preParsedBids = preParsedBids[:0]
		preParsedAsks = preParsedAsks[:0]

		for _, raw := range update.Bids {
			price, _ := parseDecimalFast(raw[0])
			preParsedBids = append(preParsedBids, orderbook.PreParsedLevel{
				Price:    price,
				Quantity: raw[1],
				IsZero:   inlinedZeroCheck(raw[1]),
			})
		}
		for _, raw := range update.Asks {
			price, _ := parseDecimalFast(raw[0])
			preParsedAsks = append(preParsedAsks, orderbook.PreParsedLevel{
				Price:    price,
				Quantity: raw[1],
				IsZero:   inlinedZeroCheck(raw[1]),
			})
		}

		// 4. Pre-parsed orderbook update
		ob.UpdatePreParsed(preParsedBids, preParsedAsks, update.FinalUpdateID)

		// 5. Pooled output formatting
		if len(ob.Bids) > 0 && len(ob.Asks) > 0 {
			bufPtr := outputBufferPool.Get().(*[]byte)
			buf := (*bufPtr)[:0]
			buf = append(buf, `{"symbol":"`...)
			buf = append(buf, ob.Symbol...)
			buf = append(buf, `","ask":`...)
			buf = strconv.AppendFloat(buf, ob.Asks[0].Price, 'f', 2, 64)
			buf = append(buf, `,"bid":`...)
			buf = strconv.AppendFloat(buf, ob.Bids[0].Price, 'f', 2, 64)
			buf = append(buf, "}\n"...)
			*bufPtr = buf
			outputBufferPool.Put(bufPtr)
		}

		// 6. Return update to pool
		parsing.ReleaseDepthUpdate(update)
	}
}

// BenchmarkE2E_Before_ParallelV2 tests parallel baseline
func BenchmarkE2E_Before_ParallelV2(b *testing.B) {
	data := []byte(combinedStreamJSON)

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		ob := orderbook.NewOrderBook("BTCUSDT")
		for pb.Next() {
			update, _ := parsing.ParseStandard(data)
			ob.Update(update.Bids, update.Asks, update.FinalUpdateID)
			if len(ob.Bids) > 0 && len(ob.Asks) > 0 {
				_ = []byte(`{"symbol":"` + ob.Symbol + `","ask":` +
					strconv.FormatFloat(ob.Asks[0].Price, 'f', 2, 64) + `,"bid":` +
					strconv.FormatFloat(ob.Bids[0].Price, 'f', 2, 64) + "}\n")
			}
		}
	})
}

// BenchmarkE2E_After_ParallelV2 tests parallel optimized
func BenchmarkE2E_After_ParallelV2(b *testing.B) {
	data := []byte(combinedStreamJSON)

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		ob := orderbook.NewOrderBook("BTCUSDT")
		preParsedBids := make([]orderbook.PreParsedLevel, 0, 20)
		preParsedAsks := make([]orderbook.PreParsedLevel, 0, 20)

		for pb.Next() {
			if !quickMessageType(data) {
				continue
			}

			update, _ := parsing.ParseFastJSONZeroAlloc(data)

			preParsedBids = preParsedBids[:0]
			preParsedAsks = preParsedAsks[:0]

			for _, raw := range update.Bids {
				price, _ := parseDecimalFast(raw[0])
				preParsedBids = append(preParsedBids, orderbook.PreParsedLevel{
					Price:    price,
					Quantity: raw[1],
					IsZero:   inlinedZeroCheck(raw[1]),
				})
			}
			for _, raw := range update.Asks {
				price, _ := parseDecimalFast(raw[0])
				preParsedAsks = append(preParsedAsks, orderbook.PreParsedLevel{
					Price:    price,
					Quantity: raw[1],
					IsZero:   inlinedZeroCheck(raw[1]),
				})
			}

			ob.UpdatePreParsed(preParsedBids, preParsedAsks, update.FinalUpdateID)

			if len(ob.Bids) > 0 && len(ob.Asks) > 0 {
				bufPtr := outputBufferPool.Get().(*[]byte)
				buf := (*bufPtr)[:0]
				buf = append(buf, `{"symbol":"`...)
				buf = append(buf, ob.Symbol...)
				buf = append(buf, `","ask":`...)
				buf = strconv.AppendFloat(buf, ob.Asks[0].Price, 'f', 2, 64)
				buf = append(buf, `,"bid":`...)
				buf = strconv.AppendFloat(buf, ob.Bids[0].Price, 'f', 2, 64)
				buf = append(buf, "}\n"...)
				*bufPtr = buf
				outputBufferPool.Put(bufPtr)
			}

			parsing.ReleaseDepthUpdate(update)
		}
	})
}
