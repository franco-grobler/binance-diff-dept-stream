package binancestream

import (
	"encoding/json"
	"testing"
)

func TestUpdateSpeed_Constants(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		speed UpdateSpeed
		want  int
	}{
		{
			name:  "Second equals 1000",
			speed: Second,
			want:  1000,
		},
		{
			name:  "DeciSecond equals 100",
			speed: DeciSecond,
			want:  100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if int(tt.speed) != tt.want {
				t.Errorf("UpdateSpeed = %d, want %d", tt.speed, tt.want)
			}
		})
	}
}

func TestDiffStream_JSON_Unmarshalling(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		json    string
		want    DiffStream
		wantErr bool
	}{
		{
			name: "valid depth update",
			json: `{
				"e": "depthUpdate",
				"E": 1672515782136,
				"s": "BNBBTC",
				"U": 157,
				"u": 160,
				"b": [["0.0024", "10"]],
				"a": [["0.0026", "100"]]
			}`,
			want: DiffStream{
				EventType:     "depthUpdate",
				EventTime:     1672515782136,
				Symbol:        "BNBBTC",
				FirstUpdateID: 157,
				FinalUpdateID: 160,
				UpdateBids:    [][2]string{{"0.0024", "10"}},
				UpdateAsks:    [][2]string{{"0.0026", "100"}},
			},
			wantErr: false,
		},
		{
			name: "empty bids and asks",
			json: `{
				"e": "depthUpdate",
				"E": 1672515782136,
				"s": "ETHUSDT",
				"U": 100,
				"u": 105,
				"b": [],
				"a": []
			}`,
			want: DiffStream{
				EventType:     "depthUpdate",
				EventTime:     1672515782136,
				Symbol:        "ETHUSDT",
				FirstUpdateID: 100,
				FinalUpdateID: 105,
				UpdateBids:    [][2]string{},
				UpdateAsks:    [][2]string{},
			},
			wantErr: false,
		},
		{
			name: "multiple bids and asks",
			json: `{
				"e": "depthUpdate",
				"E": 1770591124002,
				"s": "BTCUSDT",
				"U": 87326945351,
				"u": 87326946514,
				"b": [
					["70906.83000000", "0.00000000"],
					["70905.45000000", "0.00000000"],
					["70902.92000000", "0.00189000"]
				],
				"a": [
					["70902.93000000", "0.77401000"],
					["70903.15000000", "0.00027000"]
				]
			}`,
			want: DiffStream{
				EventType:     "depthUpdate",
				EventTime:     1770591124002,
				Symbol:        "BTCUSDT",
				FirstUpdateID: 87326945351,
				FinalUpdateID: 87326946514,
				UpdateBids: [][2]string{
					{"70906.83000000", "0.00000000"},
					{"70905.45000000", "0.00000000"},
					{"70902.92000000", "0.00189000"},
				},
				UpdateAsks: [][2]string{
					{"70902.93000000", "0.77401000"},
					{"70903.15000000", "0.00027000"},
				},
			},
			wantErr: false,
		},
		{
			name:    "invalid JSON",
			json:    `{invalid`,
			want:    DiffStream{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var got DiffStream
			err := json.Unmarshal([]byte(tt.json), &got)

			if (err != nil) != tt.wantErr {
				t.Errorf("json.Unmarshal() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				return
			}

			if got.EventType != tt.want.EventType {
				t.Errorf("EventType = %s, want %s", got.EventType, tt.want.EventType)
			}
			if got.EventTime != tt.want.EventTime {
				t.Errorf("EventTime = %d, want %d", got.EventTime, tt.want.EventTime)
			}
			if got.Symbol != tt.want.Symbol {
				t.Errorf("Symbol = %s, want %s", got.Symbol, tt.want.Symbol)
			}
			if got.FirstUpdateID != tt.want.FirstUpdateID {
				t.Errorf("FirstUpdateID = %d, want %d", got.FirstUpdateID, tt.want.FirstUpdateID)
			}
			if got.FinalUpdateID != tt.want.FinalUpdateID {
				t.Errorf("FinalUpdateID = %d, want %d", got.FinalUpdateID, tt.want.FinalUpdateID)
			}
			if len(got.UpdateBids) != len(tt.want.UpdateBids) {
				t.Errorf(
					"UpdateBids length = %d, want %d",
					len(got.UpdateBids),
					len(tt.want.UpdateBids),
				)
			} else {
				for i, bid := range got.UpdateBids {
					if bid != tt.want.UpdateBids[i] {
						t.Errorf("UpdateBids[%d] = %v, want %v", i, bid, tt.want.UpdateBids[i])
					}
				}
			}
			if len(got.UpdateAsks) != len(tt.want.UpdateAsks) {
				t.Errorf(
					"UpdateAsks length = %d, want %d",
					len(got.UpdateAsks),
					len(tt.want.UpdateAsks),
				)
			} else {
				for i, ask := range got.UpdateAsks {
					if ask != tt.want.UpdateAsks[i] {
						t.Errorf("UpdateAsks[%d] = %v, want %v", i, ask, tt.want.UpdateAsks[i])
					}
				}
			}
		})
	}
}

