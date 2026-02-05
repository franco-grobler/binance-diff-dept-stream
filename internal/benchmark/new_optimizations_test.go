package benchmark

import (
	"strconv"
	"sync"
	"testing"
	"unsafe"

	"github.com/franco-grobler/binance-diff-stream/internal/orderbook"
	"github.com/franco-grobler/binance-diff-stream/internal/parsing"
)

// ============================================================================
// OPTIMIZATION 1: WebSocket Buffer Pooling
// Status: Recommended in optimizations.md but NOT implemented in client.go
// ============================================================================

// newWsBufferPool simulates WebSocket read buffer pooling
var newWsBufferPool = sync.Pool{
	New: func() interface{} {
		buf := make([]byte, 0, 4096)
		return &buf
	},
}

// BenchmarkWebSocket_Before_NoPooling simulates reading without buffer pooling
func BenchmarkWebSocket_Before_NoPooling(b *testing.B) {
	sampleData := []byte(combinedStreamJSON)
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// Simulate WebSocket read that allocates new buffer each time
		buf := make([]byte, len(sampleData))
		copy(buf, sampleData)
		_ = buf
	}
}

// BenchmarkWebSocket_After_WithPooling simulates reading with buffer pooling
func BenchmarkWebSocket_After_WithPooling(b *testing.B) {
	sampleData := []byte(combinedStreamJSON)
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// Get buffer from pool
		bufPtr := newWsBufferPool.Get().(*[]byte)
		buf := (*bufPtr)[:0]

		// Copy data (simulates WebSocket read into pooled buffer)
		buf = append(buf, sampleData...)

		// Return to pool
		*bufPtr = buf
		newWsBufferPool.Put(bufPtr)
	}
}

// ============================================================================
// OPTIMIZATION 2: Symbol String Interning
// Status: Recommended in optimizations.md but NOT implemented
// ============================================================================

// newSymbolIntern provides a simple string intern pool for known symbols
var newSymbolIntern = map[string]string{
	"BTCUSDT": "BTCUSDT",
	"ETHUSDT": "ETHUSDT",
	"BNBUSDT": "BNBUSDT",
}

// internSymbolNew returns an interned string for known symbols
func internSymbolNew(s string) string {
	if interned, ok := newSymbolIntern[s]; ok {
		return interned
	}
	return s
}

// BenchmarkSymbol_Before_NoInterning measures symbol string allocation
func BenchmarkSymbol_Before_NoInterning(b *testing.B) {
	// Simulate parsing that creates new strings each time
	rawSymbol := []byte("BTCUSDT")
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// Standard string conversion (allocates)
		symbol := string(rawSymbol)
		_ = symbol
	}
}

// BenchmarkSymbol_After_WithInterning measures symbol interning
func BenchmarkSymbol_After_WithInterning(b *testing.B) {
	rawSymbol := []byte("BTCUSDT")
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// Convert and immediately intern
		symbol := string(rawSymbol)
		symbol = internSymbolNew(symbol)
		_ = symbol
	}
}

// BenchmarkSymbol_After_UnsafeInterning uses unsafe string + interning
func BenchmarkSymbol_After_UnsafeInterning(b *testing.B) {
	rawSymbol := []byte("BTCUSDT")
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// Use unsafe string to avoid initial allocation
		unsafeStr := unsafe.String(unsafe.SliceData(rawSymbol), len(rawSymbol))
		symbol := internSymbolNew(unsafeStr)
		_ = symbol
	}
}

// ============================================================================
// OPTIMIZATION 3: Custom Float Parser for Binance Decimal Format
// Status: Benchmarked in optimizations.md but NOT used in orderbook hot path
// ============================================================================

