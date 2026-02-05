package benchmark

import (
	"encoding/json"
	"runtime"
	"sort"
	"strconv"
	"sync"
	"testing"

	"github.com/franco-grobler/binance-diff-stream/internal/orderbook"
	"github.com/franco-grobler/binance-diff-stream/internal/parsing"
)

// ============================================================================
// OPTIMIZATION COMPARISON BENCHMARKS
// This file provides clear before/after comparisons for all optimizations
// mentioned in README.md and implemented according to todo.md
//
// Run with: go test -bench=BenchmarkOpt -benchmem ./internal/benchmark/
// ============================================================================

// Realistic Binance combined stream payload
var optTestPayload = []byte(`{"stream":"btcusdt@depth","data":{"e":"depthUpdate","E":1672531200000,"s":"BTCUSDT","U":1000001,"u":1000010,"b":[["50000.00","1.50000000"],["49999.00","2.00000000"],["49998.00","0.50000000"],["49997.00","1.25000000"],["49996.00","0.75000000"],["49995.00","0.00000000"],["49994.00","1.10000000"],["49993.00","2.30000000"],["49992.00","0.90000000"],["49991.00","1.60000000"]],"a":[["50001.00","1.00000000"],["50002.00","2.50000000"],["50003.00","0.75000000"],["50004.00","1.10000000"],["50005.00","0.90000000"],["50006.00","0.00000000"],["50007.00","1.40000000"],["50008.00","2.10000000"],["50009.00","0.80000000"],["50010.00","1.30000000"]]}}`)

// ============================================================================
// OPTIMIZATION 1: JSON PARSING (README: ~42% Faster with fastjson)
// Before: encoding/json with reflection
// After: valyala/fastjson with pooling
// ============================================================================

// BenchmarkOpt1_JSON_A_StdLib - BEFORE: Standard library encoding/json
func BenchmarkOpt1_JSON_A_StdLib(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		update, err := parsing.ParseStandard(optTestPayload)
		if err != nil {
			b.Fatal(err)
		}
		_ = update.Symbol
	}
}

// BenchmarkOpt1_JSON_B_FastJSON - AFTER: valyala/fastjson
func BenchmarkOpt1_JSON_B_FastJSON(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		update, err := parsing.ParseFastJSON(optTestPayload)
		if err != nil {
			b.Fatal(err)
		}
		_ = update.Symbol
	}
}

// BenchmarkOpt1_JSON_C_FastJSONPooled - AFTER+: Pooled parser (3x faster)
func BenchmarkOpt1_JSON_C_FastJSONPooled(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		update, err := parsing.ParseFastJSONPooled(optTestPayload)
		if err != nil {
			b.Fatal(err)
		}
		_ = update.Symbol
	}
}

// BenchmarkOpt1_JSON_D_ZeroAlloc - AFTER++: Pooled parser + pooled struct
func BenchmarkOpt1_JSON_D_ZeroAlloc(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		update, err := parsing.ParseFastJSONZeroAlloc(optTestPayload)
		if err != nil {
			b.Fatal(err)
		}
		_ = update.Symbol
		parsing.ReleaseDepthUpdate(update)
	}
}

// ============================================================================
// OPTIMIZATION 2: SORTED SLICES vs LINKED LISTS (README: Cache locality)
// Before: Map-based or linked list storage
// After: Contiguous slices with binary search
// ============================================================================

type linkedNode struct {
	Price    float64
	Quantity string
	Next     *linkedNode
}

type linkedList struct {
	Head *linkedNode
}

func (ll *linkedList) Insert(price float64, qty string) {
	newNode := &linkedNode{Price: price, Quantity: qty}
	if ll.Head == nil || ll.Head.Price < price {
		newNode.Next = ll.Head
		ll.Head = newNode
		return
	}
	current := ll.Head
	for current.Next != nil && current.Next.Price >= price {
		current = current.Next
	}
	newNode.Next = current.Next
	current.Next = newNode
}

func (ll *linkedList) Find(price float64) *linkedNode {
	current := ll.Head
	for current != nil {
		if current.Price == price {
			return current
		}
		current = current.Next
	}
	return nil
}

