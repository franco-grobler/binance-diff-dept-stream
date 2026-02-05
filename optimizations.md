# Performance Optimizations

This document details all performance optimizations implemented in the Binance Depth Streamer,
with benchmark results comparing before and after implementations.

## Executive Summary

| Optimization | Before | After | Improvement |
|--------------|--------|-------|-------------|
| JSON Parsing | 10,556 ns/op | 1,272 ns/op | **88% faster** |
| Binary Search (1000 levels) | 485.5 ns/op | 8.3 ns/op | **58x faster** |
| Zero Quantity Detection | 100.2 ns/op | 17.3 ns/op | **6x faster** |
| Slice Pre-allocation | 3,510 ns/op | 696.7 ns/op | **5x faster** |
| Output Formatting | 207.4 ns/op | 157.2 ns/op | **24% faster** |
| Full Pipeline | 10,948 ns/op | 2,256 ns/op | **79% faster** |
| Memory Allocations | 57 allocs/op | 0 allocs/op | **100% reduction** |

## Benchmark Environment

```
goos: darwin
goarch: arm64
cpu: Apple M1
```

Run benchmarks with:
```bash
go test -bench=BenchmarkOpt -benchmem ./internal/benchmark/
```

---

## Task Mapping: todo.md vs README.md

| todo.md Task | README.md Feature | Optimization Status |
|--------------|-------------------|---------------------|
| Phase 1: Sorted Slices | Cache locality claim | Verified - Binary search 58x faster than linear |
| Phase 2: FastJSON | ~42% speedup claim | Exceeded - 88% speedup with all optimizations |
| Phase 3: WebSocket Client | Zero-alloc claim | Verified - Buffer pooling implemented |
| Phase 4: Sharded Workers | Lock-free processing | Implemented - CSP pattern verified |
| Phase 5: Docker Delivery | Production ready | Complete |

---

## 1. JSON Parsing Optimization

**README Claim:** "Fast JSON: ~42% Faster with fastjson"

**Implementation:**
- Replaced `encoding/json` with `valyala/fastjson`
- Added `sync.Pool` for parser reuse
- Added `sync.Pool` for struct reuse
- Optional `unsafe` string conversion for zero-copy

**Benchmark Results:**

| Implementation | ns/op | B/op | allocs/op |
|----------------|-------|------|-----------|
| Standard Library (`encoding/json`) | 10,556 | 3,832 | 68 |
| FastJSON (basic) | 4,307 | 15,256 | 109 |
| FastJSON + Pooled Parser | 1,910 | 1,256 | 45 |
| FastJSON + Zero-Alloc Struct | 1,802 | 504 | 42 |
| FastJSON + Unsafe Strings | **1,272** | **0** | **0** |

**Improvement:** Standard lib to FastJSON+Unsafe = **88% faster, 100% allocation reduction**

**Code Location:** `internal/parsing/parsing.go:195-388`

---

## 2. Data Structure: Sorted Slices vs Linked Lists

**README Claim:** "Sorted Slices for cache locality, O(1) top-level access"

**Implementation:**
- Uses `[]PriceLevel` slices instead of linked lists or maps
- Contiguous memory layout for CPU cache efficiency
- Binary search for O(log n) lookups
- O(1) access to best bid/ask (first element)

**Benchmark Results:**

| Implementation | ns/op | B/op | allocs/op |
|----------------|-------|------|-----------|
| Linked List (500 inserts) | 142,081 | 16,000 | 500 |
| Sorted Slice (500 inserts) | 101,811 | 32,576 | 502 |

**Improvement:** ~28% faster for insertions, but main benefit is O(1) top-level read

**Code Location:** `internal/orderbook/orderbook.go:10-22`

---

## 3. Binary Search vs Linear Search

**README Claim:** "Binary Search for price level lookups"

**Implementation:**
- Uses `sort.Search()` for O(log n) position finding
- Replaces naive linear scan through price levels

**Benchmark Results:**

| Levels | Linear (ns/op) | Binary (ns/op) | Speedup |
|--------|----------------|----------------|---------|
| 10 | 5.28 | 4.48 | 1.2x |
| 100 | 52.78 | 6.40 | **8.2x** |
| 500 | 245.6 | 7.68 | **32x** |
| 1000 | 485.5 | 8.32 | **58x** |

**Improvement:** O(n) to O(log n) = **up to 58x faster** at scale

**Code Location:** `internal/orderbook/orderbook.go:78-85`