// parseDecimalFast is a specialized float parser for Binance's decimal format.
// It only handles simple decimal numbers like "50123.45" without scientific notation.
func parseDecimalFast(s string) (float64, bool) {
	if len(s) == 0 {
		return 0, false
	}

	var result float64
	var decimalPlace float64 = 0
	var isDecimal bool
	var negative bool
	start := 0

	// Handle negative sign
	if s[0] == '-' {
		negative = true
		start = 1
	}

	for i := start; i < len(s); i++ {
		c := s[i]
		if c == '.' {
			if isDecimal {
				return 0, false // Multiple decimal points
			}
			isDecimal = true
			decimalPlace = 0.1
			continue
		}
		if c < '0' || c > '9' {
			return 0, false // Invalid character
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

// BenchmarkFloatParse_Before_Strconv uses standard library
func BenchmarkFloatParse_Before_Strconv(b *testing.B) {
	prices := []string{"50123.45678900", "0.00123456", "99999.99999999", "1.00000000"}
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		for _, p := range prices {
			f, _ := strconv.ParseFloat(p, 64)
			_ = f
		}
	}
}

// BenchmarkFloatParse_After_CustomParser uses custom fast parser
func BenchmarkFloatParse_After_CustomParser(b *testing.B) {
	prices := []string{"50123.45678900", "0.00123456", "99999.99999999", "1.00000000"}
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		for _, p := range prices {
			f, _ := parseDecimalFast(p)
			_ = f
		}
	}
}

// ============================================================================
// OPTIMIZATION 4: Batch Channel Operations
// Status: NEW optimization - reduce channel overhead with batching
// ============================================================================

// BenchmarkChannel_Before_SingleItem measures single item channel sends
func BenchmarkChannel_Before_SingleItem(b *testing.B) {
	ch := make(chan int, 1000)
	done := make(chan struct{})

	// Consumer
	go func() {
		for range ch {
		}
		close(done)
	}()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		ch <- i
	}

	close(ch)
	<-done
}

// BenchmarkChannel_After_BatchedItems measures batched channel sends
func BenchmarkChannel_After_BatchedItems(b *testing.B) {
	const batchSize = 10
	ch := make(chan []int, 100)
	done := make(chan struct{})

	// Consumer
	go func() {
		for batch := range ch {
			for range batch {
			}
		}
		close(done)
	}()

	batch := make([]int, 0, batchSize)
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		batch = append(batch, i)
		if len(batch) >= batchSize {
			ch <- batch
			batch = make([]int, 0, batchSize)
		}
	}

	// Flush remaining
	if len(batch) > 0 {
		ch <- batch
	}

	close(ch)
	<-done
}

// ============================================================================
// OPTIMIZATION 5: OrderBook Update with Pre-parsed Floats (integrated)
// Status: UpdatePreParsed exists but custom parser not used
// ============================================================================

// BenchmarkOrderBook_Before_StandardParsing uses strconv.ParseFloat in Update
func BenchmarkOrderBook_Before_StandardParsing(b *testing.B) {
	ob := orderbook.NewOrderBook("BTCUSDT")
	bids := [][2]string{
		{"50000.00", "1.5"},
		{"49999.00", "2.0"},
		{"49998.00", "0.5"},
	}
	asks := [][2]string{
		{"50001.00", "1.0"},
		{"50002.00", "2.5"},
		{"50003.00", "0.75"},
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		ob.Update(bids, asks, int64(i))
	}
}

// BenchmarkOrderBook_After_PreParsedWithFastFloat uses pre-parsed levels with fast parser
func BenchmarkOrderBook_After_PreParsedWithFastFloat(b *testing.B) {
	ob := orderbook.NewOrderBook("BTCUSDT")

	// Pre-parse once using custom parser
	bidsRaw := [][2]string{
		{"50000.00", "1.5"},
		{"49999.00", "2.0"},
		{"49998.00", "0.5"},
	}
	asksRaw := [][2]string{
		{"50001.00", "1.0"},
		{"50002.00", "2.5"},
		{"50003.00", "0.75"},
	}

	bids := make([]orderbook.PreParsedLevel, len(bidsRaw))
	asks := make([]orderbook.PreParsedLevel, len(asksRaw))

	for i, raw := range bidsRaw {
		price, _ := parseDecimalFast(raw[0])
		bids[i] = orderbook.PreParsedLevel{
			Price:    price,
			Quantity: raw[1],
			IsZero:   raw[1] == "0" || raw[1] == "0.00000000",
		}
	}
	for i, raw := range asksRaw {
		price, _ := parseDecimalFast(raw[0])
		asks[i] = orderbook.PreParsedLevel{
			Price:    price,
			Quantity: raw[1],
			IsZero:   raw[1] == "0" || raw[1] == "0.00000000",
		}
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		ob.UpdatePreParsed(bids, asks, int64(i))
	}
}