// BenchmarkOpt2_DataStruct_A_LinkedList - BEFORE: Linked list with O(n) insert
func BenchmarkOpt2_DataStruct_A_LinkedList(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		ll := &linkedList{}
		for j := 0; j < 500; j++ {
			ll.Insert(50000.0-float64(j)*0.01, "1.0")
		}
		// Simulate lookup
		_ = ll.Find(49750.0)
	}
}

// BenchmarkOpt2_DataStruct_B_SortedSlice - AFTER: Sorted slice with binary search
func BenchmarkOpt2_DataStruct_B_SortedSlice(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		ob := orderbook.NewOrderBook("BTCUSDT")
		for j := 0; j < 500; j++ {
			price := 50000.0 - float64(j)*0.01
			priceStr := strconv.FormatFloat(price, 'f', 8, 64)
			ob.Update([][2]string{{priceStr, "1.0"}}, nil, int64(j))
		}
		// Simulate top-level read (O(1))
		if len(ob.Bids) > 0 {
			_ = ob.Bids[0].Price
		}
	}
}

// ============================================================================
// OPTIMIZATION 3: BINARY SEARCH vs LINEAR SEARCH
// Before: Linear O(n) scan
// After: Binary O(log n) search via sort.Search
// ============================================================================

func optLinearSearch(levels []orderbook.PriceLevel, price float64) int {
	for i, l := range levels {
		if l.Price <= price {
			return i
		}
	}
	return len(levels)
}

func optBinarySearch(levels []orderbook.PriceLevel, price float64) int {
	return sort.Search(len(levels), func(i int) bool {
		return levels[i].Price <= price
	})
}

func createTestLevels(n int) []orderbook.PriceLevel {
	levels := make([]orderbook.PriceLevel, n)
	for i := 0; i < n; i++ {
		levels[i] = orderbook.PriceLevel{Price: 50000.0 - float64(i)*0.01, Quantity: "1.0"}
	}
	return levels
}

// BenchmarkOpt3_Search_A_Linear_100 - BEFORE: Linear O(n) for 100 levels
func BenchmarkOpt3_Search_A_Linear_100(b *testing.B) {
	levels := createTestLevels(100)
	price := 49950.0 // Mid-range search
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = optLinearSearch(levels, price)
	}
}

// BenchmarkOpt3_Search_B_Binary_100 - AFTER: Binary O(log n) for 100 levels
func BenchmarkOpt3_Search_B_Binary_100(b *testing.B) {
	levels := createTestLevels(100)
	price := 49950.0
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = optBinarySearch(levels, price)
	}
}

// BenchmarkOpt3_Search_C_Linear_1000 - BEFORE: Linear O(n) for 1000 levels
func BenchmarkOpt3_Search_C_Linear_1000(b *testing.B) {
	levels := createTestLevels(1000)
	price := 49500.0 // Mid-range search
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = optLinearSearch(levels, price)
	}
}

// BenchmarkOpt3_Search_D_Binary_1000 - AFTER: Binary O(log n) for 1000 levels
func BenchmarkOpt3_Search_D_Binary_1000(b *testing.B) {
	levels := createTestLevels(1000)
	price := 49500.0
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = optBinarySearch(levels, price)
	}
}

// ============================================================================
// OPTIMIZATION 4: ZERO QUANTITY DETECTION (String vs ParseFloat)
// Before: strconv.ParseFloat to check for zero
// After: String iteration avoiding float conversion
// ============================================================================

func isZeroParseFloat(qty string) bool {
	f, err := strconv.ParseFloat(qty, 64)
	return err == nil && f == 0
}

func isZeroStringCheck(qty string) bool {
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

// BenchmarkOpt4_ZeroQty_A_ParseFloat - BEFORE: strconv.ParseFloat
func BenchmarkOpt4_ZeroQty_A_ParseFloat(b *testing.B) {
	quantities := []string{"0.00000000", "0", "0.0", "1.50000000", "0.00001"}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		for _, q := range quantities {
			_ = isZeroParseFloat(q)
		}
	}
}

// BenchmarkOpt4_ZeroQty_B_StringCheck - AFTER: String iteration
func BenchmarkOpt4_ZeroQty_B_StringCheck(b *testing.B) {
	quantities := []string{"0.00000000", "0", "0.0", "1.50000000", "0.00001"}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		for _, q := range quantities {
			_ = isZeroStringCheck(q)
		}
	}
}

