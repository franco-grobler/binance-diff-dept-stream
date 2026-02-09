# Binance diff-dept-stream

Binance offers a websocket stream allowing the user to update a local order book
with the latest value of a token.

## Overview

In this assessment, you’ll be creating a Go application which interacts with
Binance’s WebSocket API. Your solution should be containerized using Docker.
Please also provide a README that explains how to set up and run the
application, any design decisions you made, performance considerations, and any
limitations of your design. Use idiomatic Go standards and project layouts where
possible.

### Tasks

1. Connect to Binance’s WebSocket API and subscribe to the DiM. Depth Stream.
    - [Documentation can be found here.](https://developers.binance.com/docs/binance-spot-api-docs/web-socket-streams#diM-depth-stream)
    - Process incoming data on multiple symbols, using appropriate data
      structures to maintain all orderbook levels in memory. Use the standard
      library JSON package (V1) to perform the unmarshalling.
    - On each update, print the top level prices to stdout in the following
      format:
      `“{symbol”:”BTCUSDT”,“ask”:100001.00,”bid”:100000.00 “ts”:1672515782136}”`

2. Propose alternate strategies for the unmarshalling step above in order to
   improve performance.
    - Your strategies should consider: heap allocations and speed.
    - Provide verification via testing and benchmarks.

## Design

The application maintains a real-time local order book by connecting to
Binance's WebSocket API and processing depth updates. The architecture follows a
pipeline pattern with three distinct stages, each running in its own goroutine.

### Architecture Overview

```text
┌─────────────────┐     ┌─────────────────┐     ┌─────────────────┐
│   WebSocket     │     │   Per-Symbol    │     │    Printer      │
│   Client        │────▶│   Workers       │────▶│    Worker       │
└─────────────────┘     └─────────────────┘     └─────────────────┘
        │                       │                       │
   rawCh []byte          symbolChans            printCh TopLevelPrices
   (buffered: 1000)      (buffered: 100)        (buffered: 100)
```

### Pipeline Stages

#### Stage 1: WebSocket Client (`websocketClient`)

**Responsibility:** Connect to Binance WebSocket API and distribute raw messages
to symbol-specific workers.

**Design Decisions:**

- **Single WebSocket connection** for all symbols using Binance's combined
  streams feature.
- **Raw byte forwarding** - JSON parsing happens in the websocket client to
  determine routing, but the raw message is forwarded to workers
- **Non-blocking sends** with drop semantics - if a worker's channel is full,
  the update is dropped with a log message rather than blocking the entire
  pipeline
- **Buffered channel (1000)** provides backpressure absorption for burst traffic

```go
// Non-blocking send to prevent slow consumers from blocking the pipeline
select {
case ch <- update:
default:
    log.Printf("Worker channel full for %s, dropping update\n", update.Symbol)
}
```

#### Stage 2: Per-Symbol Workers (`startWorker`)

**Responsibility:** Maintain the order book state for a single symbol and
extract top-level prices.

**Design Decisions:**

- **One worker per symbol** - ensures thread safety without locks by having
  exclusive ownership of each order book
- **Initial snapshot synchronization** - follows Binance's recommended
  procedure:
    1. Fetch depth snapshot via REST API
    2. Wait for WebSocket update where `FirstUpdateID > snapshot.LastUpdateID`
    3. Apply snapshot, then continue with real-time updates
- **Retry logic** for snapshot synchronization if the first update arrives
  before the snapshot is valid

```go
// Snapshot synchronization loop
for {
    snapshot, _ := client.DepthSnapshot(symbol, 5000)
    update := <-in
    if update.FirstUpdateID <= uint64(snapshot.LastUpdateID) {
        continue // Retry - snapshot is stale
    }
    ob.Update(snapshot.Bids, snapshot.Asks, snapshot.LastUpdateID)
    break
}
```

#### Stage 3: Printer Worker (`printerWorker`)

**Responsibility:** Serialize and output top-level prices to stdout.

**Design Decisions:**

- **Dedicated goroutine** prevents I/O blocking from affecting order book
  updates
- **JSON output format** as specified in requirements
- **Non-blocking receives** from workers with drop semantics

### Data Structures

#### Order Book (`internal/orderbook`)

**Design Decisions:**

- **Sorted slices** instead of maps for better cache locality during iteration
- **Binary search** for O(log n) insert/update/delete operations
- **Pre-allocated capacity** (5000 levels) to reduce allocations during
  operation
- **Bids sorted descending**, Asks sorted ascending for O(1) best price access

```go
type OrderBook struct {
    Symbol       string
    Bids         []PriceLevel // Sorted DESC (High to Low)
    Asks         []PriceLevel // Sorted ASC (Low to High)
    LastUpdateID int64
}
```

**Why sorted slices over maps:**

| Operation     | Map           | Sorted Slice       |
| ------------- | ------------- | ------------------ |
| Insert/Update | O(1) average  | O(log n) + O(n)    |
| Delete        | O(1)          | O(log n) + O(n)    |
| Find Best     | O(n)          | O(1)               |
| Iteration     | Poor locality | Excellent locality |
| Memory        | Higher        | Lower              |

For order books where we frequently need the best bid/ask, sorted slices provide
O(1) access to the top of book while maintaining reasonable update performance.

#### Price Level

```go
type PriceLevel struct {
    Price    float64 // Used for sorting and comparison
    Quantity string  // Kept as string to preserve precision
}
```

**Design Decisions:**

- **Price as float64** - enables efficient sorting and comparison
- **Quantity as string** - preserves original precision from Binance API without
  floating-point representation issues

#### Zero Quantity Detection

```go
func isZeroQuantity(qty string) bool {
    for i := 0; i < len(qty); i++ {
        c := qty[i]
        if c == '0' || c == '.' {
            continue
        }
        return false
    }
    return true
}
```

**Design Decision:** Custom string check instead of `strconv.ParseFloat`
because:

- Binance consistently sends "0.00000000" for deletions
- String iteration is faster than float parsing for this specific case
- Avoids floating-point comparison issues

### Client Architecture (`pkg/binance-stream`)

**Design Decisions:**

- **Interface-based design** for HTTP and WebSocket clients enables dependency
  injection and testing
- **Configurable parser** allows swapping JSON implementations for performance
  tuning

```go
type Client struct {
    apiURL     string
    wsURL      string
    HTTPClient HTTPClient  // Interface for testing
    WSClient   WSClient    // Interface for testing
    Parser     Parser      // Swappable JSON parser
}
```

**Interfaces:**

```go
type HTTPClient interface {
    Do(req *http.Request) (*http.Response, error)
}

type WSClient interface {
    GetContext() context.Context
    Dial(
        url string, opts *websocket.DialOptions,
    ) (WSConnection, *http.Response, error)
}

type WSConnection interface {
    Read(ctx context.Context) (websocket.MessageType, []byte, error)
    Write(ctx context.Context, typ websocket.MessageType, p []byte) error
    Close(code websocket.StatusCode, reason string) error
}

type Parser func(data []byte, v any) error
```

The `WSConnection` interface allows mocking WebSocket connections for
comprehensive unit testing without requiring a real WebSocket server.

### Concurrency Model

**Design Decisions:**

- **No shared mutable state** - each order book is owned by exactly one
  goroutine
- **Channel-based communication** - goroutines communicate via buffered channels
- **Non-blocking sends** - prevents slow consumers from blocking producers
- **Context-based cancellation** - graceful shutdown via context propagation

```text
main goroutine
    │
    ├── printerWorker (1)
    │       └── reads from printCh
    │
    ├── websocketClient (1)
    │       ├── reads from WebSocket
    │       └── writes to symbolChans[symbol]
    │
    └── startWorker (N, one per symbol)
            ├── reads from symbolChans[symbol]
            └── writes to printCh
```

### Signal Handling

The application handles SIGINT and SIGTERM for graceful shutdown:

```go
sigCh := make(chan os.Signal, 1)
signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
go func() {
    <-sigCh
    cancel() // Propagates cancellation to all goroutines
}()
```

### Performance Considerations

1. **Buffered channels** absorb burst traffic and prevent goroutine blocking
2. **Pre-allocated slices** reduce memory allocations during operation
3. **Binary search** provides O(log n) lookups in the order book
4. **Non-blocking sends** prevent cascade blocking in the pipeline
5. **Single WebSocket connection** reduces connection overhead for multiple
   symbols

### Trade-offs

| Decision              | Benefit                   | Trade-off                 |
| --------------------- | ------------------------- | ------------------------- |
| Sorted slices vs maps | O(1) best price, locality | O(n) insert shift         |
| Non-blocking sends    | No cascade blocking       | Data loss under pressure  |
| One worker per symbol | No locks needed           | More goroutines           |
| String quantity       | Precision preserved       | Parse cost for arithmetic |

## Developer environment

The environment can be built using [devbox](https://www.jetify.com/devbox).

| Tool     | Use                       |
| -------- | ------------------------- |
| just     | Command runner            |
| prettier | Formatting markdown files |

## Possible Improvements and Shortcomings

### Architectural Improvements

#### Graceful Shutdown

The application has basic signal handling but could be improved:

**Current implementation:**

- Handles SIGINT and SIGTERM via context cancellation
- Goroutines check `ctx.Done()` to exit

**Recommended improvements:**

- Use `sync.WaitGroup` to track goroutine completion
- Add timeout for graceful shutdown before force exit
- Ensure all resources (WebSocket connections, channels) are properly closed

#### Configuration Management

Hardcoded values like API URLs, symbols, and limits should be configurable:

```go
// Current:
"https://api.binance.com/api"
"wss://stream.binance.com:9443/ws"

// Recommended: Use environment variables or config file
cfg := config.Load()
client := NewClient(cfg.APIURL, cfg.WSURL, ...)
```

### Performance Improvements

#### JSON Unmarshalling

The current implementation uses `encoding/json` which is simple but not the
fastest. Consider:

- **easyjson**: Code generation for faster unmarshalling
- **jsoniter**: Drop-in replacement, ~5-6x faster
- **sonic**: Fastest option for amd64

Benchmarks should verify improvements for the specific data structures used.

#### Memory Allocations

The order book uses sorted slices which may grow during operation. Consider:

- Pre-allocating slice capacity based on expected depth (currently 5000)
- Using a fixed-size ring buffer for high-frequency updates
- Object pooling for `PriceLevel` structs in hot paths

#### Channel Buffer Sizes

The channel buffer sizes are hardcoded. Consider:

- Making buffer sizes configurable
- Monitoring channel utilization
- Adding backpressure mechanisms

### Testing Improvements

#### Current Coverage

| Package            | Coverage | Notes                              |
| ------------------ | -------- | ---------------------------------- |
| internal/orderbook | 100%     | Fully covered                      |
| pkg/binance-stream | 91.7%    | Mockable WSConnection interface    |
| pkg/printer        | 90%      | Error logging path untested        |
| cmd/streamer       | 0%       | Main package, requires integration |

All tests run without external network calls. WebSocket and HTTP client behavior
is tested using mocks and local test servers (`httptest.NewServer`).

#### WebSocket Testing

The `WSConnection` interface enables comprehensive unit testing of the
`DepthStream` goroutine:

```go
// MockWSConnection for testing
type MockWSConnection struct {
    ReadFunc  func(ctx context.Context) (websocket.MessageType, []byte, error)
    WriteFunc func(ctx context.Context, typ websocket.MessageType, p []byte) error
    CloseFunc func(code websocket.StatusCode, reason string) error
}
```

Test coverage includes:

- Message reading and forwarding
- Read error handling (graceful exit)
- Context cancellation
- Connection close behavior

#### Future Improvements

Consider adding integration tests that:

- Connect to Binance testnet
- Verify order book synchronization
- Test reconnection logic
- Validate output format

#### Benchmarks

Add benchmarks for:

- Order book update throughput
- JSON parsing performance
- Memory allocation profiling

### Observability Improvements

#### Structured Logging

Replace `log.Printf` with structured logging (e.g., `slog`, `zap`, `zerolog`):

```go
logger.Info("order book updated",
    "symbol", symbol,
    "bid", topBid,
    "ask", topAsk,
    "latency_ms", latency,
)
```

#### Metrics

Add Prometheus metrics for:

- Messages processed per second
- Order book update latency
- WebSocket connection state
- Error rates

#### Health Checks

Implement health check endpoints for container orchestration:

- Liveness: Application is running
- Readiness: WebSocket connected and receiving data

### Documentation Improvements

- Include architecture diagrams