// ============================================================================
// OPTIMIZATION 6: Parsing with Immediate OrderBook Update (Integrated Pipeline)
// Status: NEW - combines parsing and orderbook update to avoid intermediate structs
// ============================================================================

// integratedPipelinePool provides pooled resources for the integrated pipeline
var integratedPipelinePool = sync.Pool{
	New: func() interface{} {
		return &integratedPipelineBuffer{
			bids: make([]orderbook.PreParsedLevel, 0, 20),
			asks: make([]orderbook.PreParsedLevel, 0, 20),
		}
	},
}

type integratedPipelineBuffer struct {
	bids []orderbook.PreParsedLevel
	asks []orderbook.PreParsedLevel
}

// BenchmarkPipeline_Before_Separate measures parse then update separately
func BenchmarkPipeline_Before_Separate(b *testing.B) {
	ob := orderbook.NewOrderBook("BTCUSDT")
	data := []byte(combinedStreamJSON)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		update, err := parsing.ParseFastJSONPooled(data)
		if err != nil {
			b.Fatal(err)
		}
		ob.Update(update.Bids, update.Asks, update.FinalUpdateID)
	}
}

// BenchmarkPipeline_After_Integrated measures integrated parse+update
func BenchmarkPipeline_After_Integrated(b *testing.B) {
	ob := orderbook.NewOrderBook("BTCUSDT")
	data := []byte(combinedStreamJSON)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// Parse using zero-alloc version
		update, err := parsing.ParseFastJSONZeroAlloc(data)
		if err != nil {
			b.Fatal(err)
		}

		// Get pooled buffer for pre-parsed levels
		buf := integratedPipelinePool.Get().(*integratedPipelineBuffer)
		buf.bids = buf.bids[:0]
		buf.asks = buf.asks[:0]

		// Convert to pre-parsed levels using fast float parser
		for _, raw := range update.Bids {
			price, _ := parseDecimalFast(raw[0])
			buf.bids = append(buf.bids, orderbook.PreParsedLevel{
				Price:    price,
				Quantity: raw[1],
				IsZero:   isZeroQuantityFast(raw[1]),
			})
		}
		for _, raw := range update.Asks {
			price, _ := parseDecimalFast(raw[0])
			buf.asks = append(buf.asks, orderbook.PreParsedLevel{
				Price:    price,
				Quantity: raw[1],
				IsZero:   isZeroQuantityFast(raw[1]),
			})
		}

		// Update orderbook with pre-parsed data
		ob.UpdatePreParsed(buf.bids, buf.asks, update.FinalUpdateID)

		// Return buffer to pool
		integratedPipelinePool.Put(buf)

		// Release the DepthUpdate back to pool
		parsing.ReleaseDepthUpdate(update)
	}
}

// isZeroQuantityFast is a fast zero check (copied from orderbook for benchmark)
func isZeroQuantityFast(qty string) bool {
	for i := 0; i < len(qty); i++ {
		c := qty[i]
		if c == '0' || c == '.' {
			continue
		}
		return false
	}
	return true
}

// ============================================================================
// OPTIMIZATION 7: Output Formatting with Pre-sized Buffer
// Status: Buffer pool exists but size estimation can be improved
// ============================================================================

// TopLevelNew represents the output format
type TopLevelNew struct {
	Symbol string
	Ask    float64
	Bid    float64
	TS     int64
}

var outputBufferPool = sync.Pool{
	New: func() interface{} {
		// Sized for typical output: {"symbol":"BTCUSDT","ask":50000.00,"bid":49999.00,"ts":1234567890123}
		// Approximately 80-100 bytes
		buf := make([]byte, 0, 128)
		return &buf
	},
}

