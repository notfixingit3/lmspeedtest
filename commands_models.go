package main

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	huh "charm.land/huh/v2"
)

func newAPIRequest(method, url string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}
	profile := activeProfile()
	if profile.Token != "" {
		req.Header.Set("Authorization", "Bearer "+profile.Token)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return req, nil
}

func fetchModels() []Model {
	profile := activeProfile()
	if profile.Provider == "lmstudio" {
		return fetchLMStudioModels(profile)
	}
	req, err := newAPIRequest("GET", profile.Host+"/api/tags", nil)
	if err != nil {
		fmt.Println(errorStyle.Render("❌ Cannot create request:") + " " + err.Error())
		return nil
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Println(errorStyle.Render(fmt.Sprintf("❌ Cannot connect to %s:", providerDisplayName(profile.Provider))) + " " + err.Error())
		return nil
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			fmt.Println(warningStyle.Render("⚠️ Cannot close response body:") + " " + err.Error())
		}
	}()

	var data ModelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		fmt.Println(errorStyle.Render("❌ Cannot decode models response:") + " " + err.Error())
		return nil
	}

	var filtered []Model
	for _, m := range data.Models {
		if m.Size > 0 {
			filtered = append(filtered, m)
		}
	}
	return filtered
}

func modelsCmd() {
	maxGB := 0.0
	nameFilter := ""
	if len(os.Args) > 2 {
		var err error
		maxGB, err = strconv.ParseFloat(os.Args[2], 64)
		if err != nil {
			maxGB = 0.0
			nameFilter = strings.ToLower(os.Args[2])
		}
	}
	if len(os.Args) > 3 {
		nameFilter = strings.ToLower(os.Args[3])
	}

	allModels := fetchModels()
	if len(allModels) == 0 {
		fmt.Println(warningStyle.Render("No local models found."))
		return
	}

	profile := activeProfile()
	fmt.Printf("\n%s\n", titleStyle.Render("📋 Models on "+profile.Host))
	fmt.Println(separatorStyle.Render(strings.Repeat("─", 120)))
	fmt.Printf("%s %s %s %s\n",
		headerStyle.Render(fmt.Sprintf("%-50s", "MODEL")),
		headerStyle.Render(fmt.Sprintf("%8s", "SIZE")),
		headerStyle.Render(fmt.Sprintf("%12s", "PARAMS")),
		headerStyle.Render(fmt.Sprintf("%18s", "QUANT")))
	fmt.Println(separatorStyle.Render(strings.Repeat("─", 120)))

	displayed := 0
	for _, m := range allModels {
		gb := float64(m.Size) / (1024 * 1024 * 1024)
		if maxGB > 0 && gb > maxGB {
			continue
		}
		if nameFilter != "" && !strings.Contains(strings.ToLower(m.Name), nameFilter) {
			continue
		}
		params := m.Details.ParameterSize
		if params == "" {
			params = "-"
		}
		quant := m.Details.QuantizationLevel
		if quant == "" {
			quant = "-"
		}
		fmt.Printf("%s %s %s %s\n",
			modelNameStyle.Render(fmt.Sprintf("%-50s", truncateString(m.Name, 50))),
			metricStyle.Render(fmt.Sprintf("%8.1f", gb)),
			infoStyle.Render(fmt.Sprintf("%12s", params)),
			infoStyle.Render(fmt.Sprintf("%18s", quant)))
		displayed++
	}

	fmt.Println(separatorStyle.Render(strings.Repeat("─", 120)))
	switch {
	case maxGB > 0 && nameFilter != "":
		fmt.Printf("%s %s\n",
			infoStyle.Render(fmt.Sprintf("Showing %d of %d local models", displayed, len(allModels))),
			infoStyle.Render(fmt.Sprintf("(≤ %.1f GB, matching '%s')", maxGB, nameFilter)))
	case maxGB > 0:
		fmt.Printf("%s %s\n",
			infoStyle.Render(fmt.Sprintf("Showing %d of %d local models", displayed, len(allModels))),
			infoStyle.Render(fmt.Sprintf("(≤ %.1f GB)", maxGB)))
	case nameFilter != "":
		fmt.Printf("%s %s\n",
			infoStyle.Render(fmt.Sprintf("Showing %d of %d local models", displayed, len(allModels))),
			infoStyle.Render(fmt.Sprintf("(matching '%s')", nameFilter)))
	default:
		fmt.Println(infoStyle.Render(fmt.Sprintf("Total local models: %d", len(allModels))))
	}
}

