package parsing

import (
	"testing"
)

// Sample payload from Binance docs (direct event format)
var jsonPayload = []byte(`{
  "e": "depthUpdate",
  "E": 123456789,
  "s": "BNBBTC",
  "U": 157,
  "u": 160,
  "b": [
    ["0.0024", "10"],
	["0.0025", "11"],
	["0.0026", "12"],
	["0.0027", "13"],
	["0.0028", "14"]
  ],
  "a": [
    ["0.0026", "100"],
	["0.0027", "101"],
	["0.0028", "102"],
	["0.0029", "103"],
	["0.0030", "104"]
  ]
}`)

// Combined stream payload (wrapper format used with /stream?streams=...)
var combinedPayload = []byte(`{
  "stream": "bnbbtc@depth",
  "data": {
    "e": "depthUpdate",
    "E": 123456789,
    "s": "BNBBTC",
    "U": 157,
    "u": 160,
    "b": [
      ["0.0024", "10"],
      ["0.0025", "11"],
      ["0.0026", "12"],
      ["0.0027", "13"],
      ["0.0028", "14"]
    ],
    "a": [
      ["0.0026", "100"],
      ["0.0027", "101"],
      ["0.0028", "102"],
      ["0.0029", "103"],
      ["0.0030", "104"]
    ]
  }
}`)

func TestParseStandard(t *testing.T) {
	update, err := ParseStandard(jsonPayload)
	if err != nil {
		t.Fatalf("ParseStandard failed: %v", err)
	}
	assertParsedCorrectly(t, update)
}

func TestParseStandardCombined(t *testing.T) {
	update, err := ParseStandard(combinedPayload)
	if err != nil {
		t.Fatalf("ParseStandard (combined) failed: %v", err)
	}
	assertParsedCorrectly(t, update)
}

func TestParseFastJSON(t *testing.T) {
	update, err := ParseFastJSON(jsonPayload)
	if err != nil {
		t.Fatalf("ParseFastJSON failed: %v", err)
	}
	assertParsedCorrectly(t, update)
}

func TestParseFastJSONCombined(t *testing.T) {
	update, err := ParseFastJSON(combinedPayload)
	if err != nil {
		t.Fatalf("ParseFastJSON (combined) failed: %v", err)
	}
	assertParsedCorrectly(t, update)
}

func assertParsedCorrectly(t *testing.T, update *DepthUpdate) {
	t.Helper()
	if update.Event != "depthUpdate" {
		t.Errorf("Expected event 'depthUpdate', got '%s'", update.Event)
	}
	if update.Symbol != "BNBBTC" {
		t.Errorf("Expected symbol 'BNBBTC', got '%s'", update.Symbol)
	}
	if update.EventTime != 123456789 {
		t.Errorf("Expected EventTime 123456789, got %d", update.EventTime)
	}
	if len(update.Bids) != 5 {
		t.Errorf("Expected 5 bids, got %d", len(update.Bids))
	}
	if len(update.Asks) != 5 {
		t.Errorf("Expected 5 asks, got %d", len(update.Asks))
	}
	if update.Bids[0][0] != "0.0024" {
		t.Errorf("Expected first bid price '0.0024', got '%s'", update.Bids[0][0])
	}
}

// Benchmarks use the Direct parsers to compare raw parsing performance
// without the overhead of combined stream detection

func BenchmarkParseStandard(b *testing.B) {
	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		_, err := ParseStandardDirect(jsonPayload)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkParseFastJSON(b *testing.B) {
	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		_, err := ParseFastJSONDirect(jsonPayload)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// Also benchmark the combined stream format handling
func BenchmarkParseStandardCombined(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		_, err := ParseStandard(combinedPayload)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkParseFastJSONCombined(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		_, err := ParseFastJSON(combinedPayload)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// ============================================================================
// OPTIMIZED BENCHMARKS
// ============================================================================

// BenchmarkParseFastJSONPooled tests the pooled parser version
func BenchmarkParseFastJSONPooled(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		_, err := ParseFastJSONPooledDirect(jsonPayload)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkParseFastJSONPooledCombined tests the pooled parser with combined format
func BenchmarkParseFastJSONPooledCombined(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		_, err := ParseFastJSONPooled(combinedPayload)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkParseFastJSONZeroAlloc tests the fully pooled version
// Note: This benchmark properly releases the DepthUpdate back to pool
func BenchmarkParseFastJSONZeroAlloc(b *testing.B) {
	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		update, err := ParseFastJSONZeroAlloc(combinedPayload)
		if err != nil {
			b.Fatal(err)
		}
		ReleaseDepthUpdate(update)
	}
}

// BenchmarkParseFastJSONZeroAllocParallel tests concurrent parsing performance
func BenchmarkParseFastJSONZeroAllocParallel(b *testing.B) {
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			update, err := ParseFastJSONZeroAlloc(combinedPayload)
			if err != nil {
				b.Fatal(err)
			}
			ReleaseDepthUpdate(update)
		}
	})
}