// BenchmarkOutput_Before_Sprintf uses fmt.Sprintf
func BenchmarkOutput_Before_Sprintf(b *testing.B) {
	top := TopLevelNew{Symbol: "BTCUSDT", Ask: 50000.50, Bid: 49999.25, TS: 1234567890123}
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		output := []byte(`{"symbol":"` + top.Symbol + `","ask":` +
			strconv.FormatFloat(top.Ask, 'f', 2, 64) + `,"bid":` +
			strconv.FormatFloat(top.Bid, 'f', 2, 64) + `,"ts":` +
			strconv.FormatInt(top.TS, 10) + "}\n")
		_ = output
	}
}

// BenchmarkOutput_After_PooledAppend uses pooled buffer with append
func BenchmarkOutput_After_PooledAppend(b *testing.B) {
	top := TopLevelNew{Symbol: "BTCUSDT", Ask: 50000.50, Bid: 49999.25, TS: 1234567890123}
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		bufPtr := outputBufferPool.Get().(*[]byte)
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
		outputBufferPool.Put(bufPtr)
	}
}

// ============================================================================
// COMPREHENSIVE BEFORE/AFTER COMPARISON
// ============================================================================

// BenchmarkFullSystem_Before_Baseline measures the full pipeline without optimizations
func BenchmarkFullSystem_Before_Baseline(b *testing.B) {
	data := []byte(combinedStreamJSON)
	ob := orderbook.NewOrderBook("BTCUSDT")

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// 1. Parse with standard library
		update, err := parsing.ParseStandard(data)
		if err != nil {
			b.Fatal(err)
		}

		// 2. Update orderbook (includes strconv.ParseFloat)
		ob.Update(update.Bids, update.Asks, update.FinalUpdateID)

		// 3. Format output with string concat
		if len(ob.Bids) > 0 && len(ob.Asks) > 0 {
			output := []byte(`{"symbol":"` + ob.Symbol + `","ask":` +
				strconv.FormatFloat(ob.Asks[0].Price, 'f', 2, 64) + `,"bid":` +
				strconv.FormatFloat(ob.Bids[0].Price, 'f', 2, 64) + `,"ts":` +
				strconv.FormatInt(update.EventTime, 10) + "}\n")
			_ = output
		}
	}
}

// BenchmarkFullSystem_After_AllOptimizations measures the full pipeline with all optimizations
func BenchmarkFullSystem_After_AllOptimizations(b *testing.B) {
	data := []byte(combinedStreamJSON)
	ob := orderbook.NewOrderBook("BTCUSDT")

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// 1. Parse with zero-alloc pooled parser
		update, err := parsing.ParseFastJSONZeroAlloc(data)
		if err != nil {
			b.Fatal(err)
		}

		// 2. Get pooled buffer for pre-parsed levels
		buf := integratedPipelinePool.Get().(*integratedPipelineBuffer)
		buf.bids = buf.bids[:0]
		buf.asks = buf.asks[:0]

		// 3. Convert with fast float parser
		for _, raw := range update.Bids {
			price, _ := parseDecimalFast(raw[0])
			buf.bids = append(buf.bids, orderbook.PreParsedLevel{
				Price:    price,
				Quantity: raw[1],
				IsZero:   isZeroQuantityFast(raw[1]),
			})
		}
		for _, raw := range update.Asks {
			price, _ := parseDecimalFast(raw[0])
			buf.asks = append(buf.asks, orderbook.PreParsedLevel{
				Price:    price,
				Quantity: raw[1],
				IsZero:   isZeroQuantityFast(raw[1]),
			})
		}

		// 4. Update orderbook with pre-parsed data
		ob.UpdatePreParsed(buf.bids, buf.asks, update.FinalUpdateID)

		// 5. Format output with pooled buffer
		if len(ob.Bids) > 0 && len(ob.Asks) > 0 {
			outPtr := outputBufferPool.Get().(*[]byte)
			out := (*outPtr)[:0]

			out = append(out, `{"symbol":"`...)
			out = append(out, ob.Symbol...)
			out = append(out, `","ask":`...)
			out = strconv.AppendFloat(out, ob.Asks[0].Price, 'f', 2, 64)
			out = append(out, `,"bid":`...)
			out = strconv.AppendFloat(out, ob.Bids[0].Price, 'f', 2, 64)
			out = append(out, `,"ts":`...)
			out = strconv.AppendInt(out, update.EventTime, 10)
			out = append(out, "}\n"...)

			*outPtr = out
			outputBufferPool.Put(outPtr)
		}

		// 6. Return buffers to pools
		integratedPipelinePool.Put(buf)
		parsing.ReleaseDepthUpdate(update)
	}
}

