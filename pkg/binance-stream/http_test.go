package binancestream

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewHTTPClient(t *testing.T) {
	t.Parallel()

	client := NewHTTPClient()

	if client == nil {
		t.Fatal("expected non-nil http.Client")
	}

	// Verify it's a standard http.Client
	_, ok := any(client).(*http.Client)
	if !ok {
		t.Error("expected *http.Client type")
	}
}

func TestHTTPClient_Interface(t *testing.T) {
	t.Parallel()

	// Verify http.Client implements HTTPClient interface
	var _ HTTPClient = (*http.Client)(nil)
}

func TestHTTPClient_Do(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		serverResponse string
		serverStatus   int
		wantStatus     int
		wantBody       string
	}{
		{
			name:           "successful GET request",
			serverResponse: `{"result": "success"}`,
			serverStatus:   http.StatusOK,
			wantStatus:     http.StatusOK,
			wantBody:       `{"result": "success"}`,
		},
		{
			name:           "server returns 500",
			serverResponse: `{"error": "internal server error"}`,
			serverStatus:   http.StatusInternalServerError,
			wantStatus:     http.StatusInternalServerError,
			wantBody:       `{"error": "internal server error"}`,
		},
		{
			name:           "server returns 400",
			serverResponse: `{"error": "bad request"}`,
			serverStatus:   http.StatusBadRequest,
			wantStatus:     http.StatusBadRequest,
			wantBody:       `{"error": "bad request"}`,
		},
		{
			name:           "empty response",
			serverResponse: "",
			serverStatus:   http.StatusNoContent,
			wantStatus:     http.StatusNoContent,
			wantBody:       "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			server := httptest.NewServer(
				http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(tt.serverStatus)
					_, _ = w.Write([]byte(tt.serverResponse))
				}),
			)
			defer server.Close()

			client := NewHTTPClient()

			req, err := http.NewRequest("GET", server.URL, nil)
			if err != nil {
				t.Fatalf("failed to create request: %v", err)
			}

			resp, err := client.Do(req)
			if err != nil {
				t.Fatalf("Do() error = %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != tt.wantStatus {
				t.Errorf("StatusCode = %d, want %d", resp.StatusCode, tt.wantStatus)
			}
		})
	}
}

func TestHTTPClient_DoWithHeaders(t *testing.T) {
	t.Parallel()

	var receivedHeaders http.Header

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeaders = r.Header.Clone()
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewHTTPClient()

	req, err := http.NewRequest("GET", server.URL, nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("X-Custom-Header", "test-value")
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Do() error = %v", err)
	}
	defer resp.Body.Close()

	if receivedHeaders.Get("X-Custom-Header") != "test-value" {
		t.Error("custom header not sent")
	}
	if receivedHeaders.Get("Content-Type") != "application/json" {
		t.Error("content-type header not sent")
	}
}

func TestHTTPClient_DoWithDifferentMethods(t *testing.T) {
	t.Parallel()

	methods := []string{"GET", "POST", "PUT", "DELETE", "PATCH"}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			t.Parallel()

			var receivedMethod string

			server := httptest.NewServer(
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					receivedMethod = r.Method
					w.WriteHeader(http.StatusOK)
				}),
			)
			defer server.Close()

			client := NewHTTPClient()

			req, err := http.NewRequest(method, server.URL, nil)
			if err != nil {
				t.Fatalf("failed to create request: %v", err)
			}

			resp, err := client.Do(req)
			if err != nil {
				t.Fatalf("Do() error = %v", err)
			}
			defer resp.Body.Close()

			if receivedMethod != method {
				t.Errorf("method = %s, want %s", receivedMethod, method)
			}
		})
	}
}

func TestHTTPClient_DoWithQueryParams(t *testing.T) {
	t.Parallel()

	var receivedURL string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedURL = r.URL.String()
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewHTTPClient()

	req, err := http.NewRequest("GET", server.URL+"?symbol=BTCUSDT&limit=1000", nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Do() error = %v", err)
	}
	defer resp.Body.Close()

	if receivedURL != "/?symbol=BTCUSDT&limit=1000" {
		t.Errorf("URL = %s, want /?symbol=BTCUSDT&limit=1000", receivedURL)
	}
}
