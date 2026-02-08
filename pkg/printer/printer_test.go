package printer

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestTopLevelPrices_JSON_Marshalling(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		data TopLevelPrices
		want string
	}{
		{
			name: "complete data",
			data: TopLevelPrices{
				Symbol:    "BTCUSDT",
				Ask:       50001.50,
				Bid:       50000.25,
				Timestamp: 1672515782136,
			},
			want: `{"symbol":"BTCUSDT","ask":50001.5,"bid":50000.25,"ts":1672515782136}`,
		},
		{
			name: "zero values",
			data: TopLevelPrices{
				Symbol:    "",
				Ask:       0,
				Bid:       0,
				Timestamp: 0,
			},
			want: `{"symbol":"","ask":0,"bid":0,"ts":0}`,
		},
		{
			name: "ETHUSDT data",
			data: TopLevelPrices{
				Symbol:    "ETHUSDT",
				Ask:       3000.00,
				Bid:       2999.99,
				Timestamp: 1770591124002,
			},
			want: `{"symbol":"ETHUSDT","ask":3000,"bid":2999.99,"ts":1770591124002}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := json.Marshal(tt.data)
			if err != nil {
				t.Fatalf("json.Marshal() error = %v", err)
			}

			if string(got) != tt.want {
				t.Errorf("json.Marshal() = %s, want %s", string(got), tt.want)
			}
		})
	}
}

func TestTopLevelPrices_JSON_Unmarshalling(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		json    string
		want    TopLevelPrices
		wantErr bool
	}{
		{
			name: "valid JSON",
			json: `{"symbol":"BTCUSDT","ask":50001.5,"bid":50000.25,"ts":1672515782136}`,
			want: TopLevelPrices{
				Symbol:    "BTCUSDT",
				Ask:       50001.5,
				Bid:       50000.25,
				Timestamp: 1672515782136,
			},
			wantErr: false,
		},
		{
			name:    "invalid JSON",
			json:    `{invalid`,
			want:    TopLevelPrices{},
			wantErr: true,
		},
		{
			name: "partial JSON with missing fields",
			json: `{"symbol":"ETHUSDT"}`,
			want: TopLevelPrices{
				Symbol:    "ETHUSDT",
				Ask:       0,
				Bid:       0,
				Timestamp: 0,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var got TopLevelPrices
			err := json.Unmarshal([]byte(tt.json), &got)

			if (err != nil) != tt.wantErr {
				t.Errorf("json.Unmarshal() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				return
			}

			if got.Symbol != tt.want.Symbol {
				t.Errorf("Symbol = %s, want %s", got.Symbol, tt.want.Symbol)
			}
			if got.Ask != tt.want.Ask {
				t.Errorf("Ask = %f, want %f", got.Ask, tt.want.Ask)
			}
			if got.Bid != tt.want.Bid {
				t.Errorf("Bid = %f, want %f", got.Bid, tt.want.Bid)
			}
			if got.Timestamp != tt.want.Timestamp {
				t.Errorf("Timestamp = %d, want %d", got.Timestamp, tt.want.Timestamp)
			}
		})
	}
}

func TestCurrency_Type(t *testing.T) {
	t.Parallel()

	// Verify Currency is float64
	var c Currency = 50000.50
	f := c

	if f != 50000.50 {
		t.Errorf("Currency should be float64, got %f", f)
	}
}

func TestStdout_ImplementsPrinter(t *testing.T) {
	t.Parallel()

	// Compile-time check that Stdout implements Printer interface
	var _ Printer = (*Stdout)(nil)
}

// captureStdout captures stdout during a function execution.
// Must be run sequentially (not parallel) to avoid conflicts.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}

	os.Stdout = w

	// Channel to capture output
	outputCh := make(chan string)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		outputCh <- buf.String()
	}()

	fn()

	w.Close()
	os.Stdout = oldStdout

	return <-outputCh
}

func TestStdout_Write(t *testing.T) {
	// NOT parallel - we're capturing stdout
	tests := []struct {
		name     string
		data     []any
		expected string
	}{
		{
			name:     "write string",
			data:     []any{"test string"},
			expected: "test string",
		},
		{
			name:     "write multiple values",
			data:     []any{"hello", " ", "world"},
			expected: "hello world",
		},
		{
			name:     "write number",
			data:     []any{12345},
			expected: "12345",
		},
		{
			name:     "write empty",
			data:     []any{},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// NOT parallel - we're capturing stdout
			s := &Stdout{}

			output := captureStdout(t, func() {
				err := s.Write(tt.data...)
				if err != nil {
					t.Errorf("Write() error = %v", err)
				}
			})

			if output != tt.expected {
				t.Errorf("Write() output = %q, want %q", output, tt.expected)
			}
		})
	}
}

func TestStdout_Write_Error(t *testing.T) {
	// Test that Write returns nil error in normal operation
	s := &Stdout{}

	output := captureStdout(t, func() {
		err := s.Write("test")
		if err != nil {
			t.Errorf("Write() should not return error, got %v", err)
		}
	})

	if output != "test" {
		t.Errorf("unexpected output: %q", output)
	}
}

func TestStdout_PriceWriter(t *testing.T) {
	// NOT parallel - we're capturing stdout
	tests := []struct {
		name   string
		prices []TopLevelPrices
		want   []string
	}{
		{
			name: "single price",
			prices: []TopLevelPrices{
				{Symbol: "BTCUSDT", Ask: 50001.5, Bid: 50000.25, Timestamp: 1672515782136},
			},
			want: []string{"BTCUSDT", "50001.5", "50000.25"},
		},
		{
			name: "multiple prices",
			prices: []TopLevelPrices{
				{Symbol: "BTCUSDT", Ask: 50001.5, Bid: 50000.25, Timestamp: 1672515782136},
				{Symbol: "ETHUSDT", Ask: 3000.00, Bid: 2999.99, Timestamp: 1672515782137},
			},
			want: []string{"BTCUSDT", "ETHUSDT"},
		},
		{
			name:   "empty prices",
			prices: []TopLevelPrices{},
			want:   []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// NOT parallel - we're capturing stdout
			s := &Stdout{}

			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()

			printCh := make(chan TopLevelPrices, len(tt.prices)+1)

			// Send prices
			for _, p := range tt.prices {
				printCh <- p
			}
			close(printCh)

			output := captureStdout(t, func() {
				s.PriceWriter(ctx, printCh)
			})

			for _, want := range tt.want {
				if !strings.Contains(output, want) {
					t.Errorf("output should contain %s, got %s", want, output)
				}
			}
		})
	}
}

func TestStdout_PriceWriter_ContextCancellation(t *testing.T) {
	// NOT parallel - we're capturing stdout
	ctx, cancel := context.WithCancel(context.Background())
	printCh := make(chan TopLevelPrices, 10)

	s := &Stdout{}

	done := make(chan struct{})

	output := captureStdout(t, func() {
		go func() {
			s.PriceWriter(ctx, printCh)
			close(done)
		}()

		// Cancel context
		cancel()

		// Wait for goroutine to finish
		select {
		case <-done:
			// Success - PriceWriter exited
		case <-time.After(time.Second):
			t.Error("PriceWriter did not exit after context cancellation")
		}
	})

	// Should be empty - no prices were sent
	if output != "" {
		t.Errorf("expected empty output, got %q", output)
	}
}

func TestStdout_PriceWriter_ChannelClose(t *testing.T) {
	// NOT parallel - we're capturing stdout
	ctx := context.Background()
	printCh := make(chan TopLevelPrices, 10)

	s := &Stdout{}

	done := make(chan struct{})

	output := captureStdout(t, func() {
		go func() {
			s.PriceWriter(ctx, printCh)
			close(done)
		}()

		// Close channel immediately
		close(printCh)

		// Wait for goroutine to finish
		select {
		case <-done:
			// Success - PriceWriter exited
		case <-time.After(time.Second):
			t.Error("PriceWriter did not exit after channel close")
		}
	})

	// Should be empty - no prices were sent
	if output != "" {
		t.Errorf("expected empty output, got %q", output)
	}
}

func TestStdout_PriceWriter_OutputFormat(t *testing.T) {
	// NOT parallel - we're capturing stdout
	s := &Stdout{}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	printCh := make(chan TopLevelPrices, 1)
	printCh <- TopLevelPrices{
		Symbol:    "BTCUSDT",
		Ask:       50001.5,
		Bid:       50000.25,
		Timestamp: 1672515782136,
	}
	close(printCh)

	output := captureStdout(t, func() {
		s.PriceWriter(ctx, printCh)
	})

	// Verify output is valid JSON with newline
	if !strings.HasSuffix(output, "\n") {
		t.Error("output should end with newline")
	}

	// Parse the JSON (minus the newline)
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) < 1 {
		t.Fatal("expected at least one line of output")
	}

	var parsed TopLevelPrices
	err := json.Unmarshal([]byte(lines[0]), &parsed)
	if err != nil {
		t.Errorf("output is not valid JSON: %v, output: %s", err, lines[0])
	}

	if parsed.Symbol != "BTCUSDT" {
		t.Errorf("parsed.Symbol = %s, want BTCUSDT", parsed.Symbol)
	}
}

func TestPrinter_Interface(t *testing.T) {
	t.Parallel()

	// Verify Printer interface methods exist
	type printerChecker interface {
		Write(data ...any) error
		PriceWriter(ctx context.Context, readCh <-chan TopLevelPrices)
	}

	var _ printerChecker = (*Stdout)(nil)
}

// MockWriter is a test helper to capture output.
type MockWriter struct {
	buf bytes.Buffer
	mu  sync.Mutex
}

func (m *MockWriter) Write(p []byte) (n int, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.buf.Write(p)
}

func (m *MockWriter) String() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.buf.String()
}

func TestStdout_PriceWriter_ConcurrentWrites(t *testing.T) {
	// NOT parallel - we're capturing stdout
	s := &Stdout{}

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	printCh := make(chan TopLevelPrices, 100)

	// Pre-populate channel with prices
	for i := 0; i < 50; i++ {
		printCh <- TopLevelPrices{
			Symbol:    "SYMBOL",
			Ask:       float64(i) + 0.5,
			Bid:       float64(i),
			Timestamp: uint64(i),
		}
	}
	close(printCh)

	output := captureStdout(t, func() {
		s.PriceWriter(ctx, printCh)
	})

	lines := strings.Split(strings.TrimSpace(output), "\n")

	if len(lines) != 50 {
		t.Errorf("expected 50 lines, got %d", len(lines))
	}
}

func BenchmarkTopLevelPrices_Marshal(b *testing.B) {
	data := TopLevelPrices{
		Symbol:    "BTCUSDT",
		Ask:       50001.5,
		Bid:       50000.25,
		Timestamp: 1672515782136,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = json.Marshal(data)
	}
}

func BenchmarkStdout_Write(b *testing.B) {
	// Redirect stdout to discard for benchmark
	oldStdout := os.Stdout
	null, _ := os.Open(os.DevNull)
	os.Stdout = null
	defer func() {
		os.Stdout = oldStdout
		null.Close()
	}()

	s := &Stdout{}
	data := "test benchmark data"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = s.Write(data)
	}
}