// ============================================================================
// PARALLEL BENCHMARKS (simulates real concurrent workload)
// ============================================================================

// BenchmarkFullSystem_Before_Parallel measures parallel baseline
func BenchmarkFullSystem_Before_Parallel(b *testing.B) {
	data := []byte(combinedStreamJSON)

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		ob := orderbook.NewOrderBook("BTCUSDT")
		for pb.Next() {
			update, err := parsing.ParseStandard(data)
			if err != nil {
				b.Fatal(err)
			}
			ob.Update(update.Bids, update.Asks, update.FinalUpdateID)

			if len(ob.Bids) > 0 && len(ob.Asks) > 0 {
				output := []byte(`{"symbol":"` + ob.Symbol + `","ask":` +
					strconv.FormatFloat(ob.Asks[0].Price, 'f', 2, 64) + `,"bid":` +
					strconv.FormatFloat(ob.Bids[0].Price, 'f', 2, 64) + `,"ts":` +
					strconv.FormatInt(update.EventTime, 10) + "}\n")
				_ = output
			}
		}
	})
}

// BenchmarkFullSystem_After_Parallel measures parallel optimized
func BenchmarkFullSystem_After_Parallel(b *testing.B) {
	data := []byte(combinedStreamJSON)

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		ob := orderbook.NewOrderBook("BTCUSDT")
		for pb.Next() {
			update, err := parsing.ParseFastJSONZeroAlloc(data)
			if err != nil {
				b.Fatal(err)
			}

			buf := integratedPipelinePool.Get().(*integratedPipelineBuffer)
			buf.bids = buf.bids[:0]
			buf.asks = buf.asks[:0]

			for _, raw := range update.Bids {
				price, _ := parseDecimalFast(raw[0])
				buf.bids = append(buf.bids, orderbook.PreParsedLevel{
					Price:    price,
					Quantity: raw[1],
					IsZero:   isZeroQuantityFast(raw[1]),
				})
			}
			for _, raw := range update.Asks {
				price, _ := parseDecimalFast(raw[0])
				buf.asks = append(buf.asks, orderbook.PreParsedLevel{
					Price:    price,
					Quantity: raw[1],
					IsZero:   isZeroQuantityFast(raw[1]),
				})
			}

			ob.UpdatePreParsed(buf.bids, buf.asks, update.FinalUpdateID)

			if len(ob.Bids) > 0 && len(ob.Asks) > 0 {
				outPtr := outputBufferPool.Get().(*[]byte)
				out := (*outPtr)[:0]

				out = append(out, `{"symbol":"`...)
				out = append(out, ob.Symbol...)
				out = append(out, `","ask":`...)
				out = strconv.AppendFloat(out, ob.Asks[0].Price, 'f', 2, 64)
				out = append(out, `,"bid":`...)
				out = strconv.AppendFloat(out, ob.Bids[0].Price, 'f', 2, 64)
				out = append(out, `,"ts":`...)
				out = strconv.AppendInt(out, update.EventTime, 10)
				out = append(out, "}\n"...)

				*outPtr = out
				outputBufferPool.Put(outPtr)
			}

			integratedPipelinePool.Put(buf)
			parsing.ReleaseDepthUpdate(update)
		}
	})
}

// Test data (combined stream format from Binance)
const combinedStreamJSON = `{"stream":"btcusdt@depth","data":{"e":"depthUpdate","E":1672531200000,"s":"BTCUSDT","U":1000001,"u":1000010,"b":[["50000.00","1.50000000"],["49999.00","2.00000000"],["49998.00","0.50000000"],["49997.00","1.25000000"],["49996.00","0.75000000"]],"a":[["50001.00","1.00000000"],["50002.00","2.50000000"],["50003.00","0.75000000"],["50004.00","1.10000000"],["50005.00","0.90000000"]]}}`
