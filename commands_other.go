package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"html"
	"math"
	"net/http"
	"os"
	"sort"
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

	if format != "csv" && format != "json" && format != "benchstat" {
		fmt.Println(errorStyle.Render("Invalid format. Use: csv, json, or benchstat"))
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
	}
}

func infoCmd() {
	profile := activeProfile()
	if profile.Provider == "lmstudio" {
		req, err := http.NewRequest(http.MethodGet, profile.Host+"/api/v1/models", nil)
		if err != nil {
			printError("Cannot create request", err)
			return
		}
		if profile.Token != "" {
			req.Header.Set("Authorization", "Bearer "+profile.Token)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			// Fallback to /v1/models
			req, err = http.NewRequest(http.MethodGet, profile.Host+"/v1/models", nil)
			if err != nil {
				printError("Cannot create request", err)
				return
			}
			if profile.Token != "" {
				req.Header.Set("Authorization", "Bearer "+profile.Token)
			}
			resp, err = http.DefaultClient.Do(req)
		}
		if err != nil {
			printError("Cannot connect to LM Studio", err)
			return
		}
		defer func() {
			_ = resp.Body.Close()
		}()

		if resp.StatusCode != http.StatusOK {
			printError(fmt.Sprintf("LM Studio returned status %d", resp.StatusCode), nil)
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
		req, err := newAPIRequest("GET", profile.Host+"/api/version", nil)
		if err != nil {
			printError("Cannot create request", err)
			return
		}

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			printError("Cannot connect to Ollama", err)
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
			printError("Cannot decode version response", err)
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

func compareCmd() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: lmspeedtest compare <model_name>")
		return
	}
	modelName := os.Args[2]

	var allTests []TestResult
	for _, serverData := range results {
		if tests, ok := serverData[modelName]; ok {
			allTests = append(allTests, tests...)
		}
	}

	if len(allTests) == 0 {
		fmt.Printf("%s %s\n",
			warningStyle.Render("No results found for model:"),
			modelNameStyle.Render(modelName))
		return
	}

	sort.Slice(allTests, func(i, j int) bool {
		return allTests[i].Timestamp.After(allTests[j].Timestamp)
	})

	fmt.Printf("\n%s\n",
		titleStyle.Render(fmt.Sprintf("📊 Comparing %s", modelName)))
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

func providerLogoHTML(provider string) string {
	providerClass := "unknown"
	logo := genericLogoDataURI
	if provider == "ollama" {
		providerClass = "ollama"
		logo = ollamaLogoDataURI
	} else if provider == "lmstudio" {
		providerClass = "lmstudio"
		logo = lmStudioLogoDataURI
	}
	label := html.EscapeString(providerDisplayName(provider))
	return fmt.Sprintf(`<span class="provider-logo provider-logo-%s" title="%s"><img src="%s" alt="%s"></span>`, providerClass, label, logo, label)
}

const (
	ollamaLogoDataURI   = `data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 32 32'%3E%3Crect width='32' height='32' rx='8' fill='%23111827'/%3E%3Ccircle cx='16' cy='16' r='9' fill='none' stroke='%23f8fafc' stroke-width='3'/%3E%3Ccircle cx='13' cy='14' r='1.6' fill='%23f8fafc'/%3E%3Ccircle cx='19' cy='14' r='1.6' fill='%23f8fafc'/%3E%3Cpath d='M12 20c2.2 1.6 5.8 1.6 8 0' fill='none' stroke='%23f8fafc' stroke-width='2' stroke-linecap='round'/%3E%3C/svg%3E`
	lmStudioLogoDataURI = `data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 32 32'%3E%3Crect width='32' height='32' rx='8' fill='%232563eb'/%3E%3Cpath d='M8 8h4v12h7v4H8V8zm12 0h4v16h-4V14l-4 5-4-5V9l4 5 4-6z' fill='%23fff'/%3E%3C/svg%3E`
	genericLogoDataURI  = `data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 32 32'%3E%3Crect width='32' height='32' rx='8' fill='%23475569'/%3E%3Cpath d='M9 11h14M9 16h14M9 21h14' stroke='%23fff' stroke-width='3' stroke-linecap='round'/%3E%3C/svg%3E`
)

func dashboardCmd() {
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

type flatResult struct {
	ID             string  `json:"id"`
	ServerName     string  `json:"server_name"`
	ServerHost     string  `json:"server_host,omitempty"`
	ServerProvider string  `json:"server_provider,omitempty"`
	Model          string  `json:"model"`
	Context        int     `json:"context"`
	TPS            float64 `json:"tps"`
	PromptEvalTPS  float64 `json:"prompt_eval_tps"`
	TTFTMs         float64 `json:"ttft_ms"`
	LoadMs         float64 `json:"load_ms"`
	ITLMs          float64 `json:"itl_ms"`
	Timestamp      string  `json:"timestamp"`
}

func buildFlatResults() []flatResult {
	var flat []flatResult
	for _, serverData := range results {
		for _, tests := range serverData {
			for _, r := range tests {
				flat = append(flat, flatResult{
					ID:             resultDisplayKey(r),
					ServerName:     resultServerLabel(r),
					ServerHost:     r.ServerHost,
					ServerProvider: resultServerProvider(r),
					Model:          r.Model,
					Context:        r.Context,
					TPS:            r.TPS,
					PromptEvalTPS:  r.PromptEvalTPS,
					TTFTMs:         float64(r.TTFT) / float64(time.Millisecond),
					LoadMs:         float64(r.LoadDuration) / float64(time.Millisecond),
					ITLMs:          float64(r.ITL) / float64(time.Millisecond),
					Timestamp:      r.Timestamp.Format(time.RFC3339),
				})
			}
		}
	}
	sort.Slice(flat, func(i, j int) bool {
		return flat[i].TPS > flat[j].TPS
	})
	return flat
}

func latestPerModel() []TestResult {
	var latest []TestResult
	for _, serverData := range results {
		for _, tests := range serverData {
			if len(tests) > 0 {
				var best TestResult
				for _, r := range tests {
					if r.Timestamp.After(best.Timestamp) {
						best = r
					}
				}
				latest = append(latest, best)
			}
		}
	}
	sort.Slice(latest, func(i, j int) bool {
		return latest[i].TPS > latest[j].TPS
	})
	return latest
}

func jsonResultsBytes() []byte {
	flat := buildFlatResults()
	if flat == nil {
		flat = []flatResult{}
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "")
	if err := enc.Encode(flat); err != nil {
		return []byte("[]")
	}
	return buf.Bytes()
}

func serveCmd() {
	port := "8080"
	for i := 2; i < len(os.Args); i++ {
		if (os.Args[i] == "--port" || os.Args[i] == "-p") && i+1 < len(os.Args) {
			port = os.Args[i+1]
			i++
		}
	}

	http.HandleFunc("/api/results", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		_, _ = w.Write(jsonResultsBytes())
	})

	http.HandleFunc("/api/export", func(w http.ResponseWriter, r *http.Request) {
		format := r.URL.Query().Get("format")
		flat := buildFlatResults()

		switch format {
		case "csv":
			w.Header().Set("Content-Type", "text/csv; charset=utf-8")
			w.Header().Set("Content-Disposition", "attachment; filename=lmspeedtest-results.csv")
			fmt.Fprintf(w, "server_name,server_host,server_provider,model,context,tokens_per_sec,prompt_eval_tps,ttft_ms,itl_ms,load_ms,timestamp\n")
			for _, r := range flat {
				fmt.Fprintf(w, "%s,%s,%s,%s,%d,%.2f,%.2f,%.0f,%.0f,%.0f,%s\n",
					r.ServerName, r.ServerHost, r.ServerProvider, r.Model, r.Context, r.TPS, r.PromptEvalTPS, r.TTFTMs, r.ITLMs, r.LoadMs, r.Timestamp)
			}
		case "json":
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Content-Disposition", "attachment; filename=lmspeedtest-results.json")
			enc := json.NewEncoder(w)
			enc.SetEscapeHTML(false)
			enc.SetIndent("", "  ")
			_ = enc.Encode(flat)
		default:
			http.Error(w, "Use format=csv or format=json", http.StatusBadRequest)
		}
	})

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")

		latest := latestPerModel()
		jsonData := jsonResultsBytes()

		var totalTPS, totalPromptTPS float64
		var totalTTFT time.Duration
		for _, r := range latest {
			totalTPS += r.TPS
			totalPromptTPS += r.PromptEvalTPS
			totalTTFT += r.TTFT
		}

		fastestModel := ""
		mostStableModel := ""
		var maxTPS float64
		minStddev := math.MaxFloat64
		for _, serverData := range results {
			for _, allTests := range serverData {
				if len(allTests) == 0 {
					continue
				}
				for _, t := range allTests {
					if t.TPS > maxTPS {
						maxTPS = t.TPS
						fastestModel = resultDisplayKey(t)
					}
				}
				if len(allTests) > 1 {
					var sum, mean float64
					for _, t := range allTests {
						sum += t.TPS
					}
					mean = sum / float64(len(allTests))
					var variance float64
					for _, t := range allTests {
						diff := t.TPS - mean
						variance += diff * diff
					}
					variance /= float64(len(allTests))
					stddev := math.Sqrt(variance)
					if stddev < minStddev {
						minStddev = stddev
						mostStableModel = resultDisplayKey(allTests[0])
					}
				}
			}
		}

		// --- PAGE START: HTML head + CSS ---
		fmt.Fprint(w, `<!DOCTYPE html>
<html lang="en" data-theme="dark">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>LMSpeedTest Dashboard</title>
<style>
:root {
  --bg-primary: #0a0e1a;
  --bg-secondary: #111827;
  --bg-card: #1a2236;
  --bg-hover: #243044;
  --border-color: #2d3a4f;
  --text-primary: #f0f4f8;
  --text-secondary: #94a3b8;
  --accent-blue: #3b82f6;
  --accent-cyan: #06b6d4;
  --accent-green: #10b981;
  --accent-purple: #8b5cf6;
  --accent-orange: #f59e0b;
  --accent-red: #ef4444;
  --accent-pink: #ec4899;
  --gradient-1: linear-gradient(135deg, #3b82f6 0%, #8b5cf6 100%);
  --gradient-2: linear-gradient(135deg, #06b6d4 0%, #3b82f6 100%);
  --shadow: 0 4px 6px -1px rgba(0,0,0,0.3),0 2px 4px -1px rgba(0,0,0,0.2);
  --shadow-lg: 0 20px 25px -5px rgba(0,0,0,0.4),0 10px 10px -5px rgba(0,0,0,0.2);
}
[data-theme="light"] {
  --bg-primary: #f8fafc;
  --bg-secondary: #f1f5f9;
  --bg-card: #ffffff;
  --bg-hover: #e2e8f0;
  --border-color: #cbd5e1;
  --text-primary: #0f172a;
  --text-secondary: #475569;
  --shadow: 0 4px 6px -1px rgba(0,0,0,0.08),0 2px 4px -1px rgba(0,0,0,0.04);
  --shadow-lg: 0 20px 25px -5px rgba(0,0,0,0.1),0 10px 10px -5px rgba(0,0,0,0.04);
}
*,*::before,*::after{box-sizing:border-box;margin:0;padding:0}
body{
  font-family:'Inter',-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif;
  background:var(--bg-primary);
  color:var(--text-primary);
  min-height:100vh;line-height:1.6;
  transition:background 0.3s,color 0.3s;
}
.container{max-width:1400px;margin:0 auto;padding:1.5rem 2rem}

/* Header */
header{
  background:var(--bg-secondary);
  border-bottom:1px solid var(--border-color);
  padding:1rem 2rem;position:sticky;top:0;z-index:100;
}
.header-content{
  max-width:1400px;margin:0 auto;
  display:flex;align-items:center;justify-content:space-between;
  flex-wrap:wrap;gap:0.75rem;
}
.logo{
  display:flex;align-items:center;gap:0.6rem;
  font-size:1.35rem;font-weight:700;
  background:var(--gradient-1);
  -webkit-background-clip:text;-webkit-text-fill-color:transparent;
  background-clip:text;
}
.logo-icon{font-size:1.8rem;-webkit-text-fill-color:initial}
.header-actions{display:flex;align-items:center;gap:0.5rem;flex-wrap:wrap}

/* Live indicator */
.live-indicator{
  display:flex;align-items:center;gap:0.4rem;
  font-size:0.75rem;color:var(--text-secondary);
  padding:0.35rem 0.65rem;
  border-radius:9999px;
  border:1px solid var(--border-color);
  background:var(--bg-card);white-space:nowrap;
}
.live-dot{
  width:8px;height:8px;border-radius:50%;
  background:var(--accent-green);flex-shrink:0;
}
.live-dot.refreshing{animation:pulse 0.8s ease-in-out infinite}
@keyframes pulse{0%,100%{opacity:1}50%{opacity:0.3}}

/* Buttons */
.btn{
  display:inline-flex;align-items:center;gap:0.35rem;
  padding:0.4rem 0.8rem;font-size:0.8rem;font-weight:500;
  border-radius:0.5rem;cursor:pointer;border:1px solid var(--border-color);
  background:var(--bg-card);color:var(--text-primary);
  transition:all 0.2s;white-space:nowrap;
  font-family:inherit;
}
.btn:hover{background:var(--bg-hover);border-color:var(--accent-blue)}
.btn svg{width:14px;height:14px;flex-shrink:0}
.btn-theme{font-size:1.1rem;padding:0.3rem 0.6rem;line-height:1}

/* Search */
.search-wrap{
  position:relative;max-width:420px;width:100%;
  margin-bottom:1.25rem;
}
.search-wrap input{
  width:100%;padding:0.6rem 0.9rem 0.6rem 2.4rem;
  border:1px solid var(--border-color);border-radius:0.6rem;
  background:var(--bg-card);color:var(--text-primary);
  font-size:0.9rem;font-family:inherit;
  transition:border-color 0.2s;
}
.search-wrap input:focus{
  outline:none;border-color:var(--accent-blue);
  box-shadow:0 0 0 3px rgba(59,130,246,0.15);
}
.search-wrap input::placeholder{color:var(--text-secondary)}
.search-icon{
  position:absolute;left:0.75rem;top:50%;transform:translateY(-50%);
  color:var(--text-secondary);pointer-events:none;font-size:0.9rem;
}
.search-shortcut{
  position:absolute;right:0.55rem;top:50%;transform:translateY(-50%);
  font-size:0.65rem;color:var(--text-secondary);
  background:var(--bg-secondary);padding:0.1rem 0.35rem;
  border-radius:0.25rem;pointer-events:none;
}

/* Stats grid */
.stats-grid{
  display:grid;grid-template-columns:repeat(auto-fit,minmax(220px,1fr));
  gap:1rem;margin-bottom:1.5rem;
}
.stat-card{
  border-radius:0.8rem;padding:1.25rem;
  box-shadow:var(--shadow);transition:transform 0.2s,box-shadow 0.2s;
  border:1px solid transparent;
  background:linear-gradient(var(--bg-card),var(--bg-card)) padding-box,linear-gradient(135deg,var(--accent-blue),var(--accent-purple)) border-box;
}
.stat-card:hover{transform:translateY(-2px);box-shadow:var(--shadow-lg)}
.stat-label{
  font-size:0.75rem;color:var(--text-secondary);
  text-transform:uppercase;letter-spacing:0.05em;margin-bottom:0.4rem;
}
.stat-value{
  font-size:1.75rem;font-weight:700;
  background:var(--gradient-2);
  -webkit-background-clip:text;-webkit-text-fill-color:transparent;
  background-clip:text;
}
.stat-sub{font-size:0.75rem;color:var(--text-secondary);margin-top:0.2rem}

/* Chart section */
.chart-section{
  border-radius:0.8rem;padding:1.25rem 1.5rem;
  margin-bottom:1.5rem;box-shadow:var(--shadow);
  border:1px solid transparent;
  background:linear-gradient(var(--bg-card),var(--bg-card)) padding-box,linear-gradient(135deg,var(--accent-cyan),var(--accent-blue)) border-box;
}
.chart-title{
  font-size:0.95rem;font-weight:700;margin-bottom:1rem;
  display:flex;align-items:center;gap:0.5rem;
}
.chart-bar-wrap{margin-bottom:0.65rem}
.chart-bar-wrap:last-child{margin-bottom:0}
.chart-bar-label{
  display:flex;justify-content:space-between;
  font-size:0.8rem;margin-bottom:0.2rem;
}
.chart-bar-label-name{font-weight:500;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;max-width:60%;display:inline-flex;align-items:center;gap:0.35rem}
.chart-bar-label-val{color:var(--text-secondary);font-family:'SF Mono',Monaco,monospace;flex-shrink:0}
.chart-bar-track{
  height:10px;background:var(--bg-secondary);
  border-radius:9999px;overflow:hidden;
}
.chart-bar-fill{
  height:100%;border-radius:9999px;
  background:var(--gradient-2);transition:width 0.5s ease;
  min-width:2px;
}

/* Table */
.table-container{
  border-radius:0.8rem;overflow:hidden;box-shadow:var(--shadow);
  border:1px solid transparent;
  background:linear-gradient(var(--bg-card),var(--bg-card)) padding-box,linear-gradient(135deg,var(--accent-blue),var(--accent-purple)) border-box;
}
.table-header{
  padding:1rem 1.5rem;border-bottom:1px solid var(--border-color);
  background:linear-gradient(90deg,rgba(59,130,246,0.07) 0%,transparent 60%);
  display:flex;align-items:center;justify-content:space-between;
  flex-wrap:wrap;gap:0.5rem;
}
.table-title{
  font-size:1.1rem;font-weight:700;
  background:var(--gradient-1);
  -webkit-background-clip:text;-webkit-text-fill-color:transparent;background-clip:text;
}
.table-meta{font-size:0.8rem;color:var(--text-secondary)}
.table-scroll{overflow-x:auto}
table{width:100%;border-collapse:collapse}
th{
  text-align:left;padding:0.75rem 1.25rem;
  background:var(--bg-secondary);color:var(--text-secondary);
  font-size:0.7rem;font-weight:700;text-transform:uppercase;
  letter-spacing:0.06em;cursor:pointer;user-select:none;
  transition:all 0.2s;white-space:nowrap;
  border-bottom:2px solid var(--border-color);
}
th:hover{background:var(--bg-hover);color:var(--text-primary)}
th .sort-arrow{margin-left:0.2rem;opacity:0.3;transition:opacity 0.2s}
th.sort-asc .sort-arrow,th.sort-desc .sort-arrow{opacity:1;color:var(--accent-blue)}
th.sort-asc,th.sort-desc{color:var(--accent-blue);border-bottom-color:var(--accent-blue)}
td{
  padding:0.75rem 1.25rem;border-bottom:1px solid var(--border-color);
  font-size:0.875rem;
  transition:background 0.15s,box-shadow 0.15s;white-space:nowrap;
}
tr.data-row{cursor:pointer;transition:background 0.15s,box-shadow 0.15s}
tr.data-row:hover{background:var(--bg-hover);box-shadow:inset 3px 0 0 var(--accent-blue)}
tr.data-row.expanded{background:var(--bg-hover);box-shadow:inset 3px 0 0 var(--accent-purple)}
tr:last-child td{border-bottom:none}

.model-name{
  font-weight:600;
  background:var(--gradient-2);
  -webkit-background-clip:text;-webkit-text-fill-color:transparent;background-clip:text;
}
.server-cell{display:inline-flex;align-items:center;gap:0.45rem}
.server-name{color:var(--text-primary)}
.provider-logo{
  width:1.35rem;height:1.35rem;display:inline-flex;align-items:center;justify-content:center;
  border-radius:0.35rem;overflow:hidden;flex:0 0 auto;
  box-shadow:0 0 0 1px rgba(255,255,255,0.08);
}
.provider-logo img{width:100%;height:100%;display:block}
.metric{font-family:'SF Mono',Monaco,monospace;font-weight:600}
.metric-tps{color:var(--accent-green)}
.metric-prompt{color:var(--accent-purple)}
.metric-ttft{color:var(--accent-orange)}
.metric-itl{color:var(--accent-cyan)}

/* Detail row */
tr.detail-row{display:none}
tr.detail-row.open{display:table-row}
tr.detail-row td{
  padding:0;border-bottom:1px solid var(--border-color);
  background:var(--bg-secondary);
}
.detail-panel{
  padding:1rem 1.5rem;
  border-left:3px solid var(--accent-blue);
  margin:0.25rem 0;
}
.detail-panel h4{
  font-size:0.85rem;font-weight:600;margin-bottom:0.6rem;
  color:var(--text-secondary);
}
.detail-mini-table{width:100%;border-collapse:collapse;font-size:0.82rem}
.detail-mini-table th{
  background:transparent;padding:0.35rem 0.6rem;font-size:0.68rem;
  border-bottom:1px solid var(--border-color);
}
.detail-mini-table td{
  padding:0.35rem 0.6rem;border-bottom:none;font-size:0.82rem;
}

/* Badges */
.perf-badge{
  display:inline-block;padding:0.15rem 0.45rem;border-radius:9999px;
  font-size:0.6rem;font-weight:700;text-transform:uppercase;
  letter-spacing:0.05em;margin-left:0.3rem;vertical-align:middle;
}
.badge-fastest{background:rgba(245,158,11,0.15);color:var(--accent-orange);border:1px solid rgba(245,158,11,0.3)}
.badge-stable{background:rgba(16,185,129,0.15);color:var(--accent-green);border:1px solid rgba(16,185,129,0.3)}
.badge-context{display:inline-block;padding:0.15rem 0.55rem;border-radius:9999px;font-size:0.7rem;font-weight:600;background:var(--bg-secondary);border:1px solid var(--border-color)}

/* Tooltips */
.has-tooltip{cursor:help;border-bottom:1px dotted rgba(148,163,184,0.5)}
#global-tooltip{
  position:fixed;z-index:9999;pointer-events:none;display:none;
  background:var(--bg-secondary);color:var(--text-primary);
  border:1px solid var(--border-color);box-shadow:var(--shadow);
  font-size:0.75rem;line-height:1.4;padding:0.4rem 0.7rem;
  border-radius:0.4rem;max-width:300px;
  text-transform:none;letter-spacing:normal;font-weight:400;
}

/* Help modal */
.modal-overlay{
  display:none;position:fixed;inset:0;z-index:200;
  background:rgba(0,0,0,0.5);
  align-items:center;justify-content:center;
}
.modal-overlay.open{display:flex}
.modal{
  background:var(--bg-card);border:1px solid var(--border-color);
  border-radius:1rem;padding:1.5rem 2rem;max-width:420px;width:90%;
  box-shadow:var(--shadow-lg);max-height:80vh;overflow-y:auto;
}
.modal h3{font-size:1.1rem;margin-bottom:1rem}
.modal kbd{
  display:inline-block;padding:0.1rem 0.4rem;
  font-size:0.72rem;font-family:'SF Mono',Monaco,monospace;
  background:var(--bg-secondary);border:1px solid var(--border-color);
  border-radius:0.3rem;min-width:1.4rem;text-align:center;
}
.modal-row{display:flex;justify-content:space-between;padding:0.35rem 0;font-size:0.85rem}
.modal-close{
  float:right;background:none;border:none;color:var(--text-secondary);
  font-size:1.2rem;cursor:pointer;padding:0.2rem 0.4rem;
}

/* Toast */
.toast{
  position:fixed;bottom:1.5rem;right:1.5rem;z-index:300;
  background:var(--accent-green);color:#fff;padding:0.6rem 1rem;
  border-radius:0.5rem;font-size:0.85rem;font-weight:500;
  box-shadow:var(--shadow-lg);opacity:0;transform:translateY(10px);
  transition:opacity 0.3s,transform 0.3s;pointer-events:none;
}
.toast.show{opacity:1;transform:translateY(0)}

/* No results */
.no-results{text-align:center;padding:3rem 2rem;color:var(--text-secondary)}
.no-results-icon{font-size:3rem;margin-bottom:0.75rem;opacity:0.5}
.no-results p{font-size:1rem}

/* Footer */
footer{
  background:var(--bg-secondary);border-top:1px solid var(--border-color);
  padding:1.5rem 2rem;margin-top:3rem;text-align:center;
  color:var(--text-secondary);font-size:0.8rem;
}
footer a{color:var(--accent-blue);text-decoration:none}
footer a:hover{color:var(--accent-cyan)}
footer .footer-links{display:flex;justify-content:center;gap:1.5rem;margin-bottom:0.5rem;flex-wrap:wrap}
footer .footer-version{font-family:'SF Mono',Monaco,monospace;font-size:0.7rem;opacity:0.6}

/* Responsive - card layout on mobile */
@media (max-width:768px){
  .container{padding:1rem}
  .stats-grid{grid-template-columns:repeat(2,1fr);gap:0.75rem}
  .stat-card{padding:1rem}.stat-value{font-size:1.35rem}
  .header-content{flex-direction:column;align-items:flex-start}
  .header-actions{width:100%;justify-content:flex-start}
  .search-wrap{max-width:100%}

  /* Card mode for table */
  table thead{display:none}
  table,tbody,tr,td{display:block}
  tr.data-row{
    background:var(--bg-card);border:1px solid var(--border-color);
    border-radius:0.6rem;margin-bottom:0.75rem;padding:0.75rem 1rem;
  }
  tr.data-row td{
    display:flex;justify-content:space-between;align-items:center;
    padding:0.3rem 0;border-bottom:none;white-space:normal;
    font-size:0.85rem;
  }
  tr.data-row td:before{
    content:attr(data-label);font-weight:600;font-size:0.72rem;
    color:var(--text-secondary);text-transform:uppercase;
    letter-spacing:0.04em;margin-right:0.5rem;flex-shrink:0;
  }
  tr.data-row .model-name{font-size:1rem;margin-bottom:0.2rem}
  tr.data-row .model-name:before{display:none}
  tr.detail-row{display:none}
  tr.detail-row.open{display:block;margin-top:-0.5rem;margin-bottom:0.75rem}
  tr.detail-row td{padding:0}
  .detail-panel{border-radius:0 0 0.6rem 0.6rem;border-left:none;border-top:1px solid var(--border-color)}
  .perf-badge{margin-left:0.1rem}

  footer .footer-links{flex-direction:column;gap:0.4rem}
}
@media (max-width:480px){
  .stats-grid{grid-template-columns:1fr}
  .header-actions{gap:0.3rem}
  .btn{font-size:0.72rem;padding:0.3rem 0.55rem}
}
</style>
<noscript><style>.nojs-hide{display:none!important}</style></noscript>
`)

		// --- Embed JSON data ---
		fmt.Fprintf(w, `<script id="results-data" type="application/json">%s</script>
`, string(jsonData))

		// --- JavaScript ---
		fmt.Fprint(w, `<script>
(function(){
var dataEl=document.getElementById('results-data');
var ALL_DATA=dataEl?JSON.parse(dataEl.textContent):[];

// Build result map and latest-per-model/server
var modelMap={};
ALL_DATA.forEach(function(r){
  if(!r.id)r.id=(r.server_name||r.server_host||'server')+' / '+r.model;
  if(!modelMap[r.id])modelMap[r.id]=[];
  modelMap[r.id].push(r);
});
Object.keys(modelMap).forEach(function(k){
  modelMap[k].sort(function(a,b){return b.timestamp.localeCompare(a.timestamp)});
});

function latestPerModel(){
  var res=[];
  Object.keys(modelMap).forEach(function(k){
    var runs=modelMap[k];
    var latest=runs[0];
    for(var i=1;i<runs.length;i++){
      if(runs[i].timestamp>latest.timestamp)latest=runs[i];
    }
    res.push(latest);
  });
  return res;
}

// State
var state={
  sortCol:3, sortDir:'desc', filter:'',
  theme:localStorage.getItem('lmspeedtest-theme')||'dark',
  expandedModel:null, lastPoll:Date.now()
};

// Column types for sort (0=model,1=server,2=context,3=tps,4=prompt,5=ttft,6=itl,7=timestamp)
var colTypes=['string','string','number','number','number','duration','duration','string'];
var colKeys=['model','server_name','context','tps','prompt_eval_tps','ttft_ms','itl_ms','timestamp'];

function parseDur(d){
  if(!d||d==='-')return 0;
  var m=d.match(/([\d.]+)(ms|s)/);
  if(!m)return 0;
  var v=parseFloat(m[1]);
  return m[2]==='s'?v*1000:v;
}

function formatDur(ms){
  if(ms===0||ms===undefined||ms===null)return '-';
  if(ms>=1000)return (ms/1000).toFixed(2)+'s';
  return Math.round(ms)+'ms';
}

var PROVIDER_LOGOS={
  ollama:"`+ollamaLogoDataURI+`",
  lmstudio:"`+lmStudioLogoDataURI+`",
  unknown:"`+genericLogoDataURI+`"
};

function providerName(provider){
  if(provider==='lmstudio')return 'LM Studio';
  if(provider==='ollama')return 'Ollama';
  return provider||'Unknown';
}

function providerLogoHTML(provider){
  provider=provider||'unknown';
  var key=PROVIDER_LOGOS[provider]?provider:'unknown';
  var label=providerName(provider);
  return '<span class="provider-logo provider-logo-'+escHtml(key)+'" title="'+escHtml(label)+'">'+
    '<img src="'+PROVIDER_LOGOS[key]+'" alt="'+escHtml(label)+'"></span>';
}

function serverCellHTML(r){
  var name=r.server_name||r.server_host||'-';
  return '<span class="server-cell">'+providerLogoHTML(r.server_provider)+
    '<span class="server-name">'+escHtml(name)+'</span></span>';
}

function getVal(row,col){
  var k=colKeys[col];
  if(!k)return row.model||'';
  var raw=row[k];
  if(colTypes[col]==='number')return parseFloat(raw)||0;
  if(colTypes[col]==='duration')return typeof raw==='number'?raw:parseDur(raw);
  return String(raw||'');
}

// Badge logic
function computeBadges(latest){
  var fastest='',fastestTps=0;
  var stable='',stableStd=Infinity;
  latest.forEach(function(r){
    if(r.tps>fastestTps){fastestTps=r.tps;fastest=r.id;}
  });
  Object.keys(modelMap).forEach(function(m){
    var runs=modelMap[m];
    if(runs.length<2)return;
    var s=0;runs.forEach(function(r){s+=r.tps});
    var mean=s/runs.length;
    var v=0;runs.forEach(function(r){var d=r.tps-mean;v+=d*d});
    var std=Math.sqrt(v/runs.length);
    if(std<stableStd){stableStd=std;stable=m;}
  });
  return{fastest:fastest,stable:stable};
}

// Render table
function renderTable(data){
  var tbody=document.querySelector('#results-tbody');
  if(!tbody)return;
  tbody.innerHTML='';

  var badges=computeBadges(data);

  data.forEach(function(r){
    var tr=document.createElement('tr');
    tr.className='data-row';
    tr.setAttribute('role','button');
    tr.setAttribute('tabindex','0');
    tr.setAttribute('aria-expanded',state.expandedModel===r.id?'true':'false');
    tr.setAttribute('data-model',r.id);
    if(state.expandedModel===r.id)tr.classList.add('expanded');

    var badgeHtml='';
    if(r.id===badges.fastest)badgeHtml+='<span class="perf-badge badge-fastest" aria-label="Fastest">Fastest</span>';
    if(r.id===badges.stable)badgeHtml+='<span class="perf-badge badge-stable" aria-label="Most Stable">Stable</span>';

    tr.innerHTML=
      '<td class="model-name" data-label="Model">'+escHtml(r.model)+badgeHtml+'</td>'+
      '<td data-label="Server">'+serverCellHTML(r)+'</td>'+
      '<td data-label="Context"><span class="badge-context">'+Math.round(r.context/1024)+'k</span></td>'+
      '<td class="metric metric-tps" data-label="Tokens/sec">'+r.tps.toFixed(2)+'</td>'+
      '<td class="metric metric-prompt" data-label="Prompt TPS">'+r.prompt_eval_tps.toFixed(2)+'</td>'+
      '<td class="metric metric-ttft" data-label="TTFT">'+formatDur(r.ttft_ms)+'</td>'+
      '<td class="metric metric-itl" data-label="ITL">'+formatDur(r.itl_ms)+'</td>'+
      '<td data-label="Tested">'+formatTimestamp(r.timestamp)+'</td>';
    tbody.appendChild(tr);

    // Detail row
    var detailTr=document.createElement('tr');
    detailTr.className='detail-row'+(state.expandedModel===r.id?' open':'');
    detailTr.setAttribute('data-model',r.id);
    detailTr.innerHTML='<td colspan="8">'+buildDetailHTML(r.id)+'</td>';
    tbody.appendChild(detailTr);
  });

  // Bind click handlers
  tbody.querySelectorAll('tr.data-row').forEach(function(tr){
    tr.addEventListener('click',function(e){toggleDetail(tr.getAttribute('data-model'))});
    tr.addEventListener('keydown',function(e){
      if(e.key==='Enter'||e.key===' '){e.preventDefault();toggleDetail(tr.getAttribute('data-model'))}
    });
  });

  updateRowCount(data.length);
}

function buildDetailHTML(model){
  var runs=modelMap[model]||[];
  var html='<div class="detail-panel"><h4>Historical Runs ('+runs.length+')</h4>'+
    '<table class="detail-mini-table"><thead><tr>'+
    '<th>Date</th><th>Context</th>'+
    '<th><span class="has-tooltip" data-tooltip="Tokens per second — generation speed">TPS</span></th>'+
    '<th><span class="has-tooltip" data-tooltip="Prompt TPS — input processing speed">Prompt TPS</span></th>'+
    '<th><span class="has-tooltip" data-tooltip="Time To First Token — delay before response starts">TTFT</span></th>'+
    '<th><span class="has-tooltip" data-tooltip="Inter-Token Latency — avg time between tokens">ITL</span></th>'+
    '</tr></thead><tbody>';
  runs.forEach(function(r){
    html+='<tr>'+
      '<td>'+formatTimestamp(r.timestamp)+'</td>'+
      '<td>'+Math.round(r.context/1024)+'k</td>'+
      '<td class="metric metric-tps">'+r.tps.toFixed(2)+'</td>'+
      '<td class="metric metric-prompt">'+r.prompt_eval_tps.toFixed(2)+'</td>'+
      '<td class="metric metric-ttft">'+formatDur(r.ttft_ms)+'</td>'+
      '<td class="metric metric-itl">'+formatDur(r.itl_ms)+'</td>'+
      '</tr>';
  });
  html+='</tbody></table></div>';
  return html;
}

function toggleDetail(model){
  if(state.expandedModel===model){
    state.expandedModel=null;
  }else{
    state.expandedModel=model;
  }
  refreshView();
}

function updateRowCount(n){
  var el=document.getElementById('row-count');
  if(el)el.textContent=n+' result'+(n!==1?'s':'');
}

// Render chart
function renderChart(data){
  var chart=document.getElementById('chart-bars');
  if(!chart)return;
  if(!data.length){chart.innerHTML='';return}
  var maxTps=0;
  data.forEach(function(r){if(r.tps>maxTps)maxTps=r.tps});
  if(maxTps===0)maxTps=1;

  chart.innerHTML='';
  data.forEach(function(r){
    var pct=(r.tps/maxTps*100).toFixed(1);
    var div=document.createElement('div');
    div.className='chart-bar-wrap';
    div.setAttribute('data-model',r.id);
    div.innerHTML=
      '<div class="chart-bar-label">'+
        '<span class="chart-bar-label-name">'+providerLogoHTML(r.server_provider)+escHtml(r.model)+' <small>'+escHtml(r.server_name||r.server_host||'')+'</small></span>'+
        '<span class="chart-bar-label-val">'+r.tps.toFixed(1)+' tok/s</span>'+
      '</div>'+
      '<div class="chart-bar-track">'+
        '<div class="chart-bar-fill" style="width:'+pct+'%" role="img" aria-label="'+escHtml(r.id)+' relative speed '+pct+'%"></div>'+
      '</div>';
    chart.appendChild(div);
  });
}

// Sort & filter
function getFilteredSorted(){
  var latest=latestPerModel();
  if(state.filter){
    var f=state.filter.toLowerCase();
    latest=latest.filter(function(r){
      return r.model.toLowerCase().indexOf(f)!==-1 ||
        String(r.server_name||r.server_host||'').toLowerCase().indexOf(f)!==-1;
    });
  }
  var col=state.sortCol;
  var dir=state.sortDir==='asc'?1:-1;
  latest.sort(function(a,b){
    var va=getVal(a,col),vb=getVal(b,col);
    if(va<vb)return -1*dir;
    if(va>vb)return 1*dir;
    return 0;
  });
  return latest;
}

function refreshView(){
  var data=getFilteredSorted();
  renderTable(data);
  renderChart(data);
  updateSortIndicators();
  updateHash();
}

function updateSortIndicators(){
  document.querySelectorAll('th').forEach(function(th,i){
    th.classList.remove('sort-asc','sort-desc');
    if(i===state.sortCol){
      th.classList.add('sort-'+state.sortDir);
    }
  });
}

// Sort header click
function initSortHeaders(){
  document.querySelectorAll('th').forEach(function(th,i){
    th.addEventListener('click',function(){
      if(state.sortCol===i){
        state.sortDir=state.sortDir==='asc'?'desc':'asc';
      }else{
        state.sortCol=i;
        state.sortDir='desc';
      }
      refreshView();
    });
  });
}

// Search filter
function initSearch(){
  var input=document.getElementById('filter-input');
  if(!input)return;
  if(state.filter)input.value=state.filter;
  input.addEventListener('input',function(){
    state.filter=this.value.trim();
    refreshView();
  });
}

// URL hash
function parseHash(){
  var h=location.hash.replace(/^#/,'');
  if(!h)return{};
  var obj={};
  h.split('&').forEach(function(p){
    var parts=p.split('=');
    if(parts.length===2)obj[parts[0]]=decodeURIComponent(parts[1]);
  });
  if(obj.sc!==undefined)obj.sortCol=parseInt(obj.sc);
  return obj;
}

function updateHash(){
  var parts=[];
  if(state.sortCol!==3)parts.push('sc='+state.sortCol);
  if(state.sortDir!=='desc')parts.push('sd='+state.sortDir);
  if(state.filter)parts.push('f='+encodeURIComponent(state.filter));
  var h=parts.length?'#'+parts.join('&'):'';
  if(location.hash!==h){
    history.replaceState(null,'',h||location.pathname);
  }
}

// Export
function downloadFile(content,filename,mime){
  var blob=new Blob([content],{type:mime});
  var url=URL.createObjectURL(blob);
  var a=document.createElement('a');
  a.href=url;a.download=filename;
  document.body.appendChild(a);a.click();
  document.body.removeChild(a);
  setTimeout(function(){URL.revokeObjectURL(url)},100);
}

function exportCSV(){
  var flat=getFilteredSorted();
  var lines=['server_name,server_host,server_provider,model,context,tokens_per_sec,prompt_eval_tps,ttft_ms,itl_ms,timestamp'];
  flat.forEach(function(r){
    lines.push([r.server_name||'',r.server_host||'',r.server_provider||'',r.model,r.context,r.tps.toFixed(2),r.prompt_eval_tps.toFixed(2),
      r.ttft_ms.toFixed(0),r.itl_ms.toFixed(0),r.timestamp].join(','));
  });
  downloadFile(lines.join('\n'),'lmspeedtest-results.csv','text/csv');
}

function exportJSON(){
  downloadFile(JSON.stringify(getFilteredSorted(),null,2),'lmspeedtest-results.json','application/json');
}

function copyMarkdown(){
  var flat=getFilteredSorted();
  var lines=['| Model | Server | Context | Tokens/sec | Prompt TPS | TTFT | ITL | Tested |',
             '|-------|--------|---------|------------|------------|------|-----|--------|'];
  flat.forEach(function(r){
    lines.push('| '+r.model+' | '+(r.server_name||r.server_host||'-')+' | '+Math.round(r.context/1024)+'k | '+
      r.tps.toFixed(2)+' | '+r.prompt_eval_tps.toFixed(2)+' | '+
      formatDur(r.ttft_ms)+' | '+formatDur(r.itl_ms)+' | '+formatTimestamp(r.timestamp)+' |');
  });
  var text=lines.join('\n');
  if(navigator.clipboard&&navigator.clipboard.writeText){
    navigator.clipboard.writeText(text).then(function(){showToast('Copied Markdown table!')});
  }else{
    showToast('Clipboard not available');
  }
}

// Theme
function applyTheme(t){
  document.documentElement.setAttribute('data-theme',t);
  state.theme=t;
  localStorage.setItem('lmspeedtest-theme',t);
  var btn=document.getElementById('theme-toggle');
  if(btn)btn.setAttribute('aria-label',t==='dark'?'Switch to light mode':'Switch to dark mode');
}

function toggleTheme(){
  applyTheme(state.theme==='dark'?'light':'dark');
}

// Live polling
function pollResults(){
  var dot=document.querySelector('.live-dot');
  var text=document.getElementById('live-text');
  if(dot)dot.classList.add('refreshing');

  fetch('/api/results')
    .then(function(r){return r.json()})
    .then(function(newData){
      var changed=JSON.stringify(newData)!==JSON.stringify(ALL_DATA);
      if(changed){
        ALL_DATA=newData;
        modelMap={};
        ALL_DATA.forEach(function(r){
          if(!r.id)r.id=(r.server_name||r.server_host||'server')+' / '+r.model;
          if(!modelMap[r.id])modelMap[r.id]=[];
          modelMap[r.id].push(r);
        });
        Object.keys(modelMap).forEach(function(k){
          modelMap[k].sort(function(a,b){return b.timestamp.localeCompare(a.timestamp)});
        });
        refreshView();
        updateStats();
      }
      state.lastPoll=Date.now();
      if(text)text.textContent='Updated just now';
    })
    .catch(function(){
      if(text)text.textContent='Update failed';
    })
    .finally(function(){
      if(dot)dot.classList.remove('refreshing');
    });
}

function updateLiveTimer(){
  var text=document.getElementById('live-text');
  if(!text)return;
  var elapsed=Math.floor((Date.now()-state.lastPoll)/1000);
  if(elapsed<5)text.textContent='Updated just now';
  else if(elapsed<60)text.textContent='Updated '+elapsed+'s ago';
  else text.textContent='Updated '+Math.floor(elapsed/60)+'m ago';
}

// Stats update (after poll)
function updateStats(){
  var latest=latestPerModel();
  if(!latest.length)return;
  var sumTps=0,sumPrompt=0,sumTtft=0;
  latest.forEach(function(r){sumTps+=r.tps;sumPrompt+=r.prompt_eval_tps;sumTtft+=r.ttft_ms});
  var n=latest.length;
  updateStatEl('stat-count',n);
  updateStatEl('stat-avg-tps',(sumTps/n).toFixed(1));
  updateStatEl('stat-avg-prompt',(sumPrompt/n).toFixed(1));
  updateStatEl('stat-avg-ttft',formatDur(sumTtft/n));
}

function updateStatEl(id,val){
  var el=document.getElementById(id);
  if(el)el.textContent=val;
}

// Toast
var toastTimer;
function showToast(msg){
  var t=document.getElementById('toast');
  if(!t)return;
  t.textContent=msg;t.classList.add('show');
  clearTimeout(toastTimer);
  toastTimer=setTimeout(function(){t.classList.remove('show')},2000);
}

// Modal
function showHelp(){
  document.getElementById('help-modal').classList.add('open');
}
function hideHelp(){
  document.getElementById('help-modal').classList.remove('open');
}

// Keyboard shortcuts
function handleKeyboard(e){
  if(e.target.tagName==='INPUT'||e.target.tagName==='TEXTAREA'){
    if(e.key==='Escape')e.target.blur();
    return;
  }
  if(e.key==='/'||(e.key==='k'&&(e.metaKey||e.ctrlKey))){
    e.preventDefault();
    var inp=document.getElementById('filter-input');
    if(inp)inp.focus();
  }else if(e.key==='?'){
    e.preventDefault();showHelp();
  }else if(e.key==='Escape'){
    hideHelp();
    state.expandedModel=null;refreshView();
  }else if(e.key==='r'&&!e.metaKey&&!e.ctrlKey){
    pollResults();
  }
}

// Helpers
function escHtml(s){return String(s).replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;')}
function formatTimestamp(ts){
  if(!ts)return '-';
  var d=new Date(ts);
  if(isNaN(d.getTime()))return ts;
  return d.getFullYear()+'-'+
    String(d.getMonth()+1).padStart(2,'0')+'-'+
    String(d.getDate()).padStart(2,'0')+' '+
    String(d.getHours()).padStart(2,'0')+':'+
    String(d.getMinutes()).padStart(2,'0');
}

// Tooltip
function initTooltip(){
  var tt=document.createElement('div');
  tt.id='global-tooltip';
  document.body.appendChild(tt);
  document.addEventListener('mouseover',function(e){
    var el=e.target.closest('[data-tooltip]');
    if(el){tt.textContent=el.getAttribute('data-tooltip');tt.style.display='block';}
  });
  document.addEventListener('mouseout',function(e){
    if(e.target.closest('[data-tooltip]'))tt.style.display='none';
  });
  document.addEventListener('mousemove',function(e){
    if(tt.style.display!=='none'){
      tt.style.left=(e.clientX+14)+'px';
      tt.style.top=(e.clientY-38)+'px';
    }
  });
}

// Init
function init(){
  // Read hash state
  var hs=parseHash();
  if(hs.sortCol!==undefined)state.sortCol=hs.sortCol;
  if(hs.sd)state.sortDir=hs.sd;
  if(hs.f)state.filter=hs.f;

  applyTheme(state.theme);

  initSortHeaders();
  initSearch();
  initTooltip();

  document.getElementById('theme-toggle').addEventListener('click',toggleTheme);
  document.getElementById('btn-csv').addEventListener('click',exportCSV);
  document.getElementById('btn-json').addEventListener('click',exportJSON);
  document.getElementById('btn-md').addEventListener('click',copyMarkdown);
  document.getElementById('btn-refresh').addEventListener('click',pollResults);
  document.getElementById('help-close').addEventListener('click',hideHelp);
  document.getElementById('help-modal').addEventListener('click',function(e){
    if(e.target===this)hideHelp();
  });

  document.addEventListener('keydown',handleKeyboard);
  document.getElementById('btn-help').addEventListener('click',showHelp);

  refreshView();
  setInterval(pollResults,30000);
  setInterval(updateLiveTimer,5000);
}

if(document.readyState==='loading'){
  document.addEventListener('DOMContentLoaded',init);
}else{
  init();
}
})();
</script>
</head>
<body>
`)

		// --- Server-rendered HTML body ---
		if len(latest) == 0 {
			fmt.Fprint(w, `
<header>
  <div class="header-content">
    <div class="logo"><span class="logo-icon">&#128640;</span><span>LMSpeedTest</span></div>
  </div>
</header>
<div class="container">
  <div class="no-results">
    <div class="no-results-icon">&#128640;</div>
    <p>No benchmark results yet. Run <code>lmspeedtest test &lt;size&gt;</code> to get started!</p>
  </div>
</div>
`)
		} else {
			avgTPS := totalTPS / float64(len(latest))
			avgPromptTPS := totalPromptTPS / float64(len(latest))
			avgTTFT := totalTTFT / time.Duration(len(latest))
			maxChartTPS := latest[0].TPS
			if maxChartTPS == 0 {
				maxChartTPS = 1
			}

			fmt.Fprint(w, `
<header>
  <div class="header-content">
    <div class="logo"><span class="logo-icon">&#128640;</span><span>LMSpeedTest</span></div>
    <div class="header-actions">
      <div class="live-indicator" aria-live="polite">
        <span class="live-dot" aria-hidden="true"></span>
        <span id="live-text">Live</span>
      </div>
      <button class="btn btn-theme" id="theme-toggle" aria-label="Switch to light mode" title="Toggle dark/light mode">&#9788;</button>
      <button class="btn" id="btn-refresh" title="Refresh data" aria-label="Refresh">&#8635; Refresh</button>
      <button class="btn" id="btn-csv" title="Download CSV" aria-label="Download CSV">&#128229; CSV</button>
      <button class="btn" id="btn-json" title="Download JSON" aria-label="Download JSON">&#128229; JSON</button>
      <button class="btn" id="btn-md" title="Copy as Markdown table" aria-label="Copy Markdown">&#128203; Copy MD</button>
      <button class="btn" id="btn-help" title="Keyboard shortcuts" aria-label="Help">?</button>
    </div>
  </div>
</header>
<div class="container">
`)
			fmt.Fprintf(w, `
  <div class="stats-grid">
    <div class="stat-card">
      <div class="stat-label">Models Benchmarked</div>
      <div class="stat-value" id="stat-count">%d</div>
      <div class="stat-sub">Model/server rows tested</div>
    </div>
    <div class="stat-card">
      <div class="stat-label">Avg Generation TPS</div>
      <div class="stat-value" id="stat-avg-tps">%.1f</div>
      <div class="stat-sub">Tokens per second</div>
    </div>
    <div class="stat-card">
      <div class="stat-label">Avg Prompt TPS</div>
      <div class="stat-value" id="stat-avg-prompt">%.1f</div>
      <div class="stat-sub">Input processing speed</div>
    </div>
    <div class="stat-card">
      <div class="stat-label">Avg TTFT</div>
      <div class="stat-value" id="stat-avg-ttft">%s</div>
      <div class="stat-sub">Time to first token</div>
    </div>
  </div>
`, len(latest), avgTPS, avgPromptTPS, formatDuration(avgTTFT))

			fmt.Fprint(w, `
  <div class="chart-section">
    <div class="chart-title">&#128202; Generation Speed Comparison</div>
    <div id="chart-bars">
`)
			for _, r := range latest {
				pct := (r.TPS / maxChartTPS) * 100
				fmt.Fprintf(w, `      <div class="chart-bar-wrap" data-model="%s">
        <div class="chart-bar-label">
          <span class="chart-bar-label-name">%s%s <small>%s</small></span>
          <span class="chart-bar-label-val">%.1f tok/s</span>
        </div>
        <div class="chart-bar-track">
          <div class="chart-bar-fill" style="width:%.1f%%" role="img" aria-label="%s relative speed %.1f%%"></div>
        </div>
      </div>
`, resultDisplayKey(r), providerLogoHTML(resultServerProvider(r)), html.EscapeString(r.Model), html.EscapeString(resultServerLabel(r)), r.TPS, pct, resultDisplayKey(r), pct)
			}
			fmt.Fprint(w, `    </div>
  </div>
`)

			fmt.Fprint(w, `
  <div class="search-wrap">
    <span class="search-icon" aria-hidden="true">&#128269;</span>
    <input type="search" id="filter-input" placeholder="Filter by model or server..." aria-label="Filter models by name or server">
    <span class="search-shortcut nojs-hide">/</span>
  </div>
`)

			fmt.Fprintf(w, `
  <div class="table-container">
    <div class="table-header">
      <div class="table-title">&#128202;&nbsp;Benchmark Results</div>
      <div class="table-meta"><span id="row-count">%d results</span></div>
    </div>
    <div class="table-scroll">
      <table role="grid" aria-label="Benchmark results table">
        <thead>
          <tr>
            <th scope="col" aria-sort="none">Model <span class="sort-arrow" aria-hidden="true">&#8597;</span></th>
            <th scope="col" aria-sort="none">Server <span class="sort-arrow" aria-hidden="true">&#8597;</span></th>
            <th scope="col" aria-sort="none">Context <span class="sort-arrow" aria-hidden="true">&#8597;</span></th>
            <th scope="col" aria-sort="descending"><span class="has-tooltip" data-tooltip="Generation speed — output tokens produced per second">Tokens/sec</span> <span class="sort-arrow" aria-hidden="true">&#8597;</span></th>
            <th scope="col" aria-sort="none"><span class="has-tooltip" data-tooltip="Prompt TPS — how fast the model processes your input prompt">Prompt TPS</span> <span class="sort-arrow" aria-hidden="true">&#8597;</span></th>
            <th scope="col" aria-sort="none"><span class="has-tooltip" data-tooltip="Time To First Token — delay before the model starts generating a response">TTFT</span> <span class="sort-arrow" aria-hidden="true">&#8597;</span></th>
            <th scope="col" aria-sort="none"><span class="has-tooltip" data-tooltip="Inter-Token Latency — average time between each generated token">ITL</span> <span class="sort-arrow" aria-hidden="true">&#8597;</span></th>
            <th scope="col" aria-sort="none">Tested <span class="sort-arrow" aria-hidden="true">&#8597;</span></th>
          </tr>
        </thead>
        <tbody id="results-tbody">
`, len(latest))

			// Server-rendered rows (for noscript)
			for _, r := range latest {
				var badges string
				if resultDisplayKey(r) == fastestModel {
					badges += `<span class="perf-badge badge-fastest">Fastest</span>`
				}
				if resultDisplayKey(r) == mostStableModel {
					badges += `<span class="perf-badge badge-stable">Stable</span>`
				}
				fmt.Fprintf(w, `          <tr class="data-row" data-model="%s" role="button" tabindex="0" aria-expanded="false">
            <td class="model-name" data-label="Model">%s%s</td>
            <td data-label="Server"><span class="server-cell">%s<span class="server-name">%s</span></span></td>
            <td data-label="Context"><span class="badge-context">%dk</span></td>
            <td class="metric metric-tps" data-label="Tokens/sec">%.2f</td>
            <td class="metric metric-prompt" data-label="Prompt TPS">%.2f</td>
            <td class="metric metric-ttft" data-label="TTFT">%s</td>
            <td class="metric metric-itl" data-label="ITL">%s</td>
            <td data-label="Tested">%s</td>
          </tr>
`, resultDisplayKey(r), html.EscapeString(r.Model), badges, providerLogoHTML(resultServerProvider(r)), html.EscapeString(resultServerLabel(r)), r.Context/1024, r.TPS, r.PromptEvalTPS, formatDuration(r.TTFT), formatDuration(r.ITL), r.Timestamp.Format("2006-01-02 15:04"))
			}
			fmt.Fprint(w, `        </tbody>
      </table>
    </div>
  </div>
`)
		}

		fmt.Fprint(w, `
  <div class="modal-overlay" id="help-modal" role="dialog" aria-modal="true" aria-label="Keyboard shortcuts">
    <div class="modal">
      <button class="modal-close" id="help-close" aria-label="Close">&times;</button>
      <h3>Keyboard Shortcuts</h3>
      <div class="modal-row"><span>Focus search</span><span><kbd>/</kbd> or <kbd>Ctrl+K</kbd></span></div>
      <div class="modal-row"><span>Show this help</span><span><kbd>?</kbd></span></div>
      <div class="modal-row"><span>Close / collapse</span><span><kbd>Esc</kbd></span></div>
      <div class="modal-row"><span>Refresh data</span><span><kbd>r</kbd></span></div>
      <div class="modal-row"><span>Toggle theme</span><span>button</span></div>
      <div class="modal-row"><span>Expand detail</span><span>click row</span></div>
    </div>
  </div>
`)

		fmt.Fprint(w, `  <div class="toast" id="toast" aria-live="polite"></div>
`)

		fmt.Fprintf(w, `
</div>
<footer>
  <div class="footer-links">
    <a href="https://github.com/notfixingit3/lmspeedtest" target="_blank" rel="noopener">GitHub</a>
    <a href="https://www.buymeacoffee.com/notfixingit" target="_blank" rel="noopener">Buy Me a Coffee</a>
  </div>
  <div class="footer-version">LMSpeedTest v%s — Built with Go</div>
</footer>
</body>
</html>`, version)
	})

	fmt.Printf("\n%s\n", titleStyle.Render("🌐 LMSpeedTest Web Dashboard"))
	fmt.Printf("%s http://localhost:%s\n", infoStyle.Render("Serving on"), port)
	fmt.Println(infoStyle.Render("Press Ctrl+C to stop"))
	server := &http.Server{
		Addr:         ":" + port,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 30 * time.Second, // Increased for complex pages
		IdleTimeout:  120 * time.Second,
	}
	if err := server.ListenAndServe(); err != nil {
		printError("Server error", err)
	}
}
