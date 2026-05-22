package main

import (
	"time"
)

const (
	configDir = ".lmspeedtest"
)

var (
	version = "0.3.8-dev"
	commit  = "unknown"
	date    = "unknown"
)

type ServerProfile struct {
	Name     string `json:"name"`
	Host     string `json:"host"`
	Token    string `json:"token,omitempty"`
	Provider string `json:"provider,omitempty"` // "ollama" or "lmstudio"
}

type Config struct {
	Version        int             `json:"version"`
	ActiveProfile  string          `json:"active_profile"`
	DefaultProfile string          `json:"default_profile,omitempty"`
	Profiles       []ServerProfile `json:"profiles"`
}

type ModelDetails struct {
	ParameterSize     string `json:"parameter_size"`
	QuantizationLevel string `json:"quantization_level"`
	Family            string `json:"family"`
}

type Model struct {
	Name    string       `json:"name"`
	Size    int64        `json:"size"`
	Details ModelDetails `json:"details"`
}

type ModelsResponse struct {
	Models []Model `json:"models"`
}

type TestResult struct {
	Model          string        `json:"model"`
	TPS            float64       `json:"tps"`
	PromptEvalTPS  float64       `json:"prompt_eval_tps,omitempty"`
	TTFT           time.Duration `json:"ttft,omitempty"`
	LoadDuration   time.Duration `json:"load_duration,omitempty"`
	ITL            time.Duration `json:"itl,omitempty"`
	Timestamp      time.Time     `json:"timestamp"`
	Context        int           `json:"context"`
	ServerName     string        `json:"server_name,omitempty"`
	ServerHost     string        `json:"server_host,omitempty"`
	ServerProvider string        `json:"server_provider,omitempty"`
}

func providerDisplayName(provider string) string {
	if provider == "lmstudio" {
		return "LM Studio"
	}
	return "Ollama"
}