func testCmd() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: lmspeedtest test <max_gb> [context_size] [name_filter] [--epochs N] [--template code|chat|long]")
		return
	}
	maxGB, err := strconv.ParseFloat(os.Args[2], 64)
	if err != nil {
		fmt.Println("Invalid max_gb value:", os.Args[2])
		return
	}

	contextSize := 32768
	var nameFilters []string
	epochs := 1
	template := "long"
	allMode := false
	promptFile := ""
	argIdx := 3
	for argIdx < len(os.Args) {
		arg := os.Args[argIdx]
		if arg == "--all" {
			allMode = true
			argIdx++
			continue
		}
		if arg == "--prompt-file" && argIdx+1 < len(os.Args) {
			promptFile = os.Args[argIdx+1]
			argIdx += 2
			continue
		}
		if arg == "--epochs" && argIdx+1 < len(os.Args) {
			var err error
			epochs, err = strconv.Atoi(os.Args[argIdx+1])
			if err != nil || epochs < 1 {
				epochs = 1
			}
			argIdx += 2
			continue
		}
		if arg == "--template" && argIdx+1 < len(os.Args) {
			template = os.Args[argIdx+1]
			argIdx += 2
			continue
		}
		ctxStr := strings.ToLower(arg)
		ctxStr = strings.TrimSuffix(ctxStr, "k")
		ctxVal, err := strconv.Atoi(ctxStr)
		if err == nil {
			contextSize = ctxVal * 1024
			argIdx++
			continue
		}
		nameFilter := strings.ToLower(arg)
		if strings.Contains(nameFilter, ",") {
			nameFilters = strings.Split(nameFilter, ",")
		} else {
			nameFilters = []string{nameFilter}
		}
		argIdx++
	}

	prompt := getPrompt(template)
	if promptFile != "" {
		promptFile = filepath.Clean(promptFile)
		data, err := os.ReadFile(promptFile)
		if err != nil {
			fmt.Println(errorStyle.Render("Cannot read prompt file:") + " " + err.Error())
			return
		}
		prompt = string(data)
	}

	models := fetchModels()
	var candidates []Model
	for _, m := range models {
		if float64(m.Size)/(1024*1024*1024) <= maxGB {
			candidates = append(candidates, m)
		}
	}
	if len(nameFilters) > 0 {
		var filtered []Model
		for _, m := range candidates {
			lowerName := strings.ToLower(m.Name)
			for _, f := range nameFilters {
				if strings.Contains(lowerName, f) {
					filtered = append(filtered, m)
					break
				}
			}
		}
		candidates = filtered
	}

	if len(candidates) == 0 {
		fmt.Println(warningStyle.Render("No local models found matching the criteria."))
		return
	}

	var selected []string
	ctxLabel := fmt.Sprintf("%dk", contextSize/1024)

	if allMode {
		for _, m := range candidates {
			selected = append(selected, m.Name)
		}
		fmt.Printf("\n%s %s\n",
			infoStyle.Render("→ Batch mode:"),
			fmt.Sprintf("benchmarking %d models", len(selected)))
	} else {
		const quitSentinel = "__quit__"
		var options []huh.Option[string]
		profile := activeProfile()
		for _, m := range candidates {
			gb := float64(m.Size) / (1024 * 1024 * 1024)
			label := fmt.Sprintf("%s (%.2f GB)", m.Name, gb)
			if serverData, ok := results[profile.Name]; ok {
				if _, hasResults := serverData[m.Name]; hasResults {
					label += " ✓"
				}
			}
			options = append(options, huh.NewOption(label, m.Name))
		}
		options = append(options, huh.NewOption("Quit", quitSentinel))

		form := huh.NewForm(
			huh.NewGroup(
				huh.NewMultiSelect[string]().
					Title("Select models to benchmark").
					Description(fmt.Sprintf("Local models ≤ %.1f GB • %s context • %d epoch(s)", maxGB, ctxLabel, epochs)).
					Options(options...).
					Value(&selected).
					Height(10),
			),
		)

		fmt.Println()
		if err := form.Run(); err != nil {
			fmt.Println(infoStyle.Render("Cancelled."))
			return
		}
		for _, s := range selected {
			if s == quitSentinel {
				fmt.Println(infoStyle.Render("Cancelled."))
				return
			}
		}
		var cleanSelected []string
		for _, s := range selected {
			if s != quitSentinel {
				cleanSelected = append(cleanSelected, s)
			}
		}
		selected = cleanSelected
		if len(selected) == 0 {
			fmt.Println(warningStyle.Render("No models selected."))
			return
		}
	}

	var sigCount int32
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)
	defer signal.Stop(sigCh)

	go func() {
		for range sigCh {
			c := atomic.AddInt32(&sigCount, 1)
			if c == 1 {
				fmt.Println(warningStyle.Render("\n⚠️  Cancelling current model... (press ctrl+c again to exit)"))
			} else {
				fmt.Println(errorStyle.Render("\n❌ Exiting."))
				os.Exit(1)
			}
		}
	}()

	profile := activeProfile()
	for _, model := range selected {
		atomic.StoreInt32(&sigCount, 0)

		fmt.Printf("\n%s %s (%s context, %s template)...\n",
			infoStyle.Render("→ Testing"),
			modelNameStyle.Render(model),
			ctxStyle.Render(ctxLabel),
			infoStyle.Render(template))

		if profile.Provider == "lmstudio" {
			fmt.Printf("  %s\n", infoStyle.Render("Skipping warmup for LM Studio; measured run will load the model after cleanup."))
		} else {
			fmt.Printf("  %s ", infoStyle.Render("Warmup run..."))
			_, _, _, _, _ = runSpeedTest(model, contextSize, "Hello")
			fmt.Println()
		}

		var epochResults []TestResult
		for i := range epochs {
			if atomic.LoadInt32(&sigCount) > 0 {
				fmt.Printf("  %s\n", warningStyle.Render("Skipped — cancelled by user"))
				break
			}
			if epochs > 1 {
				fmt.Printf("  %s %d/%d...\n", infoStyle.Render("Epoch"), i+1, epochs)
			}
			tps, promptEvalTPS, ttft, loadDuration, itl := runSpeedTest(model, contextSize, prompt)
			if tps > 0 {
				fmt.Printf("\r  %s %.2f %s (TTFT: %s, load: %s, ITL: %s)\n",
					infoStyle.Render("→"),
					tps,
					metricStyle.Render("tokens/sec"),
					formatDuration(ttft),
					formatDuration(loadDuration),
					formatDuration(itl))
				epochResults = append(epochResults, TestResult{
					Model:          model,
					TPS:            tps,
					PromptEvalTPS:  promptEvalTPS,
					TTFT:           ttft,
					LoadDuration:   loadDuration,
					ITL:            itl,
					Timestamp:      time.Now(),
					Context:        contextSize,
					ServerName:     profile.Name,
					ServerHost:     profile.Host,
					ServerProvider: profile.Provider,
				})
			} else {
				fmt.Printf("\r  %s %s\n",
					errorStyle.Render("❌"),
					errorStyle.Render("failed — skipping"))
			}
		}

		if len(epochResults) > 0 {
			var sumTPS, sumPromptTPS float64
			var sumTTFT, sumLoad time.Duration
			minTPS := epochResults[0].TPS
			maxTPS := epochResults[0].TPS
			for _, r := range epochResults {
				sumTPS += r.TPS
				sumPromptTPS += r.PromptEvalTPS
				sumTTFT += r.TTFT
				sumLoad += r.LoadDuration
				if r.TPS < minTPS {
					minTPS = r.TPS
				}
				if r.TPS > maxTPS {
					maxTPS = r.TPS
				}
			}
			avgTPS := sumTPS / float64(len(epochResults))
			avgTTFT := sumTTFT / time.Duration(len(epochResults))
			avgLoad := sumLoad / time.Duration(len(epochResults))

			var stddev float64
			if len(epochResults) > 1 {
				for _, r := range epochResults {
					diff := r.TPS - avgTPS
					stddev += diff * diff
				}
				stddev = math.Sqrt(stddev / float64(len(epochResults)-1))
			}

			stabilityStr := ""
			if len(epochResults) > 1 {
				cv := (stddev / avgTPS) * 100
				stabilityStr = fmt.Sprintf(" (stability: ±%.1f%%, min: %.2f, max: %.2f)", cv, minTPS, maxTPS)
			}

			fmt.Printf("%s %s %s %.2f %s%s (avg TTFT: %s, avg load: %s)\n",
				successStyle.Render("✅"),
				modelNameStyle.Render(model),
				infoStyle.Render("→"),
				avgTPS,
				metricStyle.Render("tokens/sec"),
				stabilityStr,
				formatDuration(avgTTFT),
				formatDuration(avgLoad))

			best := epochResults[0]
			for _, r := range epochResults {
				if r.TPS > best.TPS {
					best = r
				}
			}
			serverName := profile.Name
			if results[serverName] == nil {
				results[serverName] = make(map[string][]TestResult)
			}
			results[serverName][model] = append([]TestResult{best}, results[serverName][model]...)
			if len(results[serverName][model]) > 3 {
				results[serverName][model] = results[serverName][model][:3]
			}
		}
	}

	fmt.Println()
	fmt.Println(separatorStyle.Render(strings.Repeat("═", 70)))
	fmt.Printf("%s\n", titleStyle.Render("📊 Benchmark Summary"))
	fmt.Println(separatorStyle.Render(strings.Repeat("═", 70)))

	var testedModels []string
	var skippedModels []string
	for _, model := range selected {
		if serverData, ok := results[activeProfile().Name]; ok {
			if _, hasResults := serverData[model]; hasResults {
				testedModels = append(testedModels, model)
			} else {
				skippedModels = append(skippedModels, model)
			}
		} else {
			skippedModels = append(skippedModels, model)
		}
	}

	if len(testedModels) > 0 {
		fmt.Printf("%s %d\n", infoStyle.Render("Models tested:"), len(testedModels))
		for _, m := range testedModels {
			fmt.Printf("  %s %s\n", successStyle.Render("✓"), modelNameStyle.Render(m))
		}
	}

	if len(skippedModels) > 0 {
		fmt.Printf("%s %d\n", warningStyle.Render("Models skipped:"), len(skippedModels))
		for _, m := range skippedModels {
			fmt.Printf("  %s %s\n", warningStyle.Render("⊘"), modelNameStyle.Render(m))
		}
	}

	fmt.Println(separatorStyle.Render(strings.Repeat("─", 70)))
	fmt.Printf("%s %s\n", infoStyle.Render("Results saved to:"), getResultsPath())
	fmt.Println(separatorStyle.Render(strings.Repeat("═", 70)))

	saveResults()
}

