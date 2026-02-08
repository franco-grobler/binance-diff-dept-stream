// Package main serves as the entry-point for the order book manager.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/franco-grobler/binance-diff-stream/internal/orderbook"
	binanacestream "github.com/franco-grobler/binance-diff-stream/pkg/binanace-stream"
	"github.com/franco-grobler/binance-diff-stream/pkg/printer"
)

var Symbols = []string{"BTCUSDT", "ETHUSDT", "BNBUSDT"}

func main() {
	// Setup Context with graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		log.Println("Shutdown signal received")
		cancel()
	}()

	// Setup Channels
	// rawCh receives raw bytes from the WebSocket
	rawCh := make(chan []byte, 1000)

	// symbolChans maps "BTCUSDT" -> channel for that specific worker
	symbolChans := make(map[string]chan *binanacestream.DiffStream)
	for _, s := range Symbols {
		symbolChans[s] = make(chan *binanacestream.DiffStream, 100)
	}

	// printCh is the channel for the dedicated printer goroutine
	printCh := make(chan printer.TopLevelPrices, 100)

	// Create WebSocket reader.
	client := binanacestream.NewDefaultClient(ctx)
	go printerWorker(ctx, printCh)
	go websocketClient(client, rawCh, symbolChans)
	for _, s := range Symbols {
		go startWorker(client, s, symbolChans[s], printCh)
	}

	// Block until context is done (via signal)
	<-ctx.Done()
	log.Println("Exiting...")
}

func websocketClient(
	client binanacestream.Client,
	dataCh chan []byte,
	symbolCh map[string]chan *binanacestream.DiffStream,
) {
	ctx := client.WSClient.GetContext()
	if err := client.DepthStream(Symbols, binanacestream.Second, dataCh); err != nil {
		panic(err)
	}

	for {
		select {
		case <-ctx.Done():
			return
		case data, ok := <-dataCh:
			if !ok {
				return
			}

			update := &binanacestream.DiffStream{}
			if err := client.Parser(data, update); err != nil {
				log.Printf("Error parsing JSON: %v\n", err)
				continue
			}

			if ch, exists := symbolCh[update.Symbol]; exists {
				select {
				case ch <- update:
				default:
					log.Printf("Worker channel full for %s, dropping update\n", update.Symbol)
				}
			}
		}
	}
}

// startWorker maintains the OrderBook for a single symbol.
// It is the ONLY goroutine allowed to touch its OrderBook, ensuring thread safety.
// After processing an update, it sends top-level data to the printer channel.
func startWorker(
	client binanacestream.Client,
	symbol string,
	in <-chan *binanacestream.DiffStream,
	printCh chan<- printer.TopLevelPrices,
) {
	ctx := client.WSClient.GetContext()

	ob := orderbook.NewOrderBook(symbol)

	for {
		snapshot, snapErr := client.DepthSnapshot(symbol, 5000)
		if snapErr != nil {
			panic(snapErr)
		}

		update := <-in
		if update.FirstUpdateID <= uint64(snapshot.LastUpdateID) {
			fmt.Printf("Retrying snapshot for %s... \n", symbol)
			continue
		}

		if err := ob.Update(
			snapshot.Bids, snapshot.Asks, snapshot.LastUpdateID,
		); err != nil {
			panic(err)
		}
		break
	}

	for {
		select {
		case <-ctx.Done():
			return
		case update := <-in:
			// Apply updates
			err := ob.Update(
				update.UpdateBids,
				update.UpdateBids,
				int64(update.FinalUpdateID),
			)
			if err != nil {
				log.Printf("[%s] Update error: %v", symbol, err)
				continue
			}

			// Get top level prices
			var bestBid, bestAsk float64
			if len(ob.Bids) > 0 {
				bestBid = ob.Bids[0].Price
			}
			if len(ob.Asks) > 0 {
				bestAsk = ob.Asks[0].Price
			}

			// Send to printer (non-blocking to avoid back-pressure)
			select {
			case printCh <- printer.TopLevelPrices{
				Symbol:    ob.Symbol,
				Ask:       bestAsk,
				Bid:       bestBid,
				Timestamp: update.EventTime,
			}:
			default:
				log.Printf("[%s] Printer channel full, dropping update", symbol)
			}
		}
	}
}

func printerWorker(ctx context.Context, printCh <-chan printer.TopLevelPrices) {
	stdPrinter := printer.Stdout{}
	stdPrinter.PriceWriter(ctx, printCh)
}
