package orderbook

// PriceLevel is an alias for price levels, with ["0.00907200","2.92100000"] indicating the price, and quantity respectively.
// type PriceLevel [2]string

import (
	"sort"
	"strconv"
)

// PriceLevel represents a price and quantity pair.
// Keeping it simple for now, relying on float64 for price sorting.
type PriceLevel struct {
	Price    float64
	Quantity string // Quantity kept as string to preserve precision for display/logic
}

// OrderBook maintains the current state of Bids and Asks.
// It uses Sorted Slices for cache locality.
type OrderBook struct {
	Symbol       string
	Bids         []PriceLevel // Sorted DESC (High to Low)
	Asks         []PriceLevel // Sorted ASC (Low to High)
	LastUpdateID int64
}

// DefaultCapacity is the initial capacity for Bids/Asks slices.
// Set to 500 to accommodate typical order book depths without reallocation.
// Binance depth streams typically send up to 1000 levels, but most activity
// is in the top 500 levels.
const DefaultCapacity = 5000

// MaxLevels is the maximum number of levels to retain in the order book.
// This prevents unbounded memory growth when prices move significantly.
// Set to 0 to disable trimming.
const MaxLevels = 1000

// NewOrderBook creates a new empty order book.
func NewOrderBook(symbol string) *OrderBook {
	return &OrderBook{
		Symbol: symbol,
		Bids:   make([]PriceLevel, 0, DefaultCapacity),
		Asks:   make([]PriceLevel, 0, DefaultCapacity),
	}
}

// Update applies a batch of updates to the orderbook.
// updates format: [][2]string where [0] is price, [1] is quantity.
func (ob *OrderBook) Update(bidsRaw, asksRaw [][2]string, u int64) error {
	ob.LastUpdateID = u

	// 1. Process Bids (Descending)
	for _, raw := range bidsRaw {
		price, err := strconv.ParseFloat(raw[0], 64)
		if err != nil {
			return err
		}
		qty := raw[1]

		ob.updateLevel(&ob.Bids, price, qty, true)
	}

	// 2. Process Asks (Ascending)
	for _, raw := range asksRaw {
		price, err := strconv.ParseFloat(raw[0], 64)
		if err != nil {
			return err
		}
		qty := raw[1]

		ob.updateLevel(&ob.Asks, price, qty, false)
	}

	return nil
}

// updateLevel handles the insertion, update, or deletion of a price level.
func (ob *OrderBook) updateLevel(levels *[]PriceLevel, price float64, qty string, desc bool) {
	// Binary Search to find the index
	n := len(*levels)
	idx := sort.Search(n, func(i int) bool {
		if desc {
			return (*levels)[i].Price <= price // Descending: find where current <= target
		}
		return (*levels)[i].Price >= price // Ascending: find where current >= target
	})

	// Check if we found an exact match
	found := idx < n && (*levels)[idx].Price == price

	// Case 1: Delete (Quantity is "0.00000000" or "0" or "0.0")
	isDelete := isZeroQuantity(qty)

	if isDelete {
		if found {
			// Remove element at idx
			*levels = append((*levels)[:idx], (*levels)[idx+1:]...)
		}
		return
	}

	// Case 2: Update Existing
	if found {
		(*levels)[idx].Quantity = qty
		return
	}

	// Case 3: Insert New
	// Insert at idx
	*levels = append(*levels, PriceLevel{})  // Grow by 1
	copy((*levels)[idx+1:], (*levels)[idx:]) // Shift right
	(*levels)[idx] = PriceLevel{Price: price, Quantity: qty}
}

// isZeroQuantity checks if a quantity string represents zero.
// This is faster than strconv.ParseFloat for the common case.
// Binance typically sends "0.00000000" for deletes.
func isZeroQuantity(qty string) bool {
	if len(qty) == 0 {
		return false
	}
	// Fast path: check for "0" or "0.0..." patterns
	for i := 0; i < len(qty); i++ {
		c := qty[i]
		if c == '0' || c == '.' {
			continue
		}
		return false
	}
	return true
}
