package orderbook

import (
	"strconv"
	"testing"
)

func TestOrderBook_Update(t *testing.T) {
	ob := NewOrderBook("BTCUSDT")

	// Initial State:
	// Asks: [101, 102]
	// Bids: [99, 98]
	initialBids := [][2]string{{"99.0", "1.0"}, {"98.0", "2.0"}}
	initialAsks := [][2]string{{"101.0", "1.0"}, {"102.0", "2.0"}}

	err := ob.Update(initialBids, initialAsks, 1)
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	// Verify Initial State
	if len(ob.Bids) != 2 || ob.Bids[0].Price != 99.0 {
		t.Errorf("Initial Bids incorrect: %v", ob.Bids)
	}
	if len(ob.Asks) != 2 || ob.Asks[0].Price != 101.0 {
		t.Errorf("Initial Asks incorrect: %v", ob.Asks)
	}

	// Scenario 1: Update existing quantity
	// Update Bid 99.0 to qty 5.0
	updateBids := [][2]string{{"99.0", "5.0"}}
	ob.Update(updateBids, nil, 2)
	if ob.Bids[0].Quantity != "5.0" {
		t.Errorf("Expected quantity update to 5.0, got %s", ob.Bids[0].Quantity)
	}

	// Scenario 2: Insert new level (middle)
	// Insert Bid 98.5
	// Bids should be: 99, 98.5, 98
	insertBids := [][2]string{{"98.5", "0.5"}}
	ob.Update(insertBids, nil, 3)
	if len(ob.Bids) != 3 || ob.Bids[1].Price != 98.5 {
		t.Errorf("Insert failed. Bids: %v", ob.Bids)
	}

	// Scenario 3: Delete level
	// Delete Bid 98.0 (qty 0)
	deleteBids := [][2]string{{"98.0", "0.00000000"}}
	ob.Update(deleteBids, nil, 4)
	if len(ob.Bids) != 2 {
		t.Errorf("Delete failed. Expected len 2, got %d", len(ob.Bids))
	}
	// Check remaining are 99 and 98.5
	if ob.Bids[0].Price != 99.0 || ob.Bids[1].Price != 98.5 {
		t.Errorf("Wrong bids remaining after delete: %v", ob.Bids)
	}
}

func TestOrderBook_Sorting(t *testing.T) {
	ob := NewOrderBook("ETHUSDT")

	// Random order inputs
	bids := [][2]string{
		{"100.0", "1"},
		{"105.0", "1"},
		{"102.0", "1"},
	}
	// Bids should be DESC: 105, 102, 100

	asks := [][2]string{
		{"200.0", "1"},
		{"195.0", "1"},
		{"205.0", "1"},
	}
	// Asks should be ASC: 195, 200, 205

	ob.Update(bids, asks, 1)

	// Verify Bids (High to Low)
	expectedBidPrices := []float64{105.0, 102.0, 100.0}
	for i, p := range expectedBidPrices {
		if ob.Bids[i].Price != p {
			t.Errorf("Bid at index %d: expected %f, got %f", i, p, ob.Bids[i].Price)
		}
	}

	// Verify Asks (Low to High)
	expectedAskPrices := []float64{195.0, 200.0, 205.0}
	for i, p := range expectedAskPrices {
		if ob.Asks[i].Price != p {
			t.Errorf("Ask at index %d: expected %f, got %f", i, p, ob.Asks[i].Price)
		}
	}
}

// ============================================================================
// BENCHMARKS
// ============================================================================

// generatePriceLevels creates test data for benchmarking
func generatePriceLevels(count int, basePrice float64) [][2]string {
	levels := make([][2]string, count)
	for i := 0; i < count; i++ {
		price := basePrice + float64(i)*0.01
		levels[i] = [2]string{
			formatFloat(price),
			"1.00000000",
		}
	}
	return levels
}

func formatFloat(f float64) string {
	return strconv.FormatFloat(f, 'f', 8, 64)
}

// BenchmarkOrderBook_Update_Small benchmarks updates with a small orderbook (10 levels)
func BenchmarkOrderBook_Update_Small(b *testing.B) {
	ob := NewOrderBook("BTCUSDT")
	bids := generatePriceLevels(5, 100.0)
	asks := generatePriceLevels(5, 101.0)

	// Pre-populate the orderbook
	ob.Update(bids, asks, 0)

	// Create update data (mix of insert, update, delete)
	updateBids := [][2]string{
		{"100.00000000", "2.0"}, // Update existing
		{"99.50000000", "1.0"},  // Insert new
		{"100.01000000", "0.0"}, // Delete
	}
	updateAsks := [][2]string{
		{"101.00000000", "2.0"}, // Update existing
		{"101.50000000", "1.0"}, // Insert new
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		ob.Update(updateBids, updateAsks, int64(i))
	}
}