// ============================================================================
// OPTIMIZATION 5: SLICE PRE-ALLOCATION (README: DefaultCapacity = 500)
// Before: No pre-allocation, grow from 0
// After: Pre-allocate 500 capacity
// ============================================================================

// BenchmarkOpt5_Capacity_A_NoPrealloc - BEFORE: Start with capacity 0
func BenchmarkOpt5_Capacity_A_NoPrealloc(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		slice := make([]orderbook.PriceLevel, 0)
		for j := 0; j < 500; j++ {
			slice = append(slice, orderbook.PriceLevel{Price: float64(j), Quantity: "1.0"})
		}
		_ = slice
	}
}

// BenchmarkOpt5_Capacity_B_Prealloc500 - AFTER: Pre-allocate 500
func BenchmarkOpt5_Capacity_B_Prealloc500(b *testing.B) {
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
// OPTIMIZATION 6: SYNC.POOL FOR PARSER REUSE (README: Zero-allocation)
// Before: New parser each call
// After: Pooled parser reuse
// ============================================================================

var parserPoolTest = sync.Pool{
	New: func() interface{} {
		// Simulate parser allocation
		return make([]byte, 4096)
	},
}

// BenchmarkOpt6_Pool_A_NoPool - BEFORE: Allocate each time
func BenchmarkOpt6_Pool_A_NoPool(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		buf := make([]byte, 4096)
		copy(buf, optTestPayload)
		_ = buf
	}
}

// BenchmarkOpt6_Pool_B_WithPool - AFTER: Reuse via sync.Pool
func BenchmarkOpt6_Pool_B_WithPool(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		buf := parserPoolTest.Get().([]byte)
		copy(buf, optTestPayload)
		parserPoolTest.Put(buf)
	}
}

// ============================================================================
// OPTIMIZATION 7: OUTPUT FORMATTING (fmt.Printf vs strconv.Append)
// Before: String concatenation
// After: Pooled buffer with strconv.Append* functions
// ============================================================================

type topLevelTest struct {
	Symbol string
	Ask    float64
	Bid    float64
	TS     int64
}

var outputPoolTest = sync.Pool{
	New: func() interface{} {
		buf := make([]byte, 0, 128)
		return &buf
	},
}

// BenchmarkOpt7_Output_A_StringConcat - BEFORE: String concatenation
func BenchmarkOpt7_Output_A_StringConcat(b *testing.B) {
	top := topLevelTest{Symbol: "BTCUSDT", Ask: 50001.50, Bid: 50000.25, TS: 1672531200000}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		out := `{"symbol":"` + top.Symbol + `","ask":` +
			strconv.FormatFloat(top.Ask, 'f', 2, 64) + `,"bid":` +
			strconv.FormatFloat(top.Bid, 'f', 2, 64) + `,"ts":` +
			strconv.FormatInt(top.TS, 10) + "}\n"
		_ = out
	}
}

// BenchmarkOpt7_Output_B_PooledAppend - AFTER: Pooled buffer + strconv.Append*
func BenchmarkOpt7_Output_B_PooledAppend(b *testing.B) {
	top := topLevelTest{Symbol: "BTCUSDT", Ask: 50001.50, Bid: 50000.25, TS: 1672531200000}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		bufPtr := outputPoolTest.Get().(*[]byte)
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
		outputPoolTest.Put(bufPtr)
	}
}

// ============================================================================
// OPTIMIZATION 8: SLICE DELETION (append vs copy)
// Before: append(slice[:i], slice[i+1:]...)
// After: copy(slice[i:], slice[i+1:]); slice = slice[:n-1]
// ============================================================================

// BenchmarkOpt8_Delete_A_Append - BEFORE: Using append for deletion
func BenchmarkOpt8_Delete_A_Append(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		slice := make([]orderbook.PriceLevel, 500)
		for j := 0; j < 500; j++ {
			slice[j] = orderbook.PriceLevel{Price: float64(j), Quantity: "1.0"}
		}
		// Delete element at middle
		idx := 250
		slice = append(slice[:idx], slice[idx+1:]...)
		_ = slice
	}
}

// BenchmarkOpt8_Delete_B_Copy - AFTER: Using copy for deletion
func BenchmarkOpt8_Delete_B_Copy(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		slice := make([]orderbook.PriceLevel, 500)
		for j := 0; j < 500; j++ {
			slice[j] = orderbook.PriceLevel{Price: float64(j), Quantity: "1.0"}
		}
		// Delete element at middle
		idx := 250
		copy(slice[idx:], slice[idx+1:])
		slice = slice[:len(slice)-1]
		_ = slice
	}
}

