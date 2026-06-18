package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

func exportCmd() {
	format := "csv"
	for i := 2; i < len(os.Args); i++ {
		if os.Args[i] == "--format" && i+1 < len(os.Args) {
			format = os.Args[i+1]
			i++
		}
	}

	if format != "csv" && format != "json" && format != "benchstat" && format != "markdown" {
		fmt.Println(errorStyle.Render("Invalid format. Use: csv, json, benchstat, or markdown"))
		return
	}

	var allResults []TestResult
	for _, serverData := range results {
		for _, tests := range serverData {
			allResults = append(allResults, tests...)
		}
	}

	sort.Slice(allResults, func(i, j int) bool {
		if allResults[i].Model != allResults[j].Model {
			return allResults[i].Model < allResults[j].Model
		}
		if resultServerLabel(allResults[i]) != resultServerLabel(allResults[j]) {
			return resultServerLabel(allResults[i]) < resultServerLabel(allResults[j])
		}
		return allResults[i].Timestamp.After(allResults[j].Timestamp)
	})

	if len(allResults) == 0 {
		fmt.Println(warningStyle.Render("No results to export."))
		return
	}

	switch format {
	case "csv":
		fmt.Println("server_name,server_host,server_provider,model,context_size,tokens_per_sec,prompt_eval_tps,ttft_ms,load_duration_ms,timestamp")
		for _, r := range allResults {
			fmt.Printf("%s,%s,%s,%s,%d,%.2f,%.2f,%.0f,%.0f,%s\n",
				r.ServerName,
				r.ServerHost,
				resultServerProvider(r),
				r.Model,
				r.Context,
				r.TPS,
				r.PromptEvalTPS,
				float64(r.TTFT)/float64(time.Millisecond),
				float64(r.LoadDuration)/float64(time.Millisecond),
				r.Timestamp.Format(time.RFC3339))
		}
	case "json":
		data, err := json.MarshalIndent(allResults, "", "  ")
		if err != nil {
			printError("Cannot marshal results", err)
			return
		}
		fmt.Println(string(data))
	case "benchstat":
		for _, r := range allResults {
			nsPerToken := int64(1e9 / r.TPS)
			fmt.Printf("BenchmarkModel/server=%s/name=%s/context=%dk/step=generate 1 %d ns/token %.2f token/sec\n",
				resultServerLabel(r),
				r.Model,
				r.Context/1024,
				nsPerToken,
				r.TPS)
		}
	case "markdown":
		fmt.Println("| Model | Server | Context | Tokens/sec | Prompt TPS | TTFT | ITL | Tested |")
		fmt.Println("|-------|--------|---------|------------|------------|------|-----|--------|")
		for _, r := range allResults {
			ctxLabel := fmt.Sprintf("%dk", r.Context/1024)
			ttftStr := "-"
			if r.TTFT > 0 {
				ttftStr = r.TTFT.String()
			}
			itlStr := "-"
			if r.ITL > 0 {
				itlStr = r.ITL.String()
			}
			fmt.Printf("| %s | %s | %s | %.2f | %.2f | %s | %s | %s |\n",
				r.Model,
				resultServerLabel(r),
				ctxLabel,
				r.TPS,
				r.PromptEvalTPS,
				ttftStr,
				itlStr,
				r.Timestamp.Format("2006-01-02 15:04"))
		}
	}
}

