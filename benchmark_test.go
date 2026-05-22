package main

import (
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		want     string
	}{
		{name: "zero", duration: 0, want: "-"},
		{name: "500ms", duration: 500 * time.Millisecond, want: "500ms"},
		{name: "1500ms", duration: 1500 * time.Millisecond, want: "1.50s"},
		{name: "1s", duration: 1 * time.Second, want: "1.00s"},
		{name: "1µs", duration: 1 * time.Microsecond, want: "0ms"},
		{name: "100s", duration: 100 * time.Second, want: "100.00s"},
		{name: "negative", duration: -1 * time.Second, want: "-1000ms"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatDuration(tt.duration)
			if got != tt.want {
				t.Errorf("formatDuration(%v) = %q; want %q", tt.duration, got, tt.want)
			}
		})
	}
}

func TestIsLMStudioFatalLoadError(t *testing.T) {
	tests := []struct {
		name    string
		details string
		want    bool
	}{
		{name: "model_load_failed out of memory", details: "model_load_failed: out of memory", want: true},
		{name: "insufficient system resources", details: "insufficient system resources for model", want: true},
		{name: "failed to load model weights", details: "failed to load model weights", want: true},
		{name: "case insensitive", details: "MODEL_LOAD_FAILED", want: true},
		{name: "connection refused", details: "connection refused", want: false},
		{name: "empty string", details: "", want: false},
		{name: "model loaded successfully", details: "model loaded successfully", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isLMStudioFatalLoadError(tt.details)
			if got != tt.want {
				t.Errorf("isLMStudioFatalLoadError(%q) = %v; want %v", tt.details, got, tt.want)
			}
		})
	}
}

func TestResponseBodySnippet(t *testing.T) {
	largeBody := strings.Repeat("a", maxErrorBodyBytes+100)

	tests := []struct {
		name string
		body string
		want string
	}{
		{
			name: "normal body",
			body: "hello world",
			want: "hello world",
		},
		{
			name: "empty body",
			body: "",
			want: "empty response body",
		},
		{
			name: "whitespace only body",
			body: "   \n\t   ",
			want: "empty response body",
		},
		{
			name: "truncated body",
			body: largeBody,
			want: strings.Repeat("a", maxErrorBodyBytes) + "... (truncated)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := responseBodySnippet(strings.NewReader(tt.body))
			if got != tt.want {
				t.Errorf("responseBodySnippet() = %q; want %q", got, tt.want)
			}
		})
	}

	t.Run("reader error", func(t *testing.T) {
		got := responseBodySnippet(&errorReader{})
		if !strings.Contains(got, "could not read response body") {
			t.Errorf("responseBodySnippet(errorReader) = %q; want contains %q", got, "could not read response body")
		}
	})
}

func TestHttpErrorDetails(t *testing.T) {
	tests := []struct {
		name   string
		method string
		path   string
		body   string
		want   string
	}{
		{
			name:   "GET with body",
			method: "GET",
			path:   "/api/tags",
			body:   "not found",
			want:   "GET /api/tags: not found",
		},
		{
			name:   "POST with body",
			method: "POST",
			path:   "/api/generate",
			body:   "internal server error",
			want:   "POST /api/generate: internal server error",
		},
		{
			name:   "empty body",
			method: "DELETE",
			path:   "/v1/models",
			body:   "",
			want:   "DELETE /v1/models: empty response body",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := &http.Response{
				Body: io.NopCloser(strings.NewReader(tt.body)),
				Request: &http.Request{
					Method: tt.method,
					URL:    &url.URL{Path: tt.path},
				},
			}
			got := httpErrorDetails(resp)
			if got != tt.want {
				t.Errorf("httpErrorDetails() = %q; want %q", got, tt.want)
			}
		})
	}
}

type errorReader struct{}

func (e *errorReader) Read([]byte) (int, error) {
	return 0, errors.New("simulated read error")
}
