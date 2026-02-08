package orderbook

import (
	"testing"
)

func TestNewOrderBook(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		symbol string
	}{
		{
			name:   "creates order book with BTCUSDT symbol",
			symbol: "BTCUSDT",
		},
		{
			name:   "creates order book with ETHUSDT symbol",
			symbol: "ETHUSDT",
		},
		{
			name:   "creates order book with empty symbol",
			symbol: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ob := NewOrderBook(tt.symbol)

			if ob == nil {
				t.Fatal("expected non-nil OrderBook")
			}
			if ob.Symbol != tt.symbol {
				t.Errorf("got symbol %q, want %q", ob.Symbol, tt.symbol)
			}
			if ob.Bids == nil {
				t.Error("expected non-nil Bids slice")
			}
			if ob.Asks == nil {
				t.Error("expected non-nil Asks slice")
			}
			if len(ob.Bids) != 0 {
				t.Errorf("expected empty Bids, got %d", len(ob.Bids))
			}
			if len(ob.Asks) != 0 {
				t.Errorf("expected empty Asks, got %d", len(ob.Asks))
			}
			if cap(ob.Bids) != DefaultCapacity {
				t.Errorf("expected Bids capacity %d, got %d", DefaultCapacity, cap(ob.Bids))
			}
			if cap(ob.Asks) != DefaultCapacity {
				t.Errorf("expected Asks capacity %d, got %d", DefaultCapacity, cap(ob.Asks))
			}
			if ob.LastUpdateID != 0 {
				t.Errorf("expected LastUpdateID 0, got %d", ob.LastUpdateID)
			}
		})
	}
}

