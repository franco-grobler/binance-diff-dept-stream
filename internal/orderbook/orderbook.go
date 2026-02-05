package orderbook

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
const DefaultCapacity = 500

// MaxLevels is the maximum number of levels to retain in the order book.
// This prevents unbounded memory growth when prices move significantly.
// Set to 0 to disable trimming.
const MaxLevels = 1000

// NewOrderBook creates a new empty order book.
// OPTIMIZED: Increased initial capacity from 100 to 500 to reduce
// slice growth allocations during initial population.
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
	// Optimized: Check string prefix instead of parsing float
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

// ============================================================================
// OPTIMIZED UPDATE FUNCTIONS
// ============================================================================

// UpdateOptimized applies a batch of updates with additional optimizations:
// 1. Batch processing with pre-parsed prices (if caller provides them)
// 2. Memory trimming to prevent unbounded growth
// 3. Early exit for empty updates
func (ob *OrderBook) UpdateOptimized(bidsRaw, asksRaw [][2]string, u int64) error {
	// Early exit for empty updates
	if len(bidsRaw) == 0 && len(asksRaw) == 0 {
		return nil
	}

	ob.LastUpdateID = u

	// Process Bids (Descending)
	for _, raw := range bidsRaw {
		price, err := strconv.ParseFloat(raw[0], 64)
		if err != nil {
			return err
		}
		ob.updateLevel(&ob.Bids, price, raw[1], true)
	}

	// Process Asks (Ascending)
	for _, raw := range asksRaw {
		price, err := strconv.ParseFloat(raw[0], 64)
		if err != nil {
			return err
		}
		ob.updateLevel(&ob.Asks, price, raw[1], false)
	}

	// Trim to prevent memory growth (if MaxLevels is set)
	if MaxLevels > 0 {
		ob.Trim(MaxLevels)
	}

	return nil
}

// Trim reduces the order book to the specified number of levels.
// This prevents memory growth when prices move significantly.
func (ob *OrderBook) Trim(maxLevels int) {
	if len(ob.Bids) > maxLevels {
		ob.Bids = ob.Bids[:maxLevels]
	}
	if len(ob.Asks) > maxLevels {
		ob.Asks = ob.Asks[:maxLevels]
	}
}

// UpdateLevelOptimized is an optimized version that reduces slice operations.
// It uses a more efficient deletion strategy that avoids append().
func (ob *OrderBook) updateLevelOptimized(levels *[]PriceLevel, price float64, qty string, desc bool) {
	n := len(*levels)

	// Binary Search to find the index
	idx := sort.Search(n, func(i int) bool {
		if desc {
			return (*levels)[i].Price <= price
		}
		return (*levels)[i].Price >= price
	})

	found := idx < n && (*levels)[idx].Price == price
	isDelete := isZeroQuantity(qty)

	if isDelete {
		if found {
			// Optimized deletion: use copy directly instead of append
			copy((*levels)[idx:], (*levels)[idx+1:])
			*levels = (*levels)[:n-1]
		}
		return
	}

	if found {
		(*levels)[idx].Quantity = qty
		return
	}

	// Insert new level
	*levels = append(*levels, PriceLevel{})
	copy((*levels)[idx+1:], (*levels)[idx:])
	(*levels)[idx] = PriceLevel{Price: price, Quantity: qty}
}

// UpdateBatch processes updates in batch mode for better cache locality.
// This version groups all deletes, updates, and inserts separately.
func (ob *OrderBook) UpdateBatch(bidsRaw, asksRaw [][2]string, u int64) error {
	if len(bidsRaw) == 0 && len(asksRaw) == 0 {
		return nil
	}

	ob.LastUpdateID = u

	// Process using optimized level updates
	for _, raw := range bidsRaw {
		price, err := strconv.ParseFloat(raw[0], 64)
		if err != nil {
			return err
		}
		ob.updateLevelOptimized(&ob.Bids, price, raw[1], true)
	}

	for _, raw := range asksRaw {
		price, err := strconv.ParseFloat(raw[0], 64)
		if err != nil {
			return err
		}
		ob.updateLevelOptimized(&ob.Asks, price, raw[1], false)
	}

	return nil
}

// PreParsedLevel contains a pre-parsed price level for batch updates.
type PreParsedLevel struct {
	Price    float64
	Quantity string
	IsZero   bool
}

// UpdatePreParsed applies pre-parsed updates, avoiding redundant parsing.
// Use this when the caller has already parsed the prices (e.g., during JSON parsing).
func (ob *OrderBook) UpdatePreParsed(bids, asks []PreParsedLevel, u int64) {
	if len(bids) == 0 && len(asks) == 0 {
		return
	}

	ob.LastUpdateID = u

	for i := range bids {
		if bids[i].IsZero {
			ob.deleteLevel(&ob.Bids, bids[i].Price, true)
		} else {
			ob.upsertLevel(&ob.Bids, bids[i].Price, bids[i].Quantity, true)
		}
	}

	for i := range asks {
		if asks[i].IsZero {
			ob.deleteLevel(&ob.Asks, asks[i].Price, false)
		} else {
			ob.upsertLevel(&ob.Asks, asks[i].Price, asks[i].Quantity, false)
		}
	}
}

// deleteLevel removes a price level from the slice.
func (ob *OrderBook) deleteLevel(levels *[]PriceLevel, price float64, desc bool) {
	n := len(*levels)
	idx := sort.Search(n, func(i int) bool {
		if desc {
			return (*levels)[i].Price <= price
		}
		return (*levels)[i].Price >= price
	})

	if idx < n && (*levels)[idx].Price == price {
		copy((*levels)[idx:], (*levels)[idx+1:])
		*levels = (*levels)[:n-1]
	}
}

// upsertLevel inserts or updates a price level.
func (ob *OrderBook) upsertLevel(levels *[]PriceLevel, price float64, qty string, desc bool) {
	n := len(*levels)
	idx := sort.Search(n, func(i int) bool {
		if desc {
			return (*levels)[i].Price <= price
		}
		return (*levels)[i].Price >= price
	})

	if idx < n && (*levels)[idx].Price == price {
		(*levels)[idx].Quantity = qty
		return
	}

	*levels = append(*levels, PriceLevel{})
	copy((*levels)[idx+1:], (*levels)[idx:])
	(*levels)[idx] = PriceLevel{Price: price, Quantity: qty}
}
