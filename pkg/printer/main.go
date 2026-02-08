package printer

// Currency is an alias for handling currency amounts. Following banking standards, decimals are saved as 128 bit integers, allowing for accuracy up to 5 decimals. Reduce to 32 bits, since number below 2^32/10,000 (~= 429,000) should be sufficient for the order book.
type Currency = int32

// TopLevelPrices provides a struct for serialising symbols for printing.
type TopLevelPrices struct {
	Symbol    string   `json:"symbol"`
	Ask       Currency `json:"ask"`
	Bid       Currency `json:"bid"`
	Timestamp int32    `json:"ts"` // Assume this won't run after 2034
}

// Printer provides an interface to write to an output stream.
type Printer interface {
	Write(readCh chan<- []any) error
}