func TestOrderBook_Update(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		initialBids  [][2]string
		initialAsks  [][2]string
		initialID    int64
		updateBids   [][2]string
		updateAsks   [][2]string
		updateID     int64
		wantBids     []PriceLevel
		wantAsks     []PriceLevel
		wantUpdateID int64
		wantErr      bool
	}{
		{
			name:        "insert new bid and ask levels",
			initialBids: nil,
			initialAsks: nil,
			initialID:   0,
			updateBids: [][2]string{
				{"100.50", "10.5"},
				{"100.25", "5.0"},
				{"100.75", "7.5"},
			},
			updateAsks: [][2]string{
				{"101.00", "8.0"},
				{"101.25", "3.5"},
				{"100.90", "6.0"},
			},
			updateID: 12345,
			wantBids: []PriceLevel{
				{Price: 100.75, Quantity: "7.5"},
				{Price: 100.50, Quantity: "10.5"},
				{Price: 100.25, Quantity: "5.0"},
			},
			wantAsks: []PriceLevel{
				{Price: 100.90, Quantity: "6.0"},
				{Price: 101.00, Quantity: "8.0"},
				{Price: 101.25, Quantity: "3.5"},
			},
			wantUpdateID: 12345,
			wantErr:      false,
		},
		{
			name: "update existing bid and ask levels",
			initialBids: [][2]string{
				{"100.50", "10.5"},
				{"100.25", "5.0"},
			},
			initialAsks: [][2]string{
				{"101.00", "8.0"},
				{"101.25", "3.5"},
			},
			initialID: 100,
			updateBids: [][2]string{
				{"100.50", "20.0"},
			},
			updateAsks: [][2]string{
				{"101.00", "15.0"},
			},
			updateID: 101,
			wantBids: []PriceLevel{
				{Price: 100.50, Quantity: "20.0"},
				{Price: 100.25, Quantity: "5.0"},
			},
			wantAsks: []PriceLevel{
				{Price: 101.00, Quantity: "15.0"},
				{Price: 101.25, Quantity: "3.5"},
			},
			wantUpdateID: 101,
			wantErr:      false,
		},
		{
			name: "delete bid and ask levels with zero quantity",
			initialBids: [][2]string{
				{"100.50", "10.5"},
				{"100.25", "5.0"},
				{"100.00", "3.0"},
			},
			initialAsks: [][2]string{
				{"101.00", "8.0"},
				{"101.25", "3.5"},
				{"101.50", "2.0"},
			},
			initialID: 200,
			updateBids: [][2]string{
				{"100.25", "0.00000000"},
			},
			updateAsks: [][2]string{
				{"101.25", "0"},
			},
			updateID: 201,
			wantBids: []PriceLevel{
				{Price: 100.50, Quantity: "10.5"},
				{Price: 100.00, Quantity: "3.0"},
			},
			wantAsks: []PriceLevel{
				{Price: 101.00, Quantity: "8.0"},
				{Price: 101.50, Quantity: "2.0"},
			},
			wantUpdateID: 201,
			wantErr:      false,
		},
		{
			name:        "delete non-existent level does nothing",
			initialBids: [][2]string{{"100.50", "10.5"}},
			initialAsks: [][2]string{{"101.00", "8.0"}},
			initialID:   300,
			updateBids: [][2]string{
				{"99.99", "0.0"},
			},
			updateAsks: [][2]string{
				{"102.00", "0.00"},
			},
			updateID: 301,
			wantBids: []PriceLevel{
				{Price: 100.50, Quantity: "10.5"},
			},
			wantAsks: []PriceLevel{
				{Price: 101.00, Quantity: "8.0"},
			},
			wantUpdateID: 301,
			wantErr:      false,
		},
		{
			name:        "invalid bid price returns error",
			initialBids: nil,
			initialAsks: nil,
			initialID:   0,
			updateBids: [][2]string{
				{"invalid", "10.5"},
			},
			updateAsks:   nil,
			updateID:     400,
			wantErr:      true,
			wantUpdateID: 400,
		},
		{
			name:        "invalid ask price returns error",
			initialBids: nil,
			initialAsks: nil,
			initialID:   0,
			updateBids:  nil,
			updateAsks: [][2]string{
				{"not-a-number", "8.0"},
			},
			updateID:     500,
			wantErr:      true,
			wantUpdateID: 500,
		},
		{
			name:         "empty updates",
			initialBids:  [][2]string{{"100.50", "10.5"}},
			initialAsks:  [][2]string{{"101.00", "8.0"}},
			initialID:    600,
			updateBids:   nil,
			updateAsks:   nil,
			updateID:     601,
			wantBids:     []PriceLevel{{Price: 100.50, Quantity: "10.5"}},
			wantAsks:     []PriceLevel{{Price: 101.00, Quantity: "8.0"}},
			wantUpdateID: 601,
			wantErr:      false,
		},
		{
			name:        "mixed operations - insert, update, delete",
			initialBids: [][2]string{{"100.50", "10.5"}, {"100.25", "5.0"}},
			initialAsks: [][2]string{{"101.00", "8.0"}, {"101.25", "3.5"}},
			initialID:   700,
			updateBids: [][2]string{
				{"100.75", "12.0"}, // insert
				{"100.50", "15.0"}, // update
				{"100.25", "0.0"},  // delete
				{"100.00", "0.0"},  // delete non-existent
				{"99.75", "2.5"},   // insert
			},
			updateAsks: [][2]string{
				{"100.90", "7.0"},        // insert
				{"101.00", "10.0"},       // update
				{"101.25", "0.00000000"}, // delete
				{"101.50", "4.0"},        // insert
			},
			updateID: 701,
			wantBids: []PriceLevel{
				{Price: 100.75, Quantity: "12.0"},
				{Price: 100.50, Quantity: "15.0"},
				{Price: 99.75, Quantity: "2.5"},
			},
			wantAsks: []PriceLevel{
				{Price: 100.90, Quantity: "7.0"},
				{Price: 101.00, Quantity: "10.0"},
				{Price: 101.50, Quantity: "4.0"},
			},
			wantUpdateID: 701,
			wantErr:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ob := NewOrderBook("BTCUSDT")

			// Setup initial state
			if tt.initialBids != nil || tt.initialAsks != nil {
				err := ob.Update(tt.initialBids, tt.initialAsks, tt.initialID)
				if err != nil {
					t.Fatalf("failed to set up initial state: %v", err)
				}
			}

			// Perform the update
			err := ob.Update(tt.updateBids, tt.updateAsks, tt.updateID)

			// Check error
			if (err != nil) != tt.wantErr {
				t.Errorf("Update() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				return
			}

			// Verify UpdateID
			if ob.LastUpdateID != tt.wantUpdateID {
				t.Errorf("LastUpdateID = %d, want %d", ob.LastUpdateID, tt.wantUpdateID)
			}

			// Verify Bids
			if len(ob.Bids) != len(tt.wantBids) {
				t.Errorf("Bids length = %d, want %d", len(ob.Bids), len(tt.wantBids))
			} else {
				for i, want := range tt.wantBids {
					if ob.Bids[i].Price != want.Price {
						t.Errorf("Bids[%d].Price = %f, want %f", i, ob.Bids[i].Price, want.Price)
					}
					if ob.Bids[i].Quantity != want.Quantity {
						t.Errorf(
							"Bids[%d].Quantity = %s, want %s",
							i,
							ob.Bids[i].Quantity,
							want.Quantity,
						)
					}
				}
			}

			// Verify Asks
			if len(ob.Asks) != len(tt.wantAsks) {
				t.Errorf("Asks length = %d, want %d", len(ob.Asks), len(tt.wantAsks))
			} else {
				for i, want := range tt.wantAsks {
					if ob.Asks[i].Price != want.Price {
						t.Errorf("Asks[%d].Price = %f, want %f", i, ob.Asks[i].Price, want.Price)
					}
					if ob.Asks[i].Quantity != want.Quantity {
						t.Errorf(
							"Asks[%d].Quantity = %s, want %s",
							i,
							ob.Asks[i].Quantity,
							want.Quantity,
						)
					}
				}
			}

			// Verify bid ordering (descending by price)
			for i := 1; i < len(ob.Bids); i++ {
				if ob.Bids[i-1].Price < ob.Bids[i].Price {
					t.Errorf("Bids not sorted DESC: Bids[%d].Price = %f < Bids[%d].Price = %f",
						i-1, ob.Bids[i-1].Price, i, ob.Bids[i].Price)
				}
			}

			// Verify ask ordering (ascending by price)
			for i := 1; i < len(ob.Asks); i++ {
				if ob.Asks[i-1].Price > ob.Asks[i].Price {
					t.Errorf("Asks not sorted ASC: Asks[%d].Price = %f > Asks[%d].Price = %f",
						i-1, ob.Asks[i-1].Price, i, ob.Asks[i].Price)
				}
			}
		})
	}
}

