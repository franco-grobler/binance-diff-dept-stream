# Binance Depth Streamer

A high-performance Go application that connects to the Binance WebSocket API,
maintains a local order book in memory, and prints top-level price updates in
real-time.

## Features

- **Concurrency:** Uses a "Sharded Worker" pattern where each symbol is
  processed by a dedicated goroutine. This eliminates the need for global locks
  (Mutexes) on the OrderBook data, ensuring thread safety via the Actor Model
  (CSP).
- **Performance:**
  - **Sorted Slices:** Uses contiguous memory slices for Bids/Asks instead of
    Linked Lists to maximize CPU cache locality.
  - **Fast JSON:** Utilizes `valyala/fastjson` for parsing, offering ~2x speedup
    over the standard library.
  - **Zero-Allocation Logic:** Critical paths are optimized to minimize Garbage
    Collection (GC) pressure.
- **Resilience:** Graceful shutdown handling and context propagation.

## Setup & Execution

### Prerequisites

- Docker or Go 1.25+

### Running with Docker (Recommended)

1. **Build the image:**

   ```bash
   docker build -t binance-streamer .
   ```

2. **Run the container:**

   ```bash
   docker run --rm -it binance-streamer
   ```

### Running Locally

1. **Install dependencies:**

   ```bash
   go mod download
   ```

2. **Run the application:**

   ```bash
   go run ./cmd/streamer
   ```

## Design Decisions

### 1. Data Structure: Sorted Slices vs. Linked Lists vs. Trees

**Decision:** `[]PriceLevel` (Slice)

- **Why:** While a Red-Black Tree offers O(log N) updates, a Slice offers O(1)
  random access for reading the top level. More importantly, Slices use
  contiguous memory, which is CPU cache-friendly. For an order book depth of <
  5000 items, the cost of `copy()` to shift elements is negligible compared to
  the pointer-chasing overhead and cache misses of a Linked List or Tree.

### 2. Concurrency: Channel-based Sharding

**Decision:** One Goroutine per Symbol

- **Why:** Instead of protecting a single global OrderBook with a `sync.RWMutex`
  (which causes contention between the Reader and Writer), we shard the work.
  The WebSocket reader creates a stream of events, and a Dispatcher routes them
  to specific Worker channels. Each Worker _owns_ its OrderBook exclusively.
  This is lock-free processing.

### 3. JSON Parsing

**Decision:** `valyala/fastjson`

- **Why:** The standard `encoding/json` uses reflection and allocates
  significant memory. `fastjson` allows for faster parsing.
- **Benchmark Results:**
  - `Standard`: ~4000 ns/op
  - `FastJSON`: ~2300 ns/op (**~42% Faster**)

## Benchmarks

You can run the included benchmarks to verify the JSON parsing performance:

```bash
go test -bench . -benchmem ./internal/parsing
```

## Limitations

- **Error Handling:** The current implementation logs and drops errors (e.g.,
  connection loss). A production-ready version would implement exponential
  backoff reconnection logic.
- **Memory Growth:** The order book slices grow indefinitely if prices move
  significantly. A production version should trim the book (e.g., keep only top
  1000 levels) to prevent memory leaks.
- **Hardcoded Symbols:** The symbols are currently hardcoded in `main.go`. In a
  real app, these would be loaded from a config file or environment variable.
