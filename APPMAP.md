# App Map

Quick reference for where things live. Update this when adding or moving functions.

---

## Entry Point

### `main.go`
Command routing and help output.
- `main()` — arg parsing, `loadConfig()`, `loadResults()`, command switch
- `printUsage()` — help text with all commands and test options
- `printVersion()` — `--version` output

---

## Commands

### `commands_connect.go`
Everything under the `connect` subcommand — server profile management.
- `connectCmd()` — interactive menu (add, switch, set default, remove, edit)
- `connectInteractive()` — huh form flow for first-time setup
- `connectAddCmd()` — `--add <name>` flag handler
- `connectListCmd()` — `--list` flag handler
- `connectUseCmd()` — `--use <name>` / interactive switch
- `connectDefaultCmd()` — `--default <name>` / interactive default
- `connectRemoveCmd()` — `--remove <name>`
- `connectEditCurrent()` — edit host/token of active profile

### `commands_models.go`
Model listing and benchmarking (`models`, `test` commands).
- `newAPIRequest()` — shared HTTP request helper with auth header
- `fetchModels()` — calls Ollama or LM Studio, returns `[]Model`
- `fetchLMStudioModels()` — LM Studio `/v1/models` adapter
- `modelsCmd()` — list models with metadata, size filter, name filter
- `testCmd()` — full benchmark flow: arg parsing, TUI model select, epoch loop, summary
- `getPrompt()` — returns prompt string for `code`, `chat`, `long` templates
- `extractParameterSize()` / `estimateSizeFromParams()` — parse param count from model name
- `truncateString()` / `hasJSONFlag()` — small utilities used across commands

### `commands_other.go`
Smaller commands: export, info, compare, dashboard, prune, reset, doctor, completions, update.
- `exportCmd()` — CSV / JSON / benchstat / markdown export
- `infoCmd()` — server version, host, auth status
- `compareCmd()` — fuzzy compare across context sizes and servers
- `dashboardCmd()` — CLI results table with optional name filter
- `pruneCmd()` — remove results for models no longer on the active server
- `resetCmd()` — wipe all results (with confirmation)
- `doctorCmd()` — diagnostics with specific exit codes
- `completionsCmd()` — generate bash / zsh / fish completion scripts
- `updateCmd()` / `cleanVersion()` / `isUpdateAvailable()` — update checker

**Shared helpers** (used by multiple commands above and by serve):
- `resultServerLabel()` — display name for the server a result came from
- `resultServerProvider()` — `"ollama"` or `"lmstudio"` for a result
- `resultDisplayKey()` — unique `"serverName / modelName"` key for deduplication
- `providerShortName()` — short provider tag for CLI table cells
- `dashboardServerCell()` — formats the server column in the CLI dashboard table

### `commands_serve.go`
Web dashboard server (`serve` command) and its data helpers.
- `serveCmd()` — HTTP server, all route handlers, embedded HTML/CSS/JS (~1200 lines)
- `buildFlatResults()` — flattens `results` map into a sorted `[]flatResult` for the API
- `latestPerModel()` — one best result per model for the chart and stat cards
- `jsonResultsBytes()` — JSON-encodes flat results for the `/api/results` endpoint
- `providerLogoHTML()` — renders provider logo `<span>` for server-side HTML
- Logo data URIs — inline SVG constants for Ollama, LM Studio, generic logos

---

## Core Logic

### `benchmark.go`
All inference speed measurement — no CLI concerns here.
- `runSpeedTest()` — dispatches to Ollama or LM Studio path; returns `(tps, promptTPS, ttft, loadDuration, itl, tokenCount)`
- `runLMStudioSpeedTest()` — unload → load → warmup → stream benchmark for LM Studio
- `formatDuration()` — human-readable duration (`1.23s` / `450ms`)
- `unloadLMStudioModels()` / `unloadLMStudioInstance()` — LM Studio memory management
- `fetchLMStudioModelStatuses()` / `loadedLMStudioInstances()` — inspect loaded instances
- `responseBodySnippet()` / `httpErrorDetails()` / `isLMStudioFatalLoadError()` — HTTP error helpers

### `config.go`
Config and results persistence.
- `loadConfig()` / `saveConfig()` — read/write `~/.lmspeedtest/config.json`
- `loadResults()` / `saveResults()` — read/write `~/.lmspeedtest/results.json`
- `activeProfile()` — returns the currently active `ServerProfile`
- `defaultServerProfile()` — fallback profile if none configured
- `normalizeConfig()` — migration / repair of config schema across versions
- `profileByName()` — look up a profile by name
- `profileNameForHost()` / `uniqueProfileName()` — helpers for multi-server setup
- `inferProviderFromServerName()` — guess Ollama vs LM Studio from host/name

---

## Data & Styles

### `types.go`
All shared types and version variables.
- `version`, `commit`, `date` — injected by GoReleaser ldflags; fallback values for dev
- `ServerProfile`, `Config` — connection config structs
- `Model`, `ModelDetails`, `ModelsResponse` — Ollama/LM Studio model list types
- `TestResult` — single benchmark result (model, TPS, TTFT, ITL, context, server info, timestamp)
- `providerDisplayName()` — `"ollama"` → `"Ollama"`, `"lmstudio"` → `"LM Studio"`

### `styles.go`
All terminal styling via Lipgloss. Import and use styles; don't define new ones elsewhere.
- `titleStyle`, `headerStyle`, `separatorStyle` — section headings
- `infoStyle`, `successStyle`, `warningStyle`, `errorStyle` — status indicators
- `modelNameStyle`, `metricStyle`, `ctxStyle`, `promptStyle` — data formatting

---

## Tests

| File | Covers |
|------|--------|
| `benchmark_test.go` | `formatDuration`, HTTP mocking for Ollama/LM Studio streams |
| `commands_models_test.go` | `fetchModels`, `getPrompt`, model size estimation |
| `commands_other_test.go` | `compareCmd`, `dashboardCmd`, export formats, result helpers |
| `config_test.go` | Config load/save, migration, profile helpers |
| `types_test.go` | `providerDisplayName`, struct defaults |

---

## Release & CI

| File | Purpose |
|------|---------|
| `.goreleaser.yml` | Cross-platform builds (linux/darwin/windows × amd64/arm64), ldflags version injection |
| `.github/workflows/release.yml` | Triggers GoReleaser on `v*` tag push |
| `.github/workflows/security.yml` | `gosec` scan on push/PR to main |
| `.github/FUNDING.yml` | GitHub Sponsors + Buy Me a Coffee links |