func infoCmd() {
	jsonOutput := hasJSONFlag()
	profile := activeProfile()
	
	type InfoResult struct {
		Profile   string `json:"profile"`
		Provider  string `json:"provider"`
		Host      string `json:"host"`
		Version   string `json:"version,omitempty"`
		Auth      bool   `json:"auth"`
		Connected bool   `json:"connected"`
	}
	
	result := InfoResult{
		Profile: profile.Name,
		Host:    profile.Host,
		Auth:    profile.Token != "",
	}
	
	if profile.Provider == "lmstudio" {
		result.Provider = "LM Studio"
		req, err := http.NewRequest(http.MethodGet, profile.Host+"/api/v1/models", nil)
		if err != nil {
			if jsonOutput {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				result.Connected = false
				_ = json.NewEncoder(os.Stdout).Encode(result)
			} else {
				printError("Cannot create request", err)
			}
			return
		}
		if profile.Token != "" {
			req.Header.Set("Authorization", "Bearer "+profile.Token)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			req, err = http.NewRequest(http.MethodGet, profile.Host+"/v1/models", nil)
			if err != nil {
				if jsonOutput {
					fmt.Fprintf(os.Stderr, "Error: %v\n", err)
					result.Connected = false
					_ = json.NewEncoder(os.Stdout).Encode(result)
				} else {
					printError("Cannot create request", err)
				}
				return
			}
			if profile.Token != "" {
				req.Header.Set("Authorization", "Bearer "+profile.Token)
			}
			resp, err = http.DefaultClient.Do(req)
		}
		if err != nil {
			if jsonOutput {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				result.Connected = false
				_ = json.NewEncoder(os.Stdout).Encode(result)
			} else {
				printError("Cannot connect to LM Studio", err)
			}
			return
		}


		if resp.StatusCode != http.StatusOK {
			if jsonOutput {
				fmt.Fprintf(os.Stderr, "Error: LM Studio returned status %d\n", resp.StatusCode)
				result.Connected = false
				_ = json.NewEncoder(os.Stdout).Encode(result)
			} else {
				printError(fmt.Sprintf("LM Studio returned status %d", resp.StatusCode), nil)
			}
			return
		}
		result.Connected = true
		
		if jsonOutput {
			_ = json.NewEncoder(os.Stdout).Encode(result)
			return
		}
		
		fmt.Printf("\n%s\n", titleStyle.Render("ℹ️  Server Info"))
		fmt.Println(separatorStyle.Render(strings.Repeat("═", 50)))
		fmt.Printf("%s %s\n", infoStyle.Render("Profile:"), modelNameStyle.Render(profile.Name))
		fmt.Printf("%s %s\n", infoStyle.Render("Provider:"), "LM Studio")
		fmt.Printf("%s %s\n", infoStyle.Render("Host:"), profile.Host)
		fmt.Printf("%s %s\n", infoStyle.Render("Status:"), successStyle.Render("connected"))
		if profile.Token != "" {
			fmt.Printf("%s %s\n", infoStyle.Render("Auth:"), successStyle.Render("configured"))
		} else {
			fmt.Printf("%s %s\n", infoStyle.Render("Auth:"), warningStyle.Render("none"))
		}
	} else {
		result.Provider = "Ollama"
		req, err := newAPIRequest("GET", profile.Host+"/api/version", nil)
		if err != nil {
			if jsonOutput {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				result.Connected = false
				_ = json.NewEncoder(os.Stdout).Encode(result)
			} else {
				printError("Cannot create request", err)
			}
			return
		}

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			if jsonOutput {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				result.Connected = false
				_ = json.NewEncoder(os.Stdout).Encode(result)
			} else {
				printError("Cannot connect to Ollama", err)
			}
			return
		}
		defer func() {
			if err := resp.Body.Close(); err != nil {
				fmt.Println(warningStyle.Render("⚠️ Cannot close response body:") + " " + err.Error())
			}
		}()

		var versionResp struct {
			Version string `json:"version"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&versionResp); err != nil {
			if jsonOutput {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				result.Connected = false
				_ = json.NewEncoder(os.Stdout).Encode(result)
			} else {
				printError("Cannot decode version response", err)
			}
			return
		}
		result.Connected = true
		result.Version = versionResp.Version
		
		if jsonOutput {
			_ = json.NewEncoder(os.Stdout).Encode(result)
			return
		}

		fmt.Printf("\n%s\n", titleStyle.Render("ℹ️  Server Info"))
		fmt.Println(separatorStyle.Render(strings.Repeat("═", 50)))
		fmt.Printf("%s %s\n", infoStyle.Render("Profile:"), modelNameStyle.Render(profile.Name))
		fmt.Printf("%s %s\n", infoStyle.Render("Provider:"), "Ollama")
		fmt.Printf("%s %s\n", infoStyle.Render("Host:"), profile.Host)
		fmt.Printf("%s %s\n", infoStyle.Render("Version:"), versionResp.Version)
		if profile.Token != "" {
			fmt.Printf("%s %s\n", infoStyle.Render("Auth:"), successStyle.Render("configured"))
		} else {
			fmt.Printf("%s %s\n", infoStyle.Render("Auth:"), warningStyle.Render("none"))
		}
	}

	if len(config.Profiles) > 1 {
		fmt.Println()
		fmt.Println(infoStyle.Render("All profiles:"))
		for _, p := range config.Profiles {
			marker := "  "
			if p.Name == config.ActiveProfile {
				marker = successStyle.Render("→ ")
			}
			fmt.Printf("%s%s (%s - %s)\n", marker, p.Name, providerDisplayName(p.Provider), p.Host)
		}
	}
}


func resultServerLabel(r TestResult) string {
	if r.ServerName != "" {
		return r.ServerName
	}
	if r.ServerHost != "" {
		return r.ServerHost
	}
	return config.DefaultProfile
}

func resultServerProvider(r TestResult) string {
	if r.ServerProvider != "" {
		return r.ServerProvider
	}
	if profile, ok := profileByName(resultServerLabel(r)); ok && profile.Provider != "" {
		return profile.Provider
	}
	return "ollama"
}

func resultDisplayKey(r TestResult) string {
	return resultServerLabel(r) + " / " + r.Model
}

func providerShortName(provider string) string {
	if provider == "lmstudio" {
		return "LM"
	}
	if provider == "ollama" {
		return "OL"
	}
	if len(provider) >= 2 {
		return strings.ToUpper(provider[:2])
	}
	return "??"
}

func dashboardServerCell(r TestResult, width int) string {
	prefix := fmt.Sprintf("[%s] ", providerShortName(resultServerProvider(r)))
	nameWidth := width - len(prefix)
	if nameWidth < 3 {
		return truncateString(prefix, width)
	}
	return prefix + truncateString(resultServerLabel(r), nameWidth)
}


func compareCmd() {
	jsonOutput := hasJSONFlag()
	if len(os.Args) < 3 {
		fmt.Println("Usage: lmspeedtest compare <model_name>")
		return
	}
	modelQuery := strings.ToLower(os.Args[2])

	var allTests []TestResult
	seen := make(map[string]bool)
	var matchedNames []string
	for _, serverData := range results {
		for modelKey, tests := range serverData {
			if strings.Contains(strings.ToLower(modelKey), modelQuery) {
				allTests = append(allTests, tests...)
				if !seen[modelKey] {
					matchedNames = append(matchedNames, modelKey)
					seen[modelKey] = true
				}
			}
		}
	}

	displayName := os.Args[2]
	if len(matchedNames) == 1 {
		displayName = matchedNames[0]
	}

	if len(allTests) == 0 {
		if jsonOutput {
			_ = json.NewEncoder(os.Stdout).Encode([]TestResult{})
		} else {
			fmt.Printf("%s %s\n",
				warningStyle.Render("No results found matching:"),
				modelNameStyle.Render(displayName))
		}
		return
	}

	sort.Slice(allTests, func(i, j int) bool {
		return allTests[i].Timestamp.After(allTests[j].Timestamp)
	})

	if jsonOutput {
		_ = json.NewEncoder(os.Stdout).Encode(allTests)
		return
	}

	fmt.Printf("\n%s\n",
		titleStyle.Render(fmt.Sprintf("📊 Comparing %s", displayName)))
	fmt.Println(separatorStyle.Render(strings.Repeat("═", 110)))
	fmt.Printf("%s %s %s %s %s %s\n",
		headerStyle.Render(fmt.Sprintf("%10s", "CONTEXT")),
		headerStyle.Render(fmt.Sprintf("%14s", "TOKENS/SEC")),
		headerStyle.Render(fmt.Sprintf("%14s", "PROMPT TPS")),
		headerStyle.Render(fmt.Sprintf("%12s", "TTFT")),
		headerStyle.Render(fmt.Sprintf("%12s", "SERVER")),
		headerStyle.Render(fmt.Sprintf("%18s", "TESTED")))
	fmt.Println(separatorStyle.Render(strings.Repeat("═", 110)))

	for _, r := range allTests {
		ctxLabel := fmt.Sprintf("%dk", r.Context/1024)
		fmt.Printf("%s %s %s %s %s %s\n",
			ctxStyle.Render(fmt.Sprintf("%10s", ctxLabel)),
			metricStyle.Render(fmt.Sprintf("%14.2f", r.TPS)),
			infoStyle.Render(fmt.Sprintf("%14.2f", r.PromptEvalTPS)),
			metricStyle.Render(fmt.Sprintf("%12s", formatDuration(r.TTFT))),
			infoStyle.Render(fmt.Sprintf("%12s", resultServerLabel(r))),
			infoStyle.Render(fmt.Sprintf("%18s", r.Timestamp.Format("2006-01-02 15:04"))))
	}

	if len(allTests) > 1 {
		var sum, minTPS, maxTPS float64
		minTPS = allTests[0].TPS
		for _, t := range allTests {
			sum += t.TPS
			if t.TPS < minTPS {
				minTPS = t.TPS
			}
			if t.TPS > maxTPS {
				maxTPS = t.TPS
			}
		}
		avg := sum / float64(len(allTests))
		fmt.Println(separatorStyle.Render(strings.Repeat("─", 110)))
		fmt.Printf("%s %s %s %s\n",
			infoStyle.Render(fmt.Sprintf("%10s", "STATS")),
			metricStyle.Render(fmt.Sprintf("avg: %.2f", avg)),
			metricStyle.Render(fmt.Sprintf("min: %.2f", minTPS)),
			metricStyle.Render(fmt.Sprintf("max: %.2f", maxTPS)))
	}
}

func dashboardCmd() {
	jsonOutput := hasJSONFlag()
	const (
		modelColWidth  = 28
		serverColWidth = 24
	)

	nameFilter := ""
	if len(os.Args) > 2 {
		nameFilter = strings.ToLower(os.Args[2])
	}

	var latest []TestResult
	for _, serverData := range results {
		for _, tests := range serverData {
			if len(tests) > 0 {
				latest = append(latest, tests[0])
			}
		}
	}

	sort.Slice(latest, func(i, j int) bool {
		return latest[i].TPS > latest[j].TPS
	})

	if nameFilter != "" {
		var filtered []TestResult
		for _, r := range latest {
			if strings.Contains(strings.ToLower(r.Model), nameFilter) {
				filtered = append(filtered, r)
			}
		}
		latest = filtered
	}

	if jsonOutput {
		_ = json.NewEncoder(os.Stdout).Encode(latest)
		return
	}

	models := fetchModels()
	modelSizes := make(map[string]float64)
	for _, m := range models {
		modelSizes[m.Name] = float64(m.Size) / (1024 * 1024 * 1024)
	}

	title := "🚀 LMSpeedTest Dashboard"
	if nameFilter != "" {
		title = fmt.Sprintf("🚀 LMSpeedTest Dashboard — filter: %s", nameFilter)
	}
	fmt.Printf("\n%s\n", titleStyle.Render(title))
	fmt.Println(separatorStyle.Render(strings.Repeat("═", 130)))
	fmt.Printf("%s %s %s %s %s %s %s\n",
		headerStyle.Render(fmt.Sprintf("%-*s", modelColWidth, "MODEL")),
		headerStyle.Render(fmt.Sprintf("%-*s", serverColWidth, "SERVER")),
		headerStyle.Render(fmt.Sprintf("%8s", "CTX")),
		headerStyle.Render(fmt.Sprintf("%14s", "TOKENS/SEC")),
		headerStyle.Render(fmt.Sprintf("%14s", "PROMPT TPS")),
		headerStyle.Render(fmt.Sprintf("%12s", "TTFT")),
		headerStyle.Render(fmt.Sprintf("%18s", "LAST TESTED")))
	fmt.Println(separatorStyle.Render(strings.Repeat("═", 130)))

	for _, r := range latest {
		modelName := truncateString(r.Model, modelColWidth)
		serverCell := dashboardServerCell(r, serverColWidth)
		ctxLabel := fmt.Sprintf("%dk", r.Context/1024)
		fmt.Printf("%s %s %s %s %s %s %s\n",
			modelNameStyle.Render(fmt.Sprintf("%-*s", modelColWidth, modelName)),
			infoStyle.Render(fmt.Sprintf("%-*s", serverColWidth, serverCell)),
			ctxStyle.Render(fmt.Sprintf("%8s", ctxLabel)),
			metricStyle.Render(fmt.Sprintf("%14.2f", r.TPS)),
			infoStyle.Render(fmt.Sprintf("%14.2f", r.PromptEvalTPS)),
			metricStyle.Render(fmt.Sprintf("%12s", formatDuration(r.TTFT))),
			infoStyle.Render(fmt.Sprintf("%18s", r.Timestamp.Format("2006-01-02 15:04"))))
	}

	if len(latest) == 0 {
		if nameFilter != "" {
			fmt.Println(warningStyle.Render(fmt.Sprintf("No results matching '%s'.", nameFilter)))
		} else {
			fmt.Println(warningStyle.Render("No test results yet. Run: lmspeedtest test <size>"))
		}
		return
	}

	fmt.Println(separatorStyle.Render(strings.Repeat("─", 95)))
	summaryLabel := "Summary across all models:"
	if nameFilter != "" {
		summaryLabel = fmt.Sprintf("Summary for '%s' models:", nameFilter)
	}
	fmt.Println(infoStyle.Render(summaryLabel))
	var totalTPS float64
	var totalSize float64
	seenModels := make(map[string]bool)
	for _, r := range latest {
		totalTPS += r.TPS
		if !seenModels[r.Model] {
			totalSize += modelSizes[r.Model]
			seenModels[r.Model] = true
		}
	}
	fmt.Printf("  %s: %s  %s: %.1f GB  %s: %.2f tokens/sec\n",
		infoStyle.Render("Rows"),
		metricStyle.Render(fmt.Sprintf("%d", len(latest))),
		infoStyle.Render("Total Size"),
		totalSize,
		infoStyle.Render("Avg TPS"),
		totalTPS/float64(len(latest)))
}

func pruneCmd() {
	profile := activeProfile()
	models := fetchModels()
	if len(models) == 0 {
		fmt.Println(warningStyle.Render("No models found on active server — nothing to prune."))
		return
	}

	serverData, ok := results[profile.Name]
	if !ok || len(serverData) == 0 {
		fmt.Println(infoStyle.Render("No results for active server — nothing to prune."))
		return
	}

	current := make(map[string]bool)
	for _, m := range models {
		current[m.Name] = true
	}

	var stale []string
	for modelName := range serverData {
		if !current[modelName] {
			stale = append(stale, modelName)
		}
	}

	if len(stale) == 0 {
		fmt.Printf("%s All stored results match current models on %s.\n",
			successStyle.Render("✓"),
			modelNameStyle.Render(profile.Name))
		return
	}

	sort.Strings(stale)
	fmt.Printf("\n%s\n", titleStyle.Render("🧹 Pruning stale results"))
	fmt.Println(separatorStyle.Render(strings.Repeat("─", 60)))
	fmt.Printf("Server: %s\n\n", modelNameStyle.Render(profile.Name))

	for _, name := range stale {
		delete(results[profile.Name], name)
		fmt.Printf("  %s %s\n", warningStyle.Render("removed"), modelNameStyle.Render(name))
	}

	saveResults()
	fmt.Println()
	fmt.Printf("%s Removed %d stale result(s).\n", successStyle.Render("✅"), len(stale))
}

func resetCmd() {
	fmt.Printf("\n%s\n", titleStyle.Render("🗑️  Reset Results"))
	fmt.Println(separatorStyle.Render(strings.Repeat("─", 50)))
	fmt.Println(warningStyle.Render("This will delete ALL benchmark results."))
	fmt.Print(promptStyle.Render("Type 'yes' to confirm: "))

	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	if strings.TrimSpace(strings.ToLower(scanner.Text())) != "yes" {
		fmt.Println(infoStyle.Render("Cancelled."))
		return
	}

	results = make(map[string]map[string][]TestResult)
	if err := os.Remove(getResultsPath()); err != nil {
		printWarning("Could not delete results file", err)
	}
	printSuccess("All results cleared!")
}


func doctorCmd() {
	passed := 0
	warnings := 0
	configErrors := 0
	connectivityErrors := 0
	permissionErrors := 0
	dataErrors := 0
	
	// Category tracking for exit codes
	// 0 = all pass, 1 = warnings only
	// 2 = config errors, 3 = connectivity errors
	// 4 = permission errors, 5 = data errors

	fmt.Printf("\n%s\n", titleStyle.Render("🏥  Doctor"))
	fmt.Println(separatorStyle.Render(strings.Repeat("═", 50)))

	configPath := getConfigPath()
	_, err := os.Stat(configPath)
	if err != nil {
		fmt.Printf("%s  Config file missing (fresh install)\n", warningStyle.Render("⚠️"))
		warnings++
	} else {
		fmt.Printf("%s  Config file exists\n", successStyle.Render("✅"))
		passed++
	}

	var cfg Config
	if err != nil {
		fmt.Printf("%s  Config parseable\n", warningStyle.Render("⚠️"))
		warnings++
	} else {
		data, err := os.ReadFile(configPath)
		if err != nil {
			fmt.Printf("%s  Config readable: %v\n", errorStyle.Render("❌"), err)
			configErrors++
		} else if err := json.Unmarshal(data, &cfg); err != nil {
			fmt.Printf("%s  Config parseable: %v\n", errorStyle.Render("❌"), err)
			configErrors++
		} else {
			fmt.Printf("%s  Config parseable\n", successStyle.Render("✅"))
			passed++
		}
	}

	// 3. At least one profile configured
	if len(cfg.Profiles) == 0 {
		fmt.Printf("%s  At least one profile configured\n", errorStyle.Render("❌"))
		configErrors++
	} else {
		fmt.Printf("%s  Profiles configured (%d)\n", successStyle.Render("✅"), len(cfg.Profiles))
		passed++
	}

	// 4. Active profile is valid
	activeValid := false
	for _, p := range cfg.Profiles {
		if p.Name == cfg.ActiveProfile {
			activeValid = true
			break
		}
	}
	if !activeValid {
		fmt.Printf("%s  Active profile valid\n", errorStyle.Render("❌"))
		configErrors++
	} else {
		fmt.Printf("%s  Active profile valid (%s)\n", successStyle.Render("✅"), cfg.ActiveProfile)
		passed++
	}

	// 5. Server reachable
	var profile *ServerProfile
	for i := range cfg.Profiles {
		if cfg.Profiles[i].Name == cfg.ActiveProfile {
			profile = &cfg.Profiles[i]
			break
		}
	}
	if profile == nil {
		fmt.Printf("%s  Server reachable\n", errorStyle.Render("❌"))
		connectivityErrors++
	} else {
		client := &http.Client{Timeout: 10 * time.Second}
		var url string
		if profile.Provider == "lmstudio" {
			url = profile.Host + "/api/v1/models"
		} else {
			url = profile.Host + "/api/version"
		}
		req, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			fmt.Printf("%s  Server reachable\n", errorStyle.Render("❌"))
			connectivityErrors++
		} else {
			if profile.Token != "" {
				req.Header.Set("Authorization", "Bearer "+profile.Token)
			}
			resp, err := client.Do(req)
			if err != nil {
				if profile.Provider == "lmstudio" {
					req, err = http.NewRequest(http.MethodGet, profile.Host+"/v1/models", nil)
					if err == nil {
						if profile.Token != "" {
							req.Header.Set("Authorization", "Bearer "+profile.Token)
						}
						resp, err = client.Do(req)
					}
				}
			}
			if err != nil {
				fmt.Printf("%s  Server reachable: %v\n", errorStyle.Render("❌"), err)
				connectivityErrors++
			} else {
				_ = resp.Body.Close()
				if resp.StatusCode == http.StatusOK {
					fmt.Printf("%s  Server reachable (%s)\n", successStyle.Render("✅"), profile.Host)
					passed++
				} else if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
					fmt.Printf("%s  Server reachable: %d\n", errorStyle.Render("❌"), resp.StatusCode)
					connectivityErrors++
				} else {
					fmt.Printf("%s  Server reachable: status %d\n", warningStyle.Render("⚠️"), resp.StatusCode)
					warnings++
				}
			}
		}
	}

	// 6. Auth valid
	if profile != nil && profile.Token != "" {
		client := &http.Client{Timeout: 10 * time.Second}
		var url string
		if profile.Provider == "lmstudio" {
			url = profile.Host + "/api/v1/models"
		} else {
			url = profile.Host + "/api/version"
		}
		req, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			fmt.Printf("%s  Auth valid\n", errorStyle.Render("❌"))
			connectivityErrors++
		} else {
			req.Header.Set("Authorization", "Bearer "+profile.Token)
			resp, err := client.Do(req)
			if err != nil {
				fmt.Printf("%s  Auth valid: %v\n", errorStyle.Render("❌"), err)
				connectivityErrors++
			} else {
				_ = resp.Body.Close()
				if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
					fmt.Printf("%s  Auth valid: %d\n", errorStyle.Render("❌"), resp.StatusCode)
					connectivityErrors++
				} else {
					fmt.Printf("%s  Auth valid\n", successStyle.Render("✅"))
					passed++
				}
			}
		}
	} else {
		fmt.Printf("%s  Auth valid (no token)\n", successStyle.Render("✅"))
		passed++
	}

	// 7. Config dir writable
	configDir := filepath.Dir(configPath)
	tmpFile, err := os.CreateTemp(configDir, "doctor-write-test-*")
	if err != nil {
		fmt.Printf("%s  Config dir writable: %v\n", errorStyle.Render("❌"), err)
		permissionErrors++
	} else {
		_ = tmpFile.Close()
		_ = os.Remove(tmpFile.Name())
		fmt.Printf("%s  Config dir writable\n", successStyle.Render("✅"))
		passed++
	}

	// 8. Results dir writable
	resultsPath := getResultsPath()
	resultsDir := filepath.Dir(resultsPath)
	tmpFile, err = os.CreateTemp(resultsDir, "doctor-write-test-*")
	if err != nil {
		fmt.Printf("%s  Results dir writable: %v\n", errorStyle.Render("❌"), err)
		permissionErrors++
	} else {
		_ = tmpFile.Close()
		_ = os.Remove(tmpFile.Name())
		fmt.Printf("%s  Results dir writable\n", successStyle.Render("✅"))
		passed++
	}

	// 9. Results file healthy
	_, err = os.Stat(resultsPath)
	if err != nil {
		fmt.Printf("%s  Results file missing (no results yet)\n", warningStyle.Render("⚠️"))
		warnings++
	} else {
		data, err := os.ReadFile(resultsPath)
		if err != nil {
			fmt.Printf("%s  Results file readable: %v\n", errorStyle.Render("❌"), err)
			dataErrors++
		} else {
			var nestedResults map[string]map[string][]TestResult
			if err := json.Unmarshal(data, &nestedResults); err != nil {
				fmt.Printf("%s  Results file healthy: %v\n", errorStyle.Render("❌"), err)
				dataErrors++
			} else {
				fmt.Printf("%s  Results file healthy\n", successStyle.Render("✅"))
				passed++
			}
		}
	}

	totalErrors := configErrors + connectivityErrors + permissionErrors + dataErrors
	fmt.Println(separatorStyle.Render(strings.Repeat("═", 50)))
	fmt.Printf("🏥  %s passed, %s warnings, %s errors\n",
		successStyle.Render(fmt.Sprintf("%d", passed)),
		warningStyle.Render(fmt.Sprintf("%d", warnings)),
		errorStyle.Render(fmt.Sprintf("%d", totalErrors)))
	fmt.Println()

	// Determine exit code based on error categories
	if totalErrors == 0 {
		if warnings > 0 {
			os.Exit(1) // Warnings only
		}
		return // Success (0)
	}
	
	// Return highest priority error category
	switch {
	case dataErrors > 0:
		os.Exit(5)
	case permissionErrors > 0:
		os.Exit(4)
	case connectivityErrors > 0:
		os.Exit(3)
	case configErrors > 0:
		os.Exit(2)
	}
}
func completionsCmd() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: lmspeedtest completions [bash|zsh|fish]")
		fmt.Println()
		fmt.Println("Generate shell completion scripts.")
		fmt.Println()
		fmt.Println("Examples:")
		fmt.Println("  lmspeedtest completions bash > /usr/local/etc/bash_completion.d/lmspeedtest")
		fmt.Println("  lmspeedtest completions zsh > /usr/local/share/zsh/site-functions/_lmspeedtest")
		fmt.Println("  lmspeedtest completions fish > ~/.config/fish/completions/lmspeedtest.fish")
		return
	}

	shell := os.Args[2]
	switch shell {
	case "bash":
		fmt.Println(bashCompletionScript)
	case "zsh":
		fmt.Println(zshCompletionScript)
	case "fish":
		fmt.Println(fishCompletionScript)
	default:
		fmt.Printf("Unknown shell: %s\n", shell)
		fmt.Println("Supported shells: bash, zsh, fish")
		os.Exit(1)
	}
}

const bashCompletionScript = `# bash completion for lmspeedtest
_lmspeedtest() {
    local cur prev opts
    COMPREPLY=()
    cur="${COMP_WORDS[COMP_CWORD]}"
    prev="${COMP_WORDS[COMP_CWORD-1]}"
    opts="connect models test dashboard export compare info reset serve doctor completions update --version -v"

    case "${prev}" in
        connect)
            opts="--add --list --default --use --remove"
            ;;
        test)
            opts="--all --epochs --template --prompt-file"
            ;;
        export)
            opts="--format"
            ;;
        serve)
            opts="--port"
            ;;
        completions)
            opts="bash zsh fish"
            ;;
        --format)
            opts="csv json benchstat markdown"
            ;;
        --template)
            opts="code chat long"
            ;;
        --epochs|--port|--add|--default|--use|--remove)
            opts=""
            ;;
    esac

    COMPREPLY=( $(compgen -W "${opts}" -- "${cur}") )
    return 0
}
complete -F _lmspeedtest lmspeedtest`

const zshCompletionScript = `#compdef lmspeedtest