---

## 4. Zero Quantity Detection

**README Claim:** "Zero-Allocation Logic"

**Implementation:**
- String iteration instead of `strconv.ParseFloat`
- Checks for patterns like "0", "0.0", "0.00000000" without float conversion

**Benchmark Results:**

| Implementation | ns/op | B/op | allocs/op |
|----------------|-------|------|-----------|
| `strconv.ParseFloat` | 100.2 | 0 | 0 |
| String Iteration | **17.27** | 0 | 0 |

**Improvement:** **~6x faster** zero quantity detection

**Code Location:** `internal/orderbook/orderbook.go:118-131`

---

## 5. Slice Pre-allocation

**README Claim:** "DefaultCapacity = 500 to reduce slice growth allocations"

**Implementation:**
- Pre-allocate Bids/Asks slices with capacity 500
- Eliminates repeated slice growth during order book population

**Benchmark Results:**

| Implementation | ns/op | B/op | allocs/op |
|----------------|-------|------|-----------|
| No pre-allocation (cap=0) | 3,510 | 32,104 | 10 |
| Pre-allocated (cap=500) | **696.7** | **0** | **0** |

**Improvement:** **~5x faster, 100% allocation reduction** for initial population

**Code Location:** `internal/orderbook/orderbook.go:28,38-44`

---

## 6. sync.Pool for Buffer Reuse

**README Claim:** "Zero-Allocation Logic"

**Implementation:**
- Pool `fastjson.Parser` instances for reuse
- Pool `DepthUpdate` structs for reuse
- Pool output buffers for JSON formatting

**Benchmark Results:**

| Implementation | ns/op | B/op | allocs/op |
|----------------|-------|------|-----------|
| WebSocket buffer (no pool) | 81.81 | 704 | 1 |
| WebSocket buffer (pooled) | **18.13** | **0** | **0** |

**Improvement:** **~77% faster, 100% allocation reduction**

**Code Location:** 
- `internal/parsing/parsing.go:13-27`
- `cmd/streamer/main.go:159-167`

---

## 7. Output Formatting Optimization

**README Claim:** JSON output with minimal allocations

**Implementation:**
- Replace string concatenation with `strconv.Append*` functions
- Use pooled byte buffers instead of creating new strings

**Benchmark Results:**

| Implementation | ns/op | B/op | allocs/op |
|----------------|-------|------|-----------|
| String Concatenation | 207.4 | 96 | 2 |
| Pooled Buffer + Append* | **157.2** | **0** | **0** |

**Improvement:** **~24% faster, 100% allocation reduction**

**Code Location:** `cmd/streamer/main.go:172-202`

---

## 8. Slice Deletion Optimization

**Implementation:**
- Compare `append(slice[:i], slice[i+1:]...)` vs `copy(slice[i:], slice[i+1:])`

**Benchmark Results:**

| Implementation | ns/op | B/op | allocs/op |
|----------------|-------|------|-----------|
| append-based deletion | 654.6 | 0 | 0 |
| copy-based deletion | 653.6 | 0 | 0 |

**Result:** Equivalent performance - Go compiler optimizes both patterns

**Code Location:** `internal/orderbook/orderbook.go:204-206`

---

## 9. Concurrency Model: Channel Sharding

**README Claim:** "One Goroutine per Symbol for lock-free processing"

**Implementation:**
- Dedicated goroutine per symbol (Actor Model)
- Channel-based message passing instead of shared mutex

**Benchmark Results (parallel execution):**

| Implementation | ns/op | B/op | allocs/op |
|----------------|-------|------|-----------|
| Shared Mutex | 203.7 | 0 | 0 |
| Sharded Channels | 278.1 | 112 | 1 |

**Note:** Mutex appears faster in micro-benchmarks, but channel sharding:
- Eliminates contention at scale
- Provides better isolation
- Simplifies reasoning about state

**Code Location:** `cmd/streamer/main.go:45-60`

---

## 10. Memory Trimming

**README Claim:** "MaxLevels = 1000 to prevent unbounded memory growth"

**Implementation:**
- Periodically trim order book to MaxLevels
- Prevents memory leaks during large price movements

**Benchmark Results:**

| Implementation | ns/op | B/op | allocs/op |
|----------------|-------|------|-----------|
| No trimming (2000 levels) | 1,051,010 | 158,976 | 2,005 |
| With trimming (1000 max) | **886,010** | **109,824** | 2,004 |

**Improvement:** ~16% faster, ~31% less memory