func TestDiffStream_JSON_Marshalling(t *testing.T) {
	t.Parallel()

	ds := DiffStream{
		EventType:     "depthUpdate",
		EventTime:     1672515782136,
		Symbol:        "BTCUSDT",
		FirstUpdateID: 100,
		FinalUpdateID: 105,
		UpdateBids:    [][2]string{{"50000.00", "1.5"}},
		UpdateAsks:    [][2]string{{"50001.00", "2.0"}},
	}

	data, err := json.Marshal(ds)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	var got DiffStream
	err = json.Unmarshal(data, &got)
	if err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if got.EventType != ds.EventType {
		t.Errorf("round-trip EventType: got %s, want %s", got.EventType, ds.EventType)
	}
	if got.EventTime != ds.EventTime {
		t.Errorf("round-trip EventTime: got %d, want %d", got.EventTime, ds.EventTime)
	}
	if got.Symbol != ds.Symbol {
		t.Errorf("round-trip Symbol: got %s, want %s", got.Symbol, ds.Symbol)
	}
	if got.FirstUpdateID != ds.FirstUpdateID {
		t.Errorf("round-trip FirstUpdateID: got %d, want %d", got.FirstUpdateID, ds.FirstUpdateID)
	}
	if got.FinalUpdateID != ds.FinalUpdateID {
		t.Errorf("round-trip FinalUpdateID: got %d, want %d", got.FinalUpdateID, ds.FinalUpdateID)
	}
	if len(got.UpdateBids) != len(ds.UpdateBids) || got.UpdateBids[0] != ds.UpdateBids[0] {
		t.Errorf("round-trip UpdateBids mismatch")
	}
	if len(got.UpdateAsks) != len(ds.UpdateAsks) || got.UpdateAsks[0] != ds.UpdateAsks[0] {
		t.Errorf("round-trip UpdateAsks mismatch")
	}
}

func TestDiffSnapshot_JSON_Unmarshalling(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		json    string
		want    DiffSnapshot
		wantErr bool
	}{
		{
			name: "valid snapshot",
			json: `{
				"lastUpdateId": 4392930257,
				"bids": [
					["0.00907300", "5.79400000"],
					["0.00907200", "2.30000000"]
				],
				"asks": [
					["0.00907400", "3.22600000"],
					["0.00907500", "1.07500000"]
				]
			}`,
			want: DiffSnapshot{
				LastUpdateID: 4392930257,
				Bids: [][2]string{
					{"0.00907300", "5.79400000"},
					{"0.00907200", "2.30000000"},
				},
				Asks: [][2]string{
					{"0.00907400", "3.22600000"},
					{"0.00907500", "1.07500000"},
				},
			},
			wantErr: false,
		},
		{
			name: "empty bids and asks",
			json: `{
				"lastUpdateId": 100,
				"bids": [],
				"asks": []
			}`,
			want: DiffSnapshot{
				LastUpdateID: 100,
				Bids:         [][2]string{},
				Asks:         [][2]string{},
			},
			wantErr: false,
		},
		{
			name:    "invalid JSON",
			json:    `{invalid`,
			want:    DiffSnapshot{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var got DiffSnapshot
			err := json.Unmarshal([]byte(tt.json), &got)

			if (err != nil) != tt.wantErr {
				t.Errorf("json.Unmarshal() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				return
			}

			if got.LastUpdateID != tt.want.LastUpdateID {
				t.Errorf("LastUpdateID = %d, want %d", got.LastUpdateID, tt.want.LastUpdateID)
			}
			if len(got.Bids) != len(tt.want.Bids) {
				t.Errorf("Bids length = %d, want %d", len(got.Bids), len(tt.want.Bids))
			} else {
				for i, bid := range got.Bids {
					if bid != tt.want.Bids[i] {
						t.Errorf("Bids[%d] = %v, want %v", i, bid, tt.want.Bids[i])
					}
				}
			}
			if len(got.Asks) != len(tt.want.Asks) {
				t.Errorf("Asks length = %d, want %d", len(got.Asks), len(tt.want.Asks))
			} else {
				for i, ask := range got.Asks {
					if ask != tt.want.Asks[i] {
						t.Errorf("Asks[%d] = %v, want %v", i, ask, tt.want.Asks[i])
					}
				}
			}
		})
	}
}

func TestDiffSnapshot_JSON_Marshalling(t *testing.T) {
	t.Parallel()

	ds := DiffSnapshot{
		LastUpdateID: 12345,
		Bids:         [][2]string{{"50000.00", "1.5"}},
		Asks:         [][2]string{{"50001.00", "2.0"}},
	}

	data, err := json.Marshal(ds)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	var got DiffSnapshot
	err = json.Unmarshal(data, &got)
	if err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if got.LastUpdateID != ds.LastUpdateID {
		t.Errorf("round-trip LastUpdateID: got %d, want %d", got.LastUpdateID, ds.LastUpdateID)
	}
}

func TestParser_Type(t *testing.T) {
	t.Parallel()

	// Verify Parser can be used with json.Unmarshal
	var parser Parser = json.Unmarshal

	data := []byte(`{"e": "test"}`)
	var result DiffStream

	err := parser(data, &result)
	if err != nil {
		t.Errorf("Parser() error = %v", err)
	}

	if result.EventType != "test" {
		t.Errorf("Parser result.EventType = %s, want test", result.EventType)
	}
}

func TestDepthSnapshotEndpoint(t *testing.T) {
	t.Parallel()

	expected := "v3/depth"
	if DepthSnapshotEndpoint != expected {
		t.Errorf("DepthSnapshotEndpoint = %s, want %s", DepthSnapshotEndpoint, expected)
	}
}