func TestIsZeroQuantity(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		quantity string
		want     bool
	}{
		{name: "zero", quantity: "0", want: true},
		{name: "zero with decimal", quantity: "0.0", want: true},
		{name: "zero with many decimals", quantity: "0.00000000", want: true},
		{name: "leading zeros", quantity: "00.00", want: true},
		{name: "just dots", quantity: "...", want: true},
		{name: "non-zero integer", quantity: "1", want: false},
		{name: "non-zero decimal", quantity: "0.1", want: false},
		{name: "non-zero with trailing zeros", quantity: "0.10000000", want: false},
		{name: "large number", quantity: "12345.67890", want: false},
		{name: "empty string", quantity: "", want: false},
		{name: "contains letter", quantity: "0a0", want: false},
		{name: "scientific notation", quantity: "0e0", want: false},
		{name: "negative zero", quantity: "-0", want: false},
		{name: "space", quantity: " ", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := isZeroQuantity(tt.quantity)
			if got != tt.want {
				t.Errorf("isZeroQuantity(%q) = %v, want %v", tt.quantity, got, tt.want)
			}
		})
	}
}

func TestOrderBook_UpdateLevel_EdgeCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		initialLevels  []PriceLevel
		price          float64
		quantity       string
		desc           bool
		expectedLevels []PriceLevel
	}{
		{
			name:           "insert into empty slice - ascending",
			initialLevels:  []PriceLevel{},
			price:          100.0,
			quantity:       "5.0",
			desc:           false,
			expectedLevels: []PriceLevel{{Price: 100.0, Quantity: "5.0"}},
		},
		{
			name:           "insert into empty slice - descending",
			initialLevels:  []PriceLevel{},
			price:          100.0,
			quantity:       "5.0",
			desc:           true,
			expectedLevels: []PriceLevel{{Price: 100.0, Quantity: "5.0"}},
		},
		{
			name:          "insert at beginning - ascending",
			initialLevels: []PriceLevel{{Price: 100.0, Quantity: "5.0"}},
			price:         99.0,
			quantity:      "3.0",
			desc:          false,
			expectedLevels: []PriceLevel{
				{Price: 99.0, Quantity: "3.0"},
				{Price: 100.0, Quantity: "5.0"},
			},
		},
		{
			name:          "insert at beginning - descending",
			initialLevels: []PriceLevel{{Price: 100.0, Quantity: "5.0"}},
			price:         101.0,
			quantity:      "3.0",
			desc:          true,
			expectedLevels: []PriceLevel{
				{Price: 101.0, Quantity: "3.0"},
				{Price: 100.0, Quantity: "5.0"},
			},
		},
		{
			name:          "insert at end - ascending",
			initialLevels: []PriceLevel{{Price: 100.0, Quantity: "5.0"}},
			price:         101.0,
			quantity:      "3.0",
			desc:          false,
			expectedLevels: []PriceLevel{
				{Price: 100.0, Quantity: "5.0"},
				{Price: 101.0, Quantity: "3.0"},
			},
		},
		{
			name:          "insert at end - descending",
			initialLevels: []PriceLevel{{Price: 100.0, Quantity: "5.0"}},
			price:         99.0,
			quantity:      "3.0",
			desc:          true,
			expectedLevels: []PriceLevel{
				{Price: 100.0, Quantity: "5.0"},
				{Price: 99.0, Quantity: "3.0"},
			},
		},
		{
			name: "insert in middle - ascending",
			initialLevels: []PriceLevel{
				{Price: 99.0, Quantity: "2.0"},
				{Price: 101.0, Quantity: "4.0"},
			},
			price:    100.0,
			quantity: "3.0",
			desc:     false,
			expectedLevels: []PriceLevel{
				{Price: 99.0, Quantity: "2.0"},
				{Price: 100.0, Quantity: "3.0"},
				{Price: 101.0, Quantity: "4.0"},
			},
		},
		{
			name: "insert in middle - descending",
			initialLevels: []PriceLevel{
				{Price: 101.0, Quantity: "4.0"},
				{Price: 99.0, Quantity: "2.0"},
			},
			price:    100.0,
			quantity: "3.0",
			desc:     true,
			expectedLevels: []PriceLevel{
				{Price: 101.0, Quantity: "4.0"},
				{Price: 100.0, Quantity: "3.0"},
				{Price: 99.0, Quantity: "2.0"},
			},
		},
		{
			name:           "update existing level",
			initialLevels:  []PriceLevel{{Price: 100.0, Quantity: "5.0"}},
			price:          100.0,
			quantity:       "10.0",
			desc:           false,
			expectedLevels: []PriceLevel{{Price: 100.0, Quantity: "10.0"}},
		},
		{
			name:           "delete only level",
			initialLevels:  []PriceLevel{{Price: 100.0, Quantity: "5.0"}},
			price:          100.0,
			quantity:       "0",
			desc:           false,
			expectedLevels: []PriceLevel{},
		},
		{
			name: "delete first level - ascending",
			initialLevels: []PriceLevel{
				{Price: 99.0, Quantity: "2.0"},
				{Price: 100.0, Quantity: "3.0"},
				{Price: 101.0, Quantity: "4.0"},
			},
			price:    99.0,
			quantity: "0.0",
			desc:     false,
			expectedLevels: []PriceLevel{
				{Price: 100.0, Quantity: "3.0"},
				{Price: 101.0, Quantity: "4.0"},
			},
		},
		{
			name: "delete last level - ascending",
			initialLevels: []PriceLevel{
				{Price: 99.0, Quantity: "2.0"},
				{Price: 100.0, Quantity: "3.0"},
				{Price: 101.0, Quantity: "4.0"},
			},
			price:    101.0,
			quantity: "0.00000000",
			desc:     false,
			expectedLevels: []PriceLevel{
				{Price: 99.0, Quantity: "2.0"},
				{Price: 100.0, Quantity: "3.0"},
			},
		},
		{
			name: "delete middle level - ascending",
			initialLevels: []PriceLevel{
				{Price: 99.0, Quantity: "2.0"},
				{Price: 100.0, Quantity: "3.0"},
				{Price: 101.0, Quantity: "4.0"},
			},
			price:    100.0,
			quantity: "0",
			desc:     false,
			expectedLevels: []PriceLevel{
				{Price: 99.0, Quantity: "2.0"},
				{Price: 101.0, Quantity: "4.0"},
			},
		},
		{
			name:          "delete non-existent level does nothing",
			initialLevels: []PriceLevel{{Price: 100.0, Quantity: "5.0"}},
			price:         99.0,
			quantity:      "0",
			desc:          false,
			expectedLevels: []PriceLevel{
				{Price: 100.0, Quantity: "5.0"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ob := NewOrderBook("TEST")

			// Copy initial levels
			levels := make([]PriceLevel, len(tt.initialLevels))
			copy(levels, tt.initialLevels)

			ob.updateLevel(&levels, tt.price, tt.quantity, tt.desc)

			if len(levels) != len(tt.expectedLevels) {
				t.Errorf("levels length = %d, want %d", len(levels), len(tt.expectedLevels))
				return
			}

			for i, want := range tt.expectedLevels {
				if levels[i].Price != want.Price {
					t.Errorf("levels[%d].Price = %f, want %f", i, levels[i].Price, want.Price)
				}
				if levels[i].Quantity != want.Quantity {
					t.Errorf(
						"levels[%d].Quantity = %s, want %s",
						i,
						levels[i].Quantity,
						want.Quantity,
					)
				}
			}
		})
	}
}