_lmspeedtest() {
    local curcontext="$curcontext" state line
    typeset -A opt_args

    _arguments -C \\
        '1: :->command' \\
        '*: :->args' && ret=0

    case "$state" in
        command)
            _values 'commands' \\
                'connect[Configure server profile]' \\
                'models[List local models]' \\
                'test[Benchmark models]' \\
                'dashboard[Show benchmark results]' \\
                'export[Export results]' \\
                'compare[Compare context sizes]' \\
                'info[Show server info]' \\
                'reset[Clear results]' \\
                'serve[Start web dashboard]' \\
                'doctor[Run diagnostics]' \\
                'completions[Generate shell completions]' \\
                'update[Check for updates]'
            ;;
        args)
            case "$line[1]" in
                connect)
                    _arguments \\
                        '--add[Add a new profile]' \\
                        '--list[List profiles]' \\
                        '--default[Set default profile]' \\
                        '--use[Switch profile]' \\
                        '--remove[Remove profile]'
                    ;;
                test)
                    _arguments \\
                        '--all[Benchmark all models]' \\
                        '--epochs[Number of epochs]' \\
                        '--template[Prompt template]' \\
                        '--prompt-file[Custom prompt file]'
                    ;;
                export)
                    _arguments '--format[Export format]:(csv json benchstat markdown)'
                    ;;
                serve)
                    _arguments '--port[Port number]'
                    ;;
                completions)
                    _arguments '2: :(bash zsh fish)'
                    ;;
            esac
            ;;
    esac
}