// ============================================================================
// OPTIMIZATION 9: CHANNEL-BASED SHARDING (README: Lock-free processing)
// Before: Single mutex-protected map
// After: Sharded channels (one goroutine per symbol)
// ============================================================================

type sharedOrderBook struct {
	mu    sync.RWMutex
	books map[string]*orderbook.OrderBook
}

func (s *sharedOrderBook) Update(symbol string, bids, asks [][2]string, u int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.books[symbol] == nil {
		s.books[symbol] = orderbook.NewOrderBook(symbol)
	}
	s.books[symbol].Update(bids, asks, u)
}

// BenchmarkOpt9_Concurrency_A_Mutex - BEFORE: Mutex-protected shared map
func BenchmarkOpt9_Concurrency_A_Mutex(b *testing.B) {
	shared := &sharedOrderBook{
		books: make(map[string]*orderbook.OrderBook),
	}
	symbols := []string{"BTCUSDT", "ETHUSDT", "BNBUSDT"}
	bids := [][2]string{{"50000.00", "1.0"}}
	asks := [][2]string{{"50001.00", "1.0"}}

	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			symbol := symbols[i%len(symbols)]
			shared.Update(symbol, bids, asks, int64(i))
			i++
		}
	})
}

// BenchmarkOpt9_Concurrency_B_Sharded - AFTER: Dedicated goroutine per symbol
func BenchmarkOpt9_Concurrency_B_Sharded(b *testing.B) {
	type updateMsg struct {
		bids [][2]string
		asks [][2]string
		u    int64
		done chan struct{}
	}

	symbols := []string{"BTCUSDT", "ETHUSDT", "BNBUSDT"}
	channels := make(map[string]chan updateMsg)

	// Start workers
	for _, s := range symbols {
		ch := make(chan updateMsg, 100)
		channels[s] = ch
		go func(symbol string, in chan updateMsg) {
			ob := orderbook.NewOrderBook(symbol)
			for msg := range in {
				ob.Update(msg.bids, msg.asks, msg.u)
				if msg.done != nil {
					close(msg.done)
				}
			}
		}(s, ch)
	}

	bids := [][2]string{{"50000.00", "1.0"}}
	asks := [][2]string{{"50001.00", "1.0"}}

	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			symbol := symbols[i%len(symbols)]
			done := make(chan struct{})
			channels[symbol] <- updateMsg{bids: bids, asks: asks, u: int64(i), done: done}
			<-done // Wait for completion to ensure work is done
			i++
		}
	})

	// Cleanup
	for _, ch := range channels {
		close(ch)
	}
}

// ============================================================================
// OPTIMIZATION 10: MEMORY TRIMMING (README: MaxLevels = 1000)
// Before: Unbounded growth
// After: Trim to MaxLevels periodically
// ============================================================================

// BenchmarkOpt10_Trim_A_NoTrim - BEFORE: No trimming, unbounded growth
func BenchmarkOpt10_Trim_A_NoTrim(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		ob := orderbook.NewOrderBook("BTCUSDT")
		// Simulate price moves causing book to grow
		for j := 0; j < 2000; j++ {
			price := strconv.FormatFloat(50000.0+float64(j)*0.01, 'f', 8, 64)
			ob.Update([][2]string{{price, "1.0"}}, nil, int64(j))
		}
		// Book now has 2000 levels
		_ = len(ob.Bids)
	}
}

// BenchmarkOpt10_Trim_B_WithTrim - AFTER: Trim to 1000 levels
func BenchmarkOpt10_Trim_B_WithTrim(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		ob := orderbook.NewOrderBook("BTCUSDT")
		for j := 0; j < 2000; j++ {
			price := strconv.FormatFloat(50000.0+float64(j)*0.01, 'f', 8, 64)
			ob.UpdateOptimized([][2]string{{price, "1.0"}}, nil, int64(j))
		}
		// Book is trimmed to 1000 levels
		_ = len(ob.Bids)
	}
}

// ============================================================================
// OPTIMIZATION 11: FULL PIPELINE COMPARISON
// End-to-end benchmark showing cumulative optimization impact
// ============================================================================