func getPrompt(template string) string {
	switch template {
	case "code":
		return "Write a Python function to parse JSON and extract nested keys. Include error handling."
	case "chat":
		return "Explain the concept of recursion in programming. Keep it brief and clear."
	case "long":
		return "Write a detailed 800-word technical article about the evolution of large language models."
	default:
		return template
	}
}

var paramRegex = regexp.MustCompile(`(?i)\b(\d+(?:\.\d+)?)[mb]\b`)

func extractParameterSize(name string) string {
	matches := paramRegex.FindStringSubmatch(name)
	if len(matches) > 1 {
		return strings.ToUpper(matches[1] + matches[0][len(matches[0])-1:])
	}
	return ""
}

func estimateSizeFromParams(params string, quant string) int64 {
	if params == "" {
		return 0
	}
	var num float64
	_, err := fmt.Sscanf(strings.ToUpper(params), "%f", &num)
	if err != nil {
		return 0
	}

	bpw := 4.5
	q := strings.ToUpper(quant)
	switch {
	case strings.Contains(q, "Q2"):
		bpw = 2.5
	case strings.Contains(q, "Q3"):
		bpw = 3.5
	case strings.Contains(q, "Q4"):
		bpw = 4.5
	case strings.Contains(q, "Q5"):
		bpw = 5.5
	case strings.Contains(q, "Q6"):
		bpw = 6.5
	case strings.Contains(q, "Q8"):
		bpw = 8.5
	case strings.Contains(q, "F16") || strings.Contains(q, "16-BIT"):
		bpw = 16.0
	case strings.Contains(q, "F32"):
		bpw = 32.0
	}

	isMillion := strings.HasSuffix(strings.ToUpper(params), "M")
	multiplier := 1e9
	if isMillion {
		multiplier = 1e6
	}

	return int64(num * multiplier * bpw / 8.0)
}