// BenchmarkOrderBook_Update_Medium benchmarks updates with a medium orderbook (100 levels)
func BenchmarkOrderBook_Update_Medium(b *testing.B) {
	ob := NewOrderBook("BTCUSDT")
	bids := generatePriceLevels(50, 100.0)
	asks := generatePriceLevels(50, 150.0)

	// Pre-populate the orderbook
	ob.Update(bids, asks, 0)

	// Create update data
	updateBids := [][2]string{
		{"100.25000000", "2.0"}, // Update mid
		{"99.50000000", "1.0"},  // Insert at end
		{"100.50000000", "0.0"}, // Delete
	}
	updateAsks := [][2]string{
		{"150.25000000", "2.0"}, // Update mid
		{"155.00000000", "1.0"}, // Insert at end
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		ob.Update(updateBids, updateAsks, int64(i))
	}
}

// BenchmarkOrderBook_Update_Large benchmarks updates with a large orderbook (1000 levels)
func BenchmarkOrderBook_Update_Large(b *testing.B) {
	ob := NewOrderBook("BTCUSDT")
	bids := generatePriceLevels(500, 100.0)
	asks := generatePriceLevels(500, 200.0)

	// Pre-populate the orderbook
	ob.Update(bids, asks, 0)

	// Create update data
	updateBids := [][2]string{
		{"102.50000000", "2.0"}, // Update mid
		{"99.00000000", "1.0"},  // Insert at end
		{"103.00000000", "0.0"}, // Delete mid
	}
	updateAsks := [][2]string{
		{"202.50000000", "2.0"}, // Update mid
		{"300.00000000", "1.0"}, // Insert at end
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		ob.Update(updateBids, updateAsks, int64(i))
	}
}

// BenchmarkOrderBook_Update_WorstCase benchmarks worst-case insertion (always at beginning)
func BenchmarkOrderBook_Update_WorstCase(b *testing.B) {
	ob := NewOrderBook("BTCUSDT")
	bids := generatePriceLevels(500, 100.0)
	asks := generatePriceLevels(500, 200.0)

	// Pre-populate the orderbook
	ob.Update(bids, asks, 0)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		// Insert at beginning of bids (highest price) - worst case for copy
		newPrice := 200.0 + float64(i)*0.001
		updateBids := [][2]string{{formatFloat(newPrice), "1.0"}}
		ob.Update(updateBids, nil, int64(i))
	}
}

// BenchmarkOrderBook_Update_BulkInsert benchmarks inserting many levels at once
func BenchmarkOrderBook_Update_BulkInsert(b *testing.B) {
	bids := generatePriceLevels(50, 100.0)
	asks := generatePriceLevels(50, 150.0)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		ob := NewOrderBook("BTCUSDT")
		ob.Update(bids, asks, int64(i))
	}
}

// BenchmarkOrderBook_TopLevel benchmarks accessing the top level (best bid/ask)
func BenchmarkOrderBook_TopLevel(b *testing.B) {
	ob := NewOrderBook("BTCUSDT")
	bids := generatePriceLevels(500, 100.0)
	asks := generatePriceLevels(500, 200.0)
	ob.Update(bids, asks, 0)

	b.ResetTimer()
	b.ReportAllocs()
	var bestBid, bestAsk float64
	for i := 0; i < b.N; i++ {
		if len(ob.Bids) > 0 {
			bestBid = ob.Bids[0].Price
		}
		if len(ob.Asks) > 0 {
			bestAsk = ob.Asks[0].Price
		}
	}
	// Prevent compiler optimization
	_ = bestBid
	_ = bestAsk
}

// BenchmarkIsZeroQuantity benchmarks the optimized zero check vs ParseFloat
func BenchmarkIsZeroQuantity(b *testing.B) {
	quantities := []string{
		"0.00000000",
		"1.23456789",
		"0",
		"100.5",
		"0.0",
	}

	b.Run("Optimized", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			for _, q := range quantities {
				_ = isZeroQuantity(q)
			}
		}
	})

	b.Run("ParseFloat", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			for _, q := range quantities {
				if val, err := strconv.ParseFloat(q, 64); err == nil && val == 0 {
					_ = true
				} else {
					_ = false
				}
			}
		}
	})
}

// TestIsZeroQuantity verifies the isZeroQuantity function
func TestIsZeroQuantity(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"0", true},
		{"0.0", true},
		{"0.00000000", true},
		{"00.00", true},
		{"1.0", false},
		{"0.1", false},
		{"0.00000001", false},
		{"", false},
		{"100", false},
	}

	for _, tt := range tests {
		result := isZeroQuantity(tt.input)
		if result != tt.expected {
			t.Errorf("isZeroQuantity(%q) = %v, expected %v", tt.input, result, tt.expected)
		}
	}
}