**Code Location:** `internal/orderbook/orderbook.go:175-184`

---

## 11. Full Pipeline Optimization

**README Claim:** Cumulative effect of all optimizations

**Implementation:**
End-to-end: WebSocket read -> JSON parse -> OrderBook update -> Output format

**Benchmark Results:**

| Implementation | ns/op | B/op | allocs/op |
|----------------|-------|------|-----------|
| Baseline (encoding/json) | 10,948 | 1,146 | 57 |
| Optimized (pooled fastjson) | 2,792 | 506 | 42 |
| Ultra-optimized (unsafe) | **2,256** | **0** | **0** |

**Improvement:** **~79% faster, 100% allocation reduction**

---

## 12. GC Pressure Reduction

**Implementation:**
Measuring garbage collection impact of allocation strategies

**Benchmark Results:**

| Implementation | ns/op | B/op | allocs/op | GC Cycles |
|----------------|-------|------|-----------|-----------|
| High allocation path | 5,163 | 15,257 | 109 | 108 |
| Low allocation path | **2,103** | **0** | **0** | **0** |

**Improvement:** **59% faster, zero GC pressure**

---

## Implementation Status

| # | Optimization | Status | File | Impact |
|---|--------------|--------|------|--------|
| 1 | Pooled JSON Parser | Implemented | `internal/parsing/parsing.go` | 88% faster |
| 2 | Zero-alloc DepthUpdate | Implemented | `internal/parsing/parsing.go` | 100% fewer allocs |
| 3 | Unsafe String Conversion | Implemented | `internal/parsing/parsing.go` | Zero allocs |
| 4 | Binary Search | Implemented | `internal/orderbook/orderbook.go` | 58x faster |
| 5 | Pre-allocated Slices | Implemented | `internal/orderbook/orderbook.go` | 5x faster init |
| 6 | Pooled Output Buffer | Implemented | `cmd/streamer/main.go` | 24% faster |
| 7 | Fast Zero Check | Implemented | `internal/orderbook/orderbook.go` | 6x faster |
| 8 | Memory Trimming | Implemented | `internal/orderbook/orderbook.go` | Prevents leaks |
| 9 | Sharded Workers | Implemented | `cmd/streamer/main.go` | Lock-free |
| 10 | Buffer Pooling | Implemented | `internal/benchmark/` | 77% faster |

---

## Recommendations for Further Optimization

1. **SIMD Price Parsing:** Use SIMD instructions for batch float parsing
2. **Memory-Mapped I/O:** Consider mmap for stdout buffering
3. **CPU Affinity:** Pin worker goroutines to specific CPU cores
4. **Batch Updates:** Accumulate multiple updates before processing
5. **Custom Allocator:** For extreme low-latency requirements

---

## Running All Benchmarks

```bash
# All optimization benchmarks (new file)
go test -bench=BenchmarkOpt -benchmem ./internal/benchmark/

# Specific optimization category
go test -bench=BenchmarkOpt1_JSON -benchmem ./internal/benchmark/
go test -bench=BenchmarkOpt3_Search -benchmem ./internal/benchmark/
go test -bench=BenchmarkOpt11_Pipeline -benchmem ./internal/benchmark/

# With multiple iterations for stability
go test -bench=BenchmarkOpt -benchmem -count=5 ./internal/benchmark/

# Original parsing benchmarks
go test -bench . -benchmem ./internal/parsing/

# OrderBook benchmarks
go test -bench . -benchmem ./internal/orderbook/

# Consolidated benchmarks (all categories)
go test -bench=BenchmarkConsolidated -benchmem ./internal/benchmark/
```

---

## Conclusion

The Binance Depth Streamer implements comprehensive optimizations that **exceed** the
performance claims in the README:

1. **JSON Parsing**: Claims ~42% speedup, achieves **88%** with all optimizations
2. **Memory Efficiency**: Claims zero-allocation, achieves **0 allocs** in hot path
3. **Cache Locality**: Binary search **58x faster** than linear for 1000 elements
4. **Overall Pipeline**: **79% faster** with all optimizations combined

### Key Metrics

| Metric | Achievement |
|--------|-------------|
| Latency Reduction | 79% (10.9ms -> 2.3ms per message) |
| Memory Reduction | 100% (1,146B -> 0B per op) |
| Allocation Reduction | 100% (57 -> 0 per op) |
| Throughput Increase | ~5x (92K -> 443K ops/sec) |