func fetchLMStudioModels(profile ServerProfile) []Model {
	req, err := http.NewRequest(http.MethodGet, profile.Host+"/api/v1/models", nil)
	if err != nil {
		fmt.Println(errorStyle.Render("❌ Cannot create request:") + " " + err.Error())
		return nil
	}
	if profile.Token != "" {
		req.Header.Set("Authorization", "Bearer "+profile.Token)
	}

	resp, err := http.DefaultClient.Do(req)
	var bodyBytes []byte
	isV1 := false
	if err == nil {
		defer func() {
			_ = resp.Body.Close()
		}()
		if resp.StatusCode == http.StatusOK {
			bodyBytes, err = io.ReadAll(resp.Body)
			isV1 = true
		}
	}

	if !isV1 || err != nil {
		req, err = http.NewRequest(http.MethodGet, profile.Host+"/v1/models", nil)
		if err != nil {
			fmt.Println(errorStyle.Render("❌ Cannot create request:") + " " + err.Error())
			return nil
		}
		if profile.Token != "" {
			req.Header.Set("Authorization", "Bearer "+profile.Token)
		}
		resp, err = http.DefaultClient.Do(req)
		if err != nil {
			fmt.Println(errorStyle.Render("❌ Cannot connect to LM Studio:") + " " + err.Error())
			return nil
		}
		defer func() {
			_ = resp.Body.Close()
		}()
		if resp.StatusCode != http.StatusOK {
			fmt.Println(errorStyle.Render(fmt.Sprintf("❌ LM Studio returned status %d", resp.StatusCode)))
			return nil
		}
		bodyBytes, err = io.ReadAll(resp.Body)
		if err != nil {
			fmt.Println(errorStyle.Render("❌ Cannot read models response:") + " " + err.Error())
			return nil
		}
	}

	type LMStudioNativeModel struct {
		Key          string `json:"key"`
		DisplayName  string `json:"display_name"`
		Type         string `json:"type"`
		SizeBytes    int64  `json:"size_bytes"`
		ParamsString string `json:"params_string"`
		Format       string `json:"format"`
		Quantization struct {
			Name string `json:"name"`
		} `json:"quantization"`
	}

	var nativeResp struct {
		Models []LMStudioNativeModel `json:"models"`
	}

	if err := json.Unmarshal(bodyBytes, &nativeResp); err == nil && len(nativeResp.Models) > 0 {
		var models []Model
		for _, m := range nativeResp.Models {
			// Skip embeddings or other types of models if they are present
			if m.Type != "" && m.Type != "llm" {
				continue
			}

			size := m.SizeBytes
			quant := m.Quantization.Name
			params := m.ParamsString

			if size == 0 {
				params = extractParameterSize(m.Key)
				size = estimateSizeFromParams(params, quant)
			}

			models = append(models, Model{
				Name: m.Key,
				Size: size,
				Details: ModelDetails{
					ParameterSize:     params,
					QuantizationLevel: quant,
					Family:            m.Format,
				},
			})
		}
		return models
	}

	// Try the nested metadata data structure if present
	type LMStudioModel struct {
		ID       string `json:"id"`
		Object   string `json:"object"`
		Metadata struct {
			Loaded           bool   `json:"loaded"`
			SizeBytes        int64  `json:"size_bytes"`
			Quantization     string `json:"quantization"`
			MaxContextLength int    `json:"max_context_length"`
		} `json:"metadata"`
	}

	var lmResp struct {
		Data []LMStudioModel `json:"data"`
	}

	if err := json.Unmarshal(bodyBytes, &lmResp); err == nil && len(lmResp.Data) > 0 {
		var models []Model
		for _, m := range lmResp.Data {
			size := m.Metadata.SizeBytes
			quant := m.Metadata.Quantization
			var params string

			if size == 0 {
				params = extractParameterSize(m.ID)
				size = estimateSizeFromParams(params, quant)
			}

			models = append(models, Model{
				Name: m.ID,
				Size: size,
				Details: ModelDetails{
					ParameterSize:     params,
					QuantizationLevel: quant,
					Family:            "gguf",
				},
			})
		}
		return models
	}

	// Try standard OpenAI format
	var fallbackResp struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if errFallback := json.Unmarshal(bodyBytes, &fallbackResp); errFallback == nil {
		var models []Model
		for _, m := range fallbackResp.Data {
			params := extractParameterSize(m.ID)
			size := estimateSizeFromParams(params, "")
			models = append(models, Model{
				Name: m.ID,
				Size: size,
				Details: ModelDetails{
					ParameterSize: params,
					Family:        "gguf",
				},
			})
		}
		return models
	}

	fmt.Println(errorStyle.Render("❌ Cannot decode models response from LM Studio"))
	return nil
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