compdef _lmspeedtest lmspeedtest`

const fishCompletionScript = `complete -c lmspeedtest -f

# Commands
complete -c lmspeedtest -n '__fish_use_subcommand' -a 'connect' -d 'Configure server profile'
complete -c lmspeedtest -n '__fish_use_subcommand' -a 'models' -d 'List local models'
complete -c lmspeedtest -n '__fish_use_subcommand' -a 'test' -d 'Benchmark models'
complete -c lmspeedtest -n '__fish_use_subcommand' -a 'dashboard' -d 'Show benchmark results'
complete -c lmspeedtest -n '__fish_use_subcommand' -a 'export' -d 'Export results'
complete -c lmspeedtest -n '__fish_use_subcommand' -a 'compare' -d 'Compare context sizes'
complete -c lmspeedtest -n '__fish_use_subcommand' -a 'info' -d 'Show server info'
complete -c lmspeedtest -n '__fish_use_subcommand' -a 'reset' -d 'Clear results'
complete -c lmspeedtest -n '__fish_use_subcommand' -a 'serve' -d 'Start web dashboard'
complete -c lmspeedtest -n '__fish_use_subcommand' -a 'doctor' -d 'Run diagnostics'
complete -c lmspeedtest -n '__fish_use_subcommand' -a 'completions' -d 'Generate shell completions'
complete -c lmspeedtest -n '__fish_use_subcommand' -a 'update' -d 'Check for updates'

