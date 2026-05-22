package main

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
	"time"
)

func captureOutput(f func()) string {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	f()
	w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	io.Copy(&buf, r)
	return buf.String()
}

func TestResultServerLabel(t *testing.T) {
	origConfig := config
	defer func() { config = origConfig }()
	config = Config{DefaultProfile: "test-default"}

	tests := []struct {
		name     string
		result   TestResult
		expected string
	}{
		{
			name:     "ServerName set",
			result:   TestResult{ServerName: "myserver"},
			expected: "myserver",
		},
		{
			name:     "ServerName empty, ServerHost set",
			result:   TestResult{ServerName: "", ServerHost: "http://host"},
			expected: "http://host",
		},
		{
			name:     "both empty",
			result:   TestResult{ServerName: "", ServerHost: ""},
			expected: "test-default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resultServerLabel(tt.result)
			if got != tt.expected {
				t.Errorf("resultServerLabel() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestResultServerProvider(t *testing.T) {
	origConfig := config
	defer func() { config = origConfig }()
	config = Config{
		DefaultProfile: "test-default",
		Profiles: []ServerProfile{
			{Name: "myserver", Provider: "lmstudio"},
			{Name: "ollamasrv", Provider: "ollama"},
		},
	}

	tests := []struct {
		name     string
		result   TestResult
		expected string
	}{
		{
			name:     "ServerProvider set",
			result:   TestResult{ServerProvider: "lmstudio"},
			expected: "lmstudio",
		},
		{
			name:     "empty ServerProvider, matching profile",
			result:   TestResult{ServerName: "myserver"},
			expected: "lmstudio",
		},
		{
			name:     "empty ServerProvider, no matching profile",
			result:   TestResult{ServerName: "unknown"},
			expected: "ollama",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resultServerProvider(tt.result)
			if got != tt.expected {
				t.Errorf("resultServerProvider() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestResultDisplayKey(t *testing.T) {
	tests := []struct {
		name     string
		result   TestResult
		expected string
	}{
		{
			name:     "server and model",
			result:   TestResult{Model: "llama", ServerName: "srv"},
			expected: "srv / llama",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resultDisplayKey(tt.result)
			if got != tt.expected {
				t.Errorf("resultDisplayKey() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestProviderShortName(t *testing.T) {
	tests := []struct {
		provider string
		expected string
	}{
		{"lmstudio", "LM"},
		{"ollama", "OL"},
		{"custom", "CU"},
		{"x", "??"},
		{"", "??"},
	}

	for _, tt := range tests {
		t.Run(tt.provider, func(t *testing.T) {
			got := providerShortName(tt.provider)
			if got != tt.expected {
				t.Errorf("providerShortName(%q) = %q, want %q", tt.provider, got, tt.expected)
			}
		})
	}
}

func TestDashboardServerCell(t *testing.T) {
	origConfig := config
	defer func() { config = origConfig }()
	config = Config{DefaultProfile: "default"}

	tests := []struct {
		name     string
		result   TestResult
		width    int
		expected string
	}{
		{
			name:     "width 30",
			result:   TestResult{ServerName: "my-server", ServerProvider: "ollama"},
			width:    30,
			expected: "[OL] my-server",
		},
		{
			name:     "width 20 truncates",
			result:   TestResult{ServerName: "my-very-long-server-name", ServerProvider: "ollama"},
			width:    20,
			expected: "[OL] my-very-long...",
		},
		{
			name:     "width 3 truncates prefix",
			result:   TestResult{ServerName: "srv", ServerProvider: "ollama"},
			width:    3,
			expected: "...",
		},
		{
			name:     "width 3",
			result:   TestResult{ServerName: "srv", ServerProvider: "ollama"},
			width:    3,
			expected: "...",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := dashboardServerCell(tt.result, tt.width)
			if got != tt.expected {
				t.Errorf("dashboardServerCell() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestProviderLogoHTML(t *testing.T) {
	tests := []struct {
		name             string
		provider         string
		wantClass        string
		wantContainsLogo bool
	}{
		{
			name:      "ollama",
			provider:  "ollama",
			wantClass: "provider-logo-ollama",
		},
		{
			name:      "lmstudio",
			provider:  "lmstudio",
			wantClass: "provider-logo-lmstudio",
		},
		{
			name:      "unknown",
			provider:  "unknown",
			wantClass: "provider-logo-unknown",
		},
		{
			name:      "empty",
			provider:  "",
			wantClass: "provider-logo-unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := providerLogoHTML(tt.provider)
			if !strings.Contains(got, tt.wantClass) {
				t.Errorf("providerLogoHTML(%q) missing class %q, got %q", tt.provider, tt.wantClass, got)
			}
			if !strings.HasPrefix(got, "<span class=\"") {
				t.Errorf("providerLogoHTML(%q) bad HTML structure: %q", tt.provider, got)
			}
		})
	}
}

func TestBuildFlatResults(t *testing.T) {
	origResults := results
	defer func() { results = origResults }()

	tests := []struct {
		name     string
		setup    map[string]map[string][]TestResult
		expected int
		checkTPS []float64
	}{
		{
			name:     "empty results",
			setup:    map[string]map[string][]TestResult{},
			expected: 0,
		},
		{
			name: "single server single model two results",
			setup: map[string]map[string][]TestResult{
				"srv1": {
					"modelA": {
						{Model: "modelA", TPS: 10.0, Timestamp: time.Now()},
						{Model: "modelA", TPS: 20.0, Timestamp: time.Now()},
					},
				},
			},
			expected: 2,
			checkTPS: []float64{20.0, 10.0},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results = tt.setup
			got := buildFlatResults()
			if len(got) != tt.expected {
				t.Errorf("buildFlatResults() returned %d entries, want %d", len(got), tt.expected)
			}
			if tt.expected == 0 && got == nil {
				_ = got
			}
			for i, wantTPS := range tt.checkTPS {
				if i < len(got) && got[i].TPS != wantTPS {
					t.Errorf("buildFlatResults()[%d].TPS = %v, want %v", i, got[i].TPS, wantTPS)
				}
			}
		})
	}
}

func TestLatestPerModel(t *testing.T) {
	origResults := results
	defer func() { results = origResults }()

	now := time.Now()

	tests := []struct {
		name         string
		setup        map[string]map[string][]TestResult
		expectedLen  int
		expectedModel string
		expectedTPS  float64
	}{
		{
			name:         "empty results",
			setup:        map[string]map[string][]TestResult{},
			expectedLen:  0,
		},
		{
			name: "multiple results same model latest wins",
			setup: map[string]map[string][]TestResult{
				"srv1": {
					"modelA": {
						{Model: "modelA", TPS: 10.0, Timestamp: now.Add(-2 * time.Hour)},
						{Model: "modelA", TPS: 5.0, Timestamp: now},
					},
				},
			},
			expectedLen:   1,
			expectedModel: "modelA",
			expectedTPS:   5.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results = tt.setup
			got := latestPerModel()
			if len(got) != tt.expectedLen {
				t.Errorf("latestPerModel() returned %d entries, want %d", len(got), tt.expectedLen)
			}
			if tt.expectedLen > 0 {
				if got[0].Model != tt.expectedModel {
					t.Errorf("latestPerModel()[0].Model = %q, want %q", got[0].Model, tt.expectedModel)
				}
				if got[0].TPS != tt.expectedTPS {
					t.Errorf("latestPerModel()[0].TPS = %v, want %v", got[0].TPS, tt.expectedTPS)
				}
			}
		})
	}
}

func TestPrintVersion(t *testing.T) {
	output := captureOutput(printVersion)
	if !strings.Contains(output, "LMSpeedTest") {
		t.Errorf("output missing 'LMSpeedTest', got:\n%s", output)
	}
	if !strings.Contains(output, "commit:") {
		t.Errorf("output missing 'commit:', got:\n%s", output)
	}
	if !strings.Contains(output, "built:") {
		t.Errorf("output missing 'built:', got:\n%s", output)
	}
	if !strings.Contains(output, "go:") {
		t.Errorf("output missing 'go:', got:\n%s", output)
	}
}

func TestIsUpdateAvailable(t *testing.T) {
	tests := []struct {
		name    string
		current string
		latest  string
		want    bool
	}{
		{"older patch", "0.3.7", "v0.3.8", true},
		{"newer patch", "0.3.8", "v0.3.7", false},
		{"dev same version", "0.3.8-dev", "v0.3.8", false},
		{"same version", "0.3.7", "v0.3.7", false},
		{"dev newer", "0.3.8-dev", "v0.3.7", false},
		{"older minor", "0.3.7", "v0.4.0", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isUpdateAvailable(tt.current, tt.latest)
			if got != tt.want {
				t.Errorf("isUpdateAvailable(%q, %q) = %v, want %v", tt.current, tt.latest, got, tt.want)
			}
		})
	}
}

func TestCompletionsCmd(t *testing.T) {
	tests := []struct {
		name         string
		args         []string
		wantContains []string
	}{
		{
			name:         "bash",
			args:         []string{"lmspeedtest", "completions", "bash"},
			wantContains: []string{"_lmspeedtest()", "complete -F"},
		},
		{
			name:         "zsh",
			args:         []string{"lmspeedtest", "completions", "zsh"},
			wantContains: []string{"#compdef lmspeedtest", "_lmspeedtest()"},
		},
		{
			name:         "fish",
			args:         []string{"lmspeedtest", "completions", "fish"},
			wantContains: []string{"complete -c lmspeedtest"},
		},
		{
			name:         "no args shows usage",
			args:         []string{"lmspeedtest", "completions"},
			wantContains: []string{"Usage: lmspeedtest completions"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			origArgs := os.Args
			os.Args = tt.args
			defer func() { os.Args = origArgs }()
			output := captureOutput(completionsCmd)
			for _, want := range tt.wantContains {
				if !strings.Contains(output, want) {
					t.Errorf("output missing %q, got:\n%s", want, output)
				}
			}
		})
	}
}
