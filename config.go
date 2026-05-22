package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var (
	config  Config
	results = make(map[string]map[string][]TestResult)
)

func activeProfile() ServerProfile {
	for _, p := range config.Profiles {
		if p.Name == config.ActiveProfile {
			return p
		}
	}
	for _, p := range config.Profiles {
		if p.Name == config.DefaultProfile {
			return p
		}
	}
	if len(config.Profiles) > 0 {
		return config.Profiles[0]
	}
	return defaultServerProfile()
}

func defaultServerProfile() ServerProfile {
	return ServerProfile{Name: "local-ollama", Host: "http://localhost:11434", Provider: "ollama"}
}

func profileNameForHost(provider, host string) string {
	name := strings.TrimPrefix(strings.TrimPrefix(host, "https://"), "http://")
	name = strings.TrimSuffix(name, "/")
	replacer := strings.NewReplacer(":", "-", ".", "-", "/", "-", "_", "-")
	name = strings.Trim(replacer.Replace(strings.ToLower(name)), "-")
	if name == "" {
		name = "server"
	}
	if provider == "" {
		provider = "ollama"
	}
	return provider + "-" + name
}

func uniqueProfileName(base string, profiles []ServerProfile) string {
	name := base
	for i := 2; ; i++ {
		found := false
		for _, p := range profiles {
			if p.Name == name {
				found = true
				break
			}
		}
		if !found {
			return name
		}
		name = fmt.Sprintf("%s-%d", base, i)
	}
}

func normalizeConfig() bool {
	changed := false
	legacyDefaultProfile := ""
	if len(config.Profiles) == 0 {
		profile := defaultServerProfile()
		config.Profiles = []ServerProfile{profile}
		config.ActiveProfile = profile.Name
		config.DefaultProfile = profile.Name
		return true
	}
	for i := range config.Profiles {
		if config.Profiles[i].Provider == "" {
			config.Profiles[i].Provider = "ollama"
			changed = true
		}
		if strings.TrimSpace(config.Profiles[i].Name) == "" || config.Profiles[i].Name == "default" {
			config.Profiles[i].Name = uniqueProfileName(profileNameForHost(config.Profiles[i].Provider, config.Profiles[i].Host), config.Profiles[:i])
			if legacyDefaultProfile == "" {
				legacyDefaultProfile = config.Profiles[i].Name
			}
			changed = true
		}
	}
	if config.DefaultProfile == "" || config.DefaultProfile == "default" {
		if legacyDefaultProfile != "" {
			config.DefaultProfile = legacyDefaultProfile
		} else {
			config.DefaultProfile = config.Profiles[0].Name
		}
		changed = true
	}
	if config.ActiveProfile == "" || config.ActiveProfile == "default" {
		config.ActiveProfile = config.DefaultProfile
		changed = true
	}
	return changed
}

func profileByName(name string) (ServerProfile, bool) {
	for _, p := range config.Profiles {
		if p.Name == name {
			return p, true
		}
	}
	return ServerProfile{}, false
}

func inferProviderFromServerName(name string) string {
	switch {
	case strings.HasPrefix(name, "lmstudio-"):
		return "lmstudio"
	case strings.HasPrefix(name, "ollama-"):
		return "ollama"
	default:
		return ""
	}
}

