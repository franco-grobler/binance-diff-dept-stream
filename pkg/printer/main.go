package printer

import "context"

// Currency is an alias for handling currency amounts.
type Currency = float64

// TopLevelPrices provides a struct for serialising symbols for printing.
type TopLevelPrices struct {
	Symbol    string   `json:"symbol"`
	Ask       Currency `json:"ask"`
	Bid       Currency `json:"bid"`
	Timestamp uint64   `json:"ts"`
}

// Printer provides an interface to write to an output stream.
type Printer interface {
	Write(data ...any) error
	PriceWriter(ctx context.Context, readCh <-chan TopLevelPrices)
}
