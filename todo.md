# Binance Depth Streamer Implementation Plan

## Phase 1: Core Data Structures (The "List")

- [x] Create `internal/orderbook` package
- [x] Implement `OrderBook` struct with Bids and Asks using **sorted slices**
      (Optimized for cache locality)
- [x] Implement `Update(bids, asks)` logic to handle diffs (insert, update,
      delete) while maintaining sort order
- [x] **Verification:** Add unit tests to `internal/orderbook` covering edge
      cases (empty book, delete non-existent, cross-spread updates)

## Phase 2: JSON Performance & Benchmarking

- [x] Create `internal/parsing` package
- [x] Implement standard `encoding/json` parser
- [x] Implement optimized parser (using `fastjson` or custom zero-allocation
      parser)
- [x] **Verification:** Create `parsing_test.go` with Go Benchmarks comparing
      allocations and speed (ns/op)

## Phase 3: WebSocket Client & Ingestion

- [x] Add `github.com/coder/websocket` dependency
- [x] Refactor `pkg/binanace-stream` to implement a real WebSocket client using
      `github.com/coder/websocket`
- [x] Implement the "Sharded Channel" pattern: One channel per symbol to ensure
      thread isolation
- [x] **Verification:** Create an integration test that pipes mock JSON data
      into the client and verifies the channel receives objects

## Phase 4: Application Wiring (Concurrency)

- [x] Update `cmd/streamer/main.go` to spawn:
  - 1 WS Reader (distributor)
  - N OrderBook Processors (one per symbol, owning their own state)
  - 1 Printer (stdout)
- [x] Implement the "Top Level" printing format
      (`{"symbol":..., "ask":..., "bid":...}`)
- [x] **Verification:** Run the binary locally and verify output matches the
      requested format

## Phase 5: Delivery

- [x] Create `Dockerfile` (multi-stage build)
- [x] Write `README.md` (Setup, Design Decisions, Benchmark Results)
