package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const maxErrorBodyBytes = 4096

func responseBodySnippet(body io.Reader) string {
	bodyBytes, err := io.ReadAll(io.LimitReader(body, maxErrorBodyBytes+1))
	if err != nil {
		return "could not read response body: " + err.Error()
	}
	bodyText := strings.TrimSpace(string(bodyBytes))
	if bodyText == "" {
		return "empty response body"
	}
	if len(bodyBytes) > maxErrorBodyBytes {
		bodyText = bodyText[:maxErrorBodyBytes] + "... (truncated)"
	}
	return bodyText
}

func httpErrorDetails(resp *http.Response) string {
	return fmt.Sprintf("%s %s: %s", resp.Request.Method, resp.Request.URL.Path, responseBodySnippet(resp.Body))
}

func isLMStudioFatalLoadError(details string) bool {
	lowerDetails := strings.ToLower(details)
	return strings.Contains(lowerDetails, "model_load_failed") ||
		strings.Contains(lowerDetails, "insufficient system resources") ||
		strings.Contains(lowerDetails, "failed to load model")
}

func runSpeedTest(model string, contextSize int, prompt string, think bool) (float64, float64, time.Duration, time.Duration, time.Duration) {
	profile := activeProfile()
	if profile.Provider == "lmstudio" {
		return runLMStudioSpeedTest(model, contextSize, prompt, profile, think)
	}

	reqBody := map[string]any{
		"model":  model,
		"prompt": prompt,
		"options": map[string]any{
			"num_ctx":     contextSize,
			"temperature": 0.0,
		},
		"think":  think,
		"stream": true,
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		fmt.Println(errorStyle.Render("  Cannot marshal request:") + " " + err.Error())
		return 0, 0, 0, 0, 0
	}
	req, err := newAPIRequest("POST", profile.Host+"/api/generate", strings.NewReader(string(data)))
	if err != nil {
		fmt.Println(errorStyle.Render("  Cannot create request:"))
		return 0, 0, 0, 0, 0
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Println(errorStyle.Render(fmt.Sprintf("  Connection error to %s", providerDisplayName(profile.Provider))))
		return 0, 0, 0, 0, 0
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			fmt.Println(warningStyle.Render("⚠️ Cannot close response body:") + " " + err.Error())
		}
	}()

	start := time.Now()
	var evalCount int
	var promptEvalCount int
	var promptEvalDurationNanos int64
	var loadDurationNanos int64
	reader := bufio.NewReader(resp.Body)
	tokenCount := 0
	lastUpdate := time.Now()
	var itlSum time.Duration
	var itlCount int
	var lastTokenTime time.Time

	for {
		line, err := reader.ReadBytes('\n')
		if err != nil && err != io.EOF {
			break
		}
		if len(line) == 0 {
			continue
		}

		var chunk struct {
			Response           string `json:"response"`
			Done               bool   `json:"done"`
			EvalCount          int    `json:"eval_count,omitempty"`
			PromptEvalCount    int    `json:"prompt_eval_count,omitempty"`
			PromptEvalDuration int64  `json:"prompt_eval_duration,omitempty"`
			LoadDuration       int64  `json:"load_duration,omitempty"`
			TotalDuration      int64  `json:"total_duration,omitempty"`
			EvalDuration       int64  `json:"eval_duration,omitempty"`
		}
		if err := json.Unmarshal(line, &chunk); err != nil {
			continue
		}

		if chunk.Response != "" {
			tokenCount += len(strings.Fields(chunk.Response)) + 1
			now := time.Now()
			if !lastTokenTime.IsZero() {
				itlSum += now.Sub(lastTokenTime)
				itlCount++
			}
			lastTokenTime = now
		}

		if time.Since(lastUpdate) > 500*time.Millisecond {
			elapsed := time.Since(start).Seconds()
			if elapsed > 0 {
				currentTPS := float64(tokenCount) / elapsed
				fmt.Printf("\r  %s %.2f %s (%d tokens, %.1fs)",
					infoStyle.Render("→"),
					currentTPS,
					metricStyle.Render("tokens/sec"),
					tokenCount,
					elapsed)
			}
			lastUpdate = time.Now()
		}

		if chunk.Done {
			evalCount = chunk.EvalCount
			promptEvalCount = chunk.PromptEvalCount
			promptEvalDurationNanos = chunk.PromptEvalDuration
			loadDurationNanos = chunk.LoadDuration
			break
		}
	}

	seconds := time.Since(start).Seconds()
	if seconds == 0 || evalCount == 0 {
		return 0, 0, 0, 0, 0
	}
	tpsOut := float64(evalCount) / seconds

	var promptEvalTPS float64
	if promptEvalDurationNanos > 0 {
		promptEvalTPS = float64(promptEvalCount) / (float64(promptEvalDurationNanos) / 1e9)
	}

	var ttft time.Duration
	if loadDurationNanos > 0 || promptEvalDurationNanos > 0 {
		ttft = time.Duration(loadDurationNanos + promptEvalDurationNanos)
	}

	var itl time.Duration
	if itlCount > 0 {
		itl = itlSum / time.Duration(itlCount)
	}

	return tpsOut, promptEvalTPS, ttft, time.Duration(loadDurationNanos), itl
}