func TestOrderBook_LargeDataset(t *testing.T) {
	t.Parallel()

	ob := NewOrderBook("BTCUSDT")

	// Create a large dataset
	numLevels := 1000
	bids := make([][2]string, numLevels)
	asks := make([][2]string, numLevels)

	for i := 0; i < numLevels; i++ {
		bidPrice := 50000.0 - float64(i)
		askPrice := 50001.0 + float64(i)
		bids[i] = [2]string{formatFloat(bidPrice), "1.5"}
		asks[i] = [2]string{formatFloat(askPrice), "1.5"}
	}

	err := ob.Update(bids, asks, 1)
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	if len(ob.Bids) != numLevels {
		t.Errorf("Bids length = %d, want %d", len(ob.Bids), numLevels)
	}
	if len(ob.Asks) != numLevels {
		t.Errorf("Asks length = %d, want %d", len(ob.Asks), numLevels)
	}

	// Verify bids are sorted descending
	for i := 1; i < len(ob.Bids); i++ {
		if ob.Bids[i-1].Price < ob.Bids[i].Price {
			t.Errorf("Bids not sorted DESC at index %d", i)
			break
		}
	}

	// Verify asks are sorted ascending
	for i := 1; i < len(ob.Asks); i++ {
		if ob.Asks[i-1].Price > ob.Asks[i].Price {
			t.Errorf("Asks not sorted ASC at index %d", i)
			break
		}
	}

	// Best bid should be highest
	if ob.Bids[0].Price != 50000.0 {
		t.Errorf("Best bid = %f, want 50000.0", ob.Bids[0].Price)
	}

	// Best ask should be lowest
	if ob.Asks[0].Price != 50001.0 {
		t.Errorf("Best ask = %f, want 50001.0", ob.Asks[0].Price)
	}
}