# Command-specific options
complete -c lmspeedtest -n '__fish_seen_subcommand_from connect' -l add -d 'Add a new profile'
complete -c lmspeedtest -n '__fish_seen_subcommand_from connect' -l list -d 'List profiles'
complete -c lmspeedtest -n '__fish_seen_subcommand_from connect' -l default -d 'Set default profile'
complete -c lmspeedtest -n '__fish_seen_subcommand_from connect' -l use -d 'Switch profile'
complete -c lmspeedtest -n '__fish_seen_subcommand_from connect' -l remove -d 'Remove profile'

complete -c lmspeedtest -n '__fish_seen_subcommand_from test' -l all -d 'Benchmark all models'
complete -c lmspeedtest -n '__fish_seen_subcommand_from test' -l epochs -d 'Number of epochs'
complete -c lmspeedtest -n '__fish_seen_subcommand_from test' -l template -d 'Prompt template'
complete -c lmspeedtest -n '__fish_seen_subcommand_from test' -l prompt-file -d 'Custom prompt file'

complete -c lmspeedtest -n '__fish_seen_subcommand_from export' -l format -a 'csv json benchstat markdown' -d 'Export format'
complete -c lmspeedtest -n '__fish_seen_subcommand_from serve' -l port -d 'Port number'
complete -c lmspeedtest -n '__fish_seen_subcommand_from completions' -a 'bash zsh fish' -d 'Shell type'`

func updateCmd() {
	fmt.Println("Checking for updates...")
	fmt.Println()

	resp, err := http.Get("https://api.github.com/repos/notfixingit3/lmspeedtest/releases/latest")
	if err != nil {
		printError("Cannot check for updates", err)
		return
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		printError(fmt.Sprintf("GitHub API returned status %d", resp.StatusCode), nil)
		return
	}

	var release struct {
		TagName string `json:"tag_name"`
		HTMLURL string `json:"html_url"`
		Name    string `json:"name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		printError("Cannot decode release info", err)
		return
	}

	fmt.Printf("Current version: %s\n", version)
	fmt.Printf("Latest version:  %s\n", release.TagName)
	fmt.Println()

	if isUpdateAvailable(version, release.TagName) {
		fmt.Println(successStyle.Render("✅ A new version is available!"))
		fmt.Println()
		fmt.Printf("Release: %s\n", release.Name)
		fmt.Printf("URL:     %s\n", release.HTMLURL)
		fmt.Println()
		fmt.Println("To update, run:")
		fmt.Printf("  go install github.com/notfixingit3/lmspeedtest@%s\n", release.TagName)
		fmt.Println()
		fmt.Println("Or download from the releases page:")
		fmt.Printf("  %s\n", release.HTMLURL)
	} else {
		fmt.Println(successStyle.Render("✅ You are on the latest version."))
	}
}

func cleanVersion(v string) string {
	return strings.TrimPrefix(v, "v")
}

func isUpdateAvailable(current, latest string) bool {
	current = cleanVersion(current)
	latest = cleanVersion(latest)

	// Strip -dev suffix for comparison
	current = strings.Split(current, "-")[0]

	parts := strings.Split(current, ".")
	latestParts := strings.Split(latest, ".")

	for i := 0; i < 3; i++ {
		var c, l int
		if i < len(parts) {
			c, _ = strconv.Atoi(parts[i])
		}
		if i < len(latestParts) {
			l, _ = strconv.Atoi(latestParts[i])
		}
		if l > c {
			return true
		}
		if l < c {
			return false
		}
	}
	return false
}
