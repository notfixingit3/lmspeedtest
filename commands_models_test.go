package main

import (
	"os"
	"testing"
)

func TestExtractParameterSize(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"no match", "llama3.2:latest", ""},
		{"extract size uppercase", "model-7b-q4", "7B"},
		{"already uppercase", "model-13B-q4", "13B"},
		{"decimal", "model-1.5b", "1.5B"},
		{"millions", "model-412m", "412M"},
		{"empty", "", ""},
		{"first match only", "7b-13b-model", "7B"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractParameterSize(tt.input)
			if got != tt.want {
				t.Errorf("extractParameterSize(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestEstimateSizeFromParams(t *testing.T) {
	tests := []struct {
		name   string
		params string
		quant  string
		want   int64
	}{
		{"Q4_K_M", "7B", "Q4_K_M", 3937500000},
		{"Q2_K", "7B", "Q2_K", 2187500000},
		{"Q8_0", "7B", "Q8_0", 7437500000},
		{"F16", "7B", "F16", 14000000000},
		{"unknown quant", "7B", "unknown", 3937500000},
		{"empty params", "", "Q4_K_M", 0},
		{"M suffix F16", "137M", "F16", 274000000},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := estimateSizeFromParams(tt.params, tt.quant)
			if got != tt.want {
				t.Errorf("estimateSizeFromParams(%q, %q) = %d, want %d", tt.params, tt.quant, got, tt.want)
			}
		})
	}
}

func TestGetPrompt(t *testing.T) {
	tests := []struct {
		name     string
		template string
		want     string
	}{
		{"code", "code", "Write a Python function to parse JSON and extract nested keys. Include error handling."},
		{"chat", "chat", "Explain the concept of recursion in programming. Keep it brief and clear."},
		{"long", "long", "Write a detailed 800-word technical article about the evolution of large language models."},
		{"custom passthrough", "custom prompt text", "custom prompt text"},
		{"empty passthrough", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getPrompt(tt.template)
			if got != tt.want {
				t.Errorf("getPrompt(%q) = %q, want %q", tt.template, got, tt.want)
			}
		})
	}
}

func TestTruncateString(t *testing.T) {
	t.Run("short string no truncation", func(t *testing.T) {
		got := truncateString("hello", 10)
		want := "hello"
		if got != want {
			t.Errorf("truncateString(%q, %d) = %q, want %q", "hello", 10, got, want)
		}
	})

	t.Run("truncate with ellipsis", func(t *testing.T) {
		got := truncateString("hello world", 8)
		// s[:maxLen-3] + "..." = s[:5] + "..." = "hello" + "..." = "hello..."
		if got != "hello..." {
			t.Errorf("truncateString(%q, %d) = %q, want %q", "hello world", 8, got, "hello...")
		}
	})

	t.Run("empty string", func(t *testing.T) {
		got := truncateString("", 5)
		want := ""
		if got != want {
			t.Errorf("truncateString(%q, %d) = %q, want %q", "", 5, got, want)
		}
	})

	t.Run("exact max length", func(t *testing.T) {
		got := truncateString("abc", 3)
		want := "abc"
		if got != want {
			t.Errorf("truncateString(%q, %d) = %q, want %q", "abc", 3, got, want)
		}
	})

	t.Run("only ellipsis", func(t *testing.T) {
		got := truncateString("abcd", 3)
		want := "..."
		if got != want {
			t.Errorf("truncateString(%q, %d) = %q, want %q", "abcd", 3, got, want)
		}
	})

	t.Run("zero maxLen panics", func(t *testing.T) {
		// truncateString("hello", 0) causes a panic due to negative slice index
		func() {
			defer func() {
				if r := recover(); r == nil {
					t.Errorf("truncateString(%q, %d) should panic", "hello", 0)
				}
			}()
			truncateString("hello", 0)
		}()
	})

	t.Run("unicode byte truncation", func(t *testing.T) {
		input := "unicode测试text"
		// "unicode" = 7 bytes, "测试" = 6 bytes (3 each), "text" = 4 bytes → total 17 bytes
		// maxLen=12 → s[:12-3] + "..." = s[:9] + "..."
		// s[:9] = "unicode" + first 2 bytes of "测"
		got := truncateString(input, 12)
		want := input[:9] + "..."
		if got != want {
			t.Errorf("truncateString(%q, %d) = %q, want %q", input, 12, got, want)
		}
		// Verify the output has exactly maxLen bytes
		if len(got) > 12 {
			t.Errorf("result length %d exceeds maxLen %d", len(got), 12)
		}
	})
}

func TestTruncateStringTableDriven(t *testing.T) {
	// Additional table-driven cases that don't trigger panic
	tests := []struct {
		name   string
		input  string
		maxLen int
		want   string
	}{
		{"short string", "hello", 10, "hello"},
		{"truncate with ellipsis", "hello world", 8, "hello..."},
		{"empty string", "", 5, ""},
		{"exact max length", "abc", 3, "abc"},
		{"only ellipsis", "abcd", 3, "..."},
		{"unicode byte truncation", "unicode测试text", 12, "unicode测试text"[:9] + "..."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateString(tt.input, tt.maxLen)
			if got != tt.want {
				t.Errorf("truncateString(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.want)
			}
		})
	}
}

func TestHasJSONFlag(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want bool
	}{
		{"no json flag", []string{"lmspeedtest", "models", "8"}, false},
		{"--json present", []string{"lmspeedtest", "models", "--json"}, true},
		{"--json at start", []string{"lmspeedtest", "--json", "models"}, true},
		{"--json only arg", []string{"lmspeedtest", "--json"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			origArgs := os.Args
			os.Args = tt.args
			defer func() { os.Args = origArgs }()
			if got := hasJSONFlag(); got != tt.want {
				t.Errorf("hasJSONFlag() = %v, want %v", got, tt.want)
			}
		})
	}
}
