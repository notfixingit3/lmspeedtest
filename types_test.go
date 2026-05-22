package main

import "testing"

func TestProviderDisplayName(t *testing.T) {
	tests := []struct {
		name     string
		provider string
		want     string
	}{
		{"lmstudio returns LM Studio", "lmstudio", "LM Studio"},
		{"ollama returns Ollama", "ollama", "Ollama"},
		{"empty string defaults to Ollama", "", "Ollama"},
		{"unknown defaults to Ollama", "unknown", "Ollama"},
		{"case-sensitive no match", "LMSTUDIO", "Ollama"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := providerDisplayName(tt.provider)
			if got != tt.want {
				t.Errorf("providerDisplayName(%q) = %q, want %q", tt.provider, got, tt.want)
			}
		})
	}
}