// BenchmarkOpt11_Pipeline_A_Baseline - BEFORE: No optimizations
func BenchmarkOpt11_Pipeline_A_Baseline(b *testing.B) {
	// Use encoding/json for parsing (baseline)
	var wrapper struct {
		Stream string          `json:"stream"`
		Data   json.RawMessage `json:"data"`
	}
	var update parsing.DepthUpdate

	ob := orderbook.NewOrderBook("BTCUSDT")
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		// Parse with standard lib
		json.Unmarshal(optTestPayload, &wrapper)
		json.Unmarshal(wrapper.Data, &update)

		// Update orderbook
		ob.Update(update.Bids, update.Asks, update.FinalUpdateID)

		// Get top level and format output
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
}

// BenchmarkOpt11_Pipeline_B_Optimized - AFTER: All optimizations applied
func BenchmarkOpt11_Pipeline_B_Optimized(b *testing.B) {
	ob := orderbook.NewOrderBook("BTCUSDT")
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		// Parse with pooled fastjson + zero-alloc struct
		update, _ := parsing.ParseFastJSONZeroAlloc(optTestPayload)

		// Update orderbook with optimized batch update
		ob.UpdateBatch(update.Bids, update.Asks, update.FinalUpdateID)

		// Get top level
		var bestBid, bestAsk float64
		if len(ob.Bids) > 0 {
			bestBid = ob.Bids[0].Price
		}
		if len(ob.Asks) > 0 {
			bestAsk = ob.Asks[0].Price
		}

		// Format with pooled buffer
		bufPtr := outputPoolTest.Get().(*[]byte)
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
		outputPoolTest.Put(bufPtr)

		parsing.ReleaseDepthUpdate(update)
	}
}

// BenchmarkOpt11_Pipeline_C_UltraOptimized - AFTER+: Maximum optimization
func BenchmarkOpt11_Pipeline_C_UltraOptimized(b *testing.B) {
	ob := orderbook.NewOrderBook("BTCUSDT")
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		// Parse with unsafe string conversion (zero-copy)
		update, _ := parsing.ParseFastJSONUnsafe(optTestPayload)

		// Update orderbook
		ob.UpdateBatch(update.Bids, update.Asks, update.FinalUpdateID)

		// Get top level
		var bestBid, bestAsk float64
		if len(ob.Bids) > 0 {
			bestBid = ob.Bids[0].Price
		}
		if len(ob.Asks) > 0 {
			bestAsk = ob.Asks[0].Price
		}

		// Format with pooled buffer
		bufPtr := outputPoolTest.Get().(*[]byte)
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
		outputPoolTest.Put(bufPtr)

		parsing.ReleaseDepthUpdate(update)
	}
}

// ============================================================================
// OPTIMIZATION 12: GC PRESSURE MEASUREMENT
// Demonstrates allocation reduction impact on garbage collection
// ============================================================================

// BenchmarkOpt12_GC_A_HighAlloc - BEFORE: High allocation path
func BenchmarkOpt12_GC_A_HighAlloc(b *testing.B) {
	runtime.GC() // Start with clean heap
	var m1, m2 runtime.MemStats
	runtime.ReadMemStats(&m1)

	ob := orderbook.NewOrderBook("BTCUSDT")
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		update, _ := parsing.ParseFastJSON(optTestPayload)
		ob.Update(update.Bids, update.Asks, update.FinalUpdateID)
	}

	runtime.ReadMemStats(&m2)
	b.ReportMetric(float64(m2.NumGC-m1.NumGC), "gc_cycles")
}

// BenchmarkOpt12_GC_B_LowAlloc - AFTER: Low allocation path
func BenchmarkOpt12_GC_B_LowAlloc(b *testing.B) {
	runtime.GC() // Start with clean heap
	var m1, m2 runtime.MemStats
	runtime.ReadMemStats(&m1)

	ob := orderbook.NewOrderBook("BTCUSDT")
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		update, _ := parsing.ParseFastJSONUnsafe(optTestPayload)
		ob.UpdateBatch(update.Bids, update.Asks, update.FinalUpdateID)
		parsing.ReleaseDepthUpdate(update)
	}

	runtime.ReadMemStats(&m2)
	b.ReportMetric(float64(m2.NumGC-m1.NumGC), "gc_cycles")
}