func formatFloat(f float64) string {
	return string([]byte{
		byte('0' + int(f)/10000%10),
		byte('0' + int(f)/1000%10),
		byte('0' + int(f)/100%10),
		byte('0' + int(f)/10%10),
		byte('0' + int(f)%10),
		'.',
		'0',
	})
}

func TestDefaultCapacity(t *testing.T) {
	t.Parallel()

	if DefaultCapacity != 5000 {
		t.Errorf("DefaultCapacity = %d, want 5000", DefaultCapacity)
	}
}

func BenchmarkOrderBook_Update(b *testing.B) {
	ob := NewOrderBook("BTCUSDT")

	// Pre-populate with data
	initial := make([][2]string, 1000)
	for i := 0; i < 1000; i++ {
		initial[i] = [2]string{formatFloat(50000.0 + float64(i)), "1.5"}
	}
	_ = ob.Update(initial, initial, 0)

	// Prepare update data
	updates := make([][2]string, 100)
	for i := 0; i < 100; i++ {
		updates[i] = [2]string{formatFloat(50000.0 + float64(i*2)), "2.5"}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ob.Update(updates, updates, int64(i))
	}
}

func BenchmarkIsZeroQuantity(b *testing.B) {
	testCases := []string{"0", "0.00000000", "1.5", "0.1", "100.50000000"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, tc := range testCases {
			isZeroQuantity(tc)
		}
	}
}