func loadConfig() {
	data, err := os.ReadFile(getConfigPath())
	if err != nil {
		profile := defaultServerProfile()
		config = Config{
			Version:        2,
			ActiveProfile:  profile.Name,
			DefaultProfile: profile.Name,
			Profiles:       []ServerProfile{profile},
		}
		return
	}

	var shapeDetector struct {
		Version int    `json:"version"`
		Host    string `json:"host"`
	}
	_ = json.Unmarshal(data, &shapeDetector)

	if shapeDetector.Version == 0 && shapeDetector.Host != "" {
		var oldConfig struct {
			Host  string `json:"host"`
			Token string `json:"token,omitempty"`
		}
		if err := json.Unmarshal(data, &oldConfig); err == nil {
			profile := ServerProfile{
				Name:     profileNameForHost("ollama", oldConfig.Host),
				Host:     oldConfig.Host,
				Token:    oldConfig.Token,
				Provider: "ollama",
			}
			config = Config{
				Version:        2,
				ActiveProfile:  profile.Name,
				DefaultProfile: profile.Name,
				Profiles:       []ServerProfile{profile},
			}
			saveConfig()
			return
		}
	}

	if err := json.Unmarshal(data, &config); err != nil {
		profile := defaultServerProfile()
		config = Config{
			Version:        2,
			ActiveProfile:  profile.Name,
			DefaultProfile: profile.Name,
			Profiles:       []ServerProfile{profile},
		}
	} else {
		if normalizeConfig() {
			saveConfig()
		}
	}
}

func saveConfig() {
	dir := filepath.Dir(getConfigPath())
	if err := os.MkdirAll(dir, 0750); err != nil {
		printError("Cannot create config directory", err)
		return
	}
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		printError("Cannot marshal config", err)
		return
	}
	if err := os.WriteFile(getConfigPath(), data, 0600); err != nil {
		printError("Cannot write config", err)
	}
}

func loadResults() {
	data, err := os.ReadFile(getResultsPath())
	if err != nil {
		return
	}

	var nestedResults map[string]map[string][]TestResult
	if err := json.Unmarshal(data, &nestedResults); err == nil && len(nestedResults) > 0 {
		changed := false
		normalized := make(map[string]map[string][]TestResult)
		for _, serverData := range nestedResults {
			if len(serverData) > 0 {
				for serverName, serverData := range nestedResults {
					targetServer := serverName
					if targetServer == "default" {
						targetServer = config.DefaultProfile
						changed = true
					}
					if normalized[targetServer] == nil {
						normalized[targetServer] = make(map[string][]TestResult)
					}
					profile, hasProfile := profileByName(targetServer)
					for modelName, tests := range serverData {
						for i := range tests {
							if tests[i].ServerName == "" || tests[i].ServerName == "default" {
								tests[i].ServerName = targetServer
								changed = true
							}
							if hasProfile {
								if tests[i].ServerHost == "" {
									tests[i].ServerHost = profile.Host
									changed = true
								}
								if tests[i].ServerProvider == "" {
									tests[i].ServerProvider = profile.Provider
									changed = true
								}
							} else if tests[i].ServerProvider == "" {
								if provider := inferProviderFromServerName(targetServer); provider != "" {
									tests[i].ServerProvider = provider
									changed = true
								}
							}
						}
						normalized[targetServer][modelName] = append(normalized[targetServer][modelName], tests...)
					}
				}
				results = normalized
				if changed {
					saveResults()
				}
				return
			}
		}
	}

	var flatResults map[string][]TestResult
	if err := json.Unmarshal(data, &flatResults); err != nil {
		printWarning("Cannot parse results file, starting fresh", err)
		return
	}

	results = make(map[string]map[string][]TestResult)
	serverName := config.DefaultProfile
	profile, hasProfile := profileByName(serverName)
	results[serverName] = make(map[string][]TestResult)
	for modelName, tests := range flatResults {
		for i := range tests {
			tests[i].ServerName = serverName
			if hasProfile {
				tests[i].ServerHost = profile.Host
				tests[i].ServerProvider = profile.Provider
			}
		}
		results[serverName][modelName] = tests
	}
	saveResults()
}

func saveResults() {
	dir := filepath.Dir(getResultsPath())
	if err := os.MkdirAll(dir, 0750); err != nil {
		printError("Cannot create results directory", err)
		return
	}
	data, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		printError("Cannot marshal results", err)
		return
	}
	if err := os.WriteFile(getResultsPath(), data, 0600); err != nil {
		printError("Cannot write results", err)
	}
}