func formatDuration(d time.Duration) string {
	if d == 0 {
		return "-"
	}
	if d >= time.Second {
		return fmt.Sprintf("%.2fs", d.Seconds())
	}
	return fmt.Sprintf("%.0fms", float64(d/time.Millisecond))
}

type lmStudioLoadedInstance struct {
	ID string `json:"id"`
}

type lmStudioModelStatus struct {
	Key             string                   `json:"key"`
	LoadedInstances []lmStudioLoadedInstance `json:"loaded_instances"`
}

func fetchLMStudioModelStatuses(profile ServerProfile) ([]lmStudioModelStatus, error) {
	req, err := http.NewRequest(http.MethodGet, profile.Host+"/api/v1/models", nil)
	if err != nil {
		return nil, err
	}
	if profile.Token != "" {
		req.Header.Set("Authorization", "Bearer "+profile.Token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("LM Studio model status returned %d: %s", resp.StatusCode, responseBodySnippet(resp.Body))
	}
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var lmResp struct {
		Models []lmStudioModelStatus `json:"models"`
	}

	if err := json.Unmarshal(bodyBytes, &lmResp); err != nil {
		return nil, err
	}
	return lmResp.Models, nil
}

func loadedLMStudioInstances(profile ServerProfile) ([]lmStudioLoadedInstance, error) {
	statuses, err := fetchLMStudioModelStatuses(profile)
	if err != nil {
		return nil, err
	}
	var instances []lmStudioLoadedInstance
	for _, m := range statuses {
		instances = append(instances, m.LoadedInstances...)
	}
	return instances, nil
}

func unloadLMStudioInstance(profile ServerProfile, instanceID string) bool {
	unloadBody := map[string]string{
		"instance_id": instanceID,
	}
	unloadData, err := json.Marshal(unloadBody)
	if err != nil {
		return false
	}
	unloadReq, err := http.NewRequest(http.MethodPost, profile.Host+"/api/v1/models/unload", strings.NewReader(string(unloadData)))
	if err != nil {
		return false
	}
	if profile.Token != "" {
		unloadReq.Header.Set("Authorization", "Bearer "+profile.Token)
	}
	unloadReq.Header.Set("Content-Type", "application/json")
	unloadResp, err := http.DefaultClient.Do(unloadReq)
	if err != nil {
		fmt.Printf("  %s Could not unload LM Studio instance %s: %s\n", warningStyle.Render("⚠️"), instanceID, err)
		return false
	}
	defer func() {
		_ = unloadResp.Body.Close()
	}()
	if unloadResp.StatusCode != http.StatusOK {
		fmt.Printf("  %s Unload returned status %d for %s: %s\n",
			warningStyle.Render("⚠️"),
			unloadResp.StatusCode,
			instanceID,
			responseBodySnippet(unloadResp.Body))
		return false
	}
	return true
}

func unloadLMStudioModels(profile ServerProfile) bool {
	deadline := time.Now().Add(60 * time.Second)
	totalUnloaded := 0

	for {
		instances, err := loadedLMStudioInstances(profile)
		if err != nil {
			fmt.Printf("  %s Could not list LM Studio loaded models: %s\n", warningStyle.Render("⚠️"), err)
			return false
		}
		if len(instances) == 0 {
			if totalUnloaded > 0 {
				fmt.Printf("  %s LM Studio model memory is clear.\n", infoStyle.Render("→"))
			}
			return true
		}
		if time.Now().After(deadline) {
			fmt.Printf("  %s Timed out waiting for LM Studio to unload %d model instance(s)\n", warningStyle.Render("⚠️"), len(instances))
			return false
		}

		unloadedThisPass := 0
		for _, inst := range instances {
			if inst.ID == "" {
				continue
			}
			if unloadLMStudioInstance(profile, inst.ID) {
				unloadedThisPass++
				totalUnloaded++
			}
		}
		if unloadedThisPass > 0 {
			fmt.Printf("  %s Unloaded %d LM Studio model instance(s), waiting for memory release...\n", infoStyle.Render("→"), unloadedThisPass)
		}
		time.Sleep(750 * time.Millisecond)
	}
}

func runLMStudioSpeedTest(model string, contextSize int, prompt string, profile ServerProfile, think bool) (float64, float64, time.Duration, time.Duration, time.Duration) {
	// 0. Unload existing models to free memory
	if !unloadLMStudioModels(profile) {
		fmt.Printf("  %s LM Studio still has loaded models; skipping %s to avoid memory pressure.\n",
			warningStyle.Render("→"),
			modelNameStyle.Render(model))
		return 0, 0, 0, 0, 0
	}

	// 1. Load Model
	loadStart := time.Now()
	var loadDuration time.Duration

	loadReqBody := map[string]any{
		"model":          model,
		"context_length": contextSize,
	}
	loadData, err := json.Marshal(loadReqBody)
	if err == nil {
		loadReq, err := http.NewRequest(http.MethodPost, profile.Host+"/api/v1/models/load", strings.NewReader(string(loadData)))
		if err == nil {
			if profile.Token != "" {
				loadReq.Header.Set("Authorization", "Bearer "+profile.Token)
			}
			loadReq.Header.Set("Content-Type", "application/json")
			fmt.Printf("  %s Loading %s in LM Studio with %dk context...\n",
				infoStyle.Render("→"),
				modelNameStyle.Render(model),
				contextSize/1024)
			loadResp, err := http.DefaultClient.Do(loadReq)
			if err == nil {
				defer func() {
					_ = loadResp.Body.Close()
				}()
				if loadResp.StatusCode == http.StatusOK {
					loadDuration = time.Since(loadStart)
					fmt.Printf("  %s LM Studio loaded model in %s. Starting benchmark stream...\n",
						infoStyle.Render("→"),
						formatDuration(loadDuration))
				} else {
					details := httpErrorDetails(loadResp)
					fmt.Printf("\n%s Load returned status %d for %s at %dk context.\n",
						warningStyle.Render("⚠️"),
						loadResp.StatusCode,
						modelNameStyle.Render(model),
						contextSize/1024)
					fmt.Printf("  %s %s\n", warningStyle.Render("Response:"), details)
					if isLMStudioFatalLoadError(details) {
						fmt.Printf("  %s LM Studio reported a model load failure; skipping chat request.\n", warningStyle.Render("→"))
						return 0, 0, 0, 0, 0
					}
					fmt.Printf("  %s Proceeding with chat request anyway...\n", warningStyle.Render("→"))
				}
			} else {
				fmt.Printf("\n%s Failed to contact load endpoint for %s: %s. Proceeding anyway...\n",
					warningStyle.Render("⚠️"),
					modelNameStyle.Render(model),
					err)
				fmt.Printf("  %s Starting benchmark stream via chat endpoint...\n", infoStyle.Render("→"))
			}
		}
	}

	// 2. Chat Completions Stream
	chatReqBody := map[string]any{
		"model": model,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
		"temperature": 0.0,
		"think":       think,
		"stream":      true,
		"stream_options": map[string]any{
			"include_usage": true,
		},
	}

	chatData, err := json.Marshal(chatReqBody)
	if err != nil {
		fmt.Println(errorStyle.Render("  Cannot marshal chat request:") + " " + err.Error())
		return 0, 0, 0, 0, 0
	}
	req, err := http.NewRequest(http.MethodPost, profile.Host+"/v1/chat/completions", strings.NewReader(string(chatData)))
	if err != nil {
		fmt.Println(errorStyle.Render("  Cannot create request:"))
		return 0, 0, 0, 0, 0
	}
	if profile.Token != "" {
		req.Header.Set("Authorization", "Bearer "+profile.Token)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Println(errorStyle.Render("  Connection error to LM Studio"))
		return 0, 0, 0, 0, 0
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		fmt.Printf("\n%s LM Studio returned status %d for %s at %dk context\n",
			errorStyle.Render("❌"),
			resp.StatusCode,
			modelNameStyle.Render(model),
			contextSize/1024)
		fmt.Printf("  %s %s\n", errorStyle.Render("Response:"), httpErrorDetails(resp))
		return 0, 0, 0, 0, 0
	}

	start := time.Now()
	var evalCount int
	var promptEvalCount int
	var promptEvalDurationNanos int64
	var ttft time.Duration
	var lastTokenTime time.Time
	var itlSum time.Duration
	var itlCount int
	tokenCount := 0
	lastUpdate := time.Now()

	reader := bufio.NewReader(resp.Body)

	for {
		line, err := reader.ReadBytes('\n')
		if err != nil && err != io.EOF {
			break
		}
		lineStr := strings.TrimSpace(string(line))
		if lineStr == "" {
			if err == io.EOF {
				break
			}
			continue
		}

		if !strings.HasPrefix(lineStr, "data: ") {
			if err == io.EOF {
				break
			}
			continue
		}

		dataStr := strings.TrimPrefix(lineStr, "data: ")
		if dataStr == "[DONE]" {
			break
		}

		type ChatCompletionChunk struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
				FinishReason string `json:"finish_reason"`
			} `json:"choices"`
			Usage *struct {
				PromptTokens     int `json:"prompt_tokens"`
				CompletionTokens int `json:"completion_tokens"`
				TotalTokens      int `json:"total_tokens"`
			} `json:"usage,omitempty"`
		}

		var chunk ChatCompletionChunk
		if err := json.Unmarshal([]byte(dataStr), &chunk); err != nil {
			continue
		}

		hasContent := false
		if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Content != "" {
			hasContent = true
			if tokenCount == 0 {
				ttft = time.Since(start)
				promptEvalDurationNanos = ttft.Nanoseconds()
			}
			tokenCount++
			now := time.Now()
			if !lastTokenTime.IsZero() {
				itlSum += now.Sub(lastTokenTime)
				itlCount++
			}
			lastTokenTime = now
		}

		if chunk.Usage != nil {
			evalCount = chunk.Usage.CompletionTokens
			promptEvalCount = chunk.Usage.PromptTokens
		}

		if hasContent && time.Since(lastUpdate) > 500*time.Millisecond {
			elapsed := time.Since(start).Seconds()
			if elapsed > 0 {
				currentTPS := float64(tokenCount) / elapsed
				fmt.Printf("\r  %s %.2f %s (%d tokens, %.1fs)",
					infoStyle.Render("→"),
					currentTPS,
					metricStyle.Render("tokens/sec"),
					tokenCount,
					elapsed)
			}
			lastUpdate = time.Now()
		}

		if err == io.EOF {
			break
		}
	}

	seconds := time.Since(start).Seconds()
	if seconds == 0 {
		return 0, 0, 0, 0, 0
	}

	if evalCount == 0 {
		evalCount = tokenCount
	}
	if promptEvalCount == 0 {
		promptEvalCount = len(strings.Fields(prompt))
	}

	tpsOut := float64(evalCount) / seconds

	var promptEvalTPS float64
	if promptEvalDurationNanos > 0 {
		promptEvalTPS = float64(promptEvalCount) / (float64(promptEvalDurationNanos) / 1e9)
	}

	var itl time.Duration
	if itlCount > 0 {
		itl = itlSum / time.Duration(itlCount)
	}

	return tpsOut, promptEvalTPS, ttft, loadDuration, itl
}
