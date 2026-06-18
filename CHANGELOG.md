# Changelog

All notable changes to LMSpeedTest are documented here.

## [0.4.4] - 2026-06-18

### Added
- Token count shown in each epoch result line (e.g., `42.3 tokens/sec · 312 tokens`) — gives confidence in the measurement
- Delta vs last run shown in per-model summary (`▲ +3.2 vs last` / `▼ -1.1 vs last`) — immediately see if a config change helped
- LM Studio now runs a warmup inference after loading the model, before the measured benchmark — results are now comparable to Ollama
- `prune` command — removes stored results for models no longer present on the active server

---

## [0.4.1] - 2026-06-17

### Added
- `--think` flag for `test` command — sends `"think": true` to Ollama and LM Studio to enable thinking/reasoning mode. Warmup runs always suppress thinking regardless of the flag.
- LM Studio benchmark requests now include `"think": false` by default (was missing previously)
- Context size badge (`32k`, `64k`, etc.) shown on chart bars in the web dashboard — both initial server render and live JS updates
- `compare` now does partial/fuzzy matching — `compare qwen` matches all models whose name contains "qwen"
- 1-second settling delay after warmup before the first measured epoch (allows model cache to stabilize)
- `dashboard` CLI command accepts an optional name filter argument (e.g., `dashboard qwen`)

### Fixed
- Duplicate `→ X.XX tokens/sec` lines no longer appear — stream progress line is correctly overwritten by the epoch result
- Table row font size in web dashboard now matches the rest of the UI (was inheriting browser default ~16px)
- Hover tooltips (TTFT, ITL, Prompt TPS, Tokens/sec) now use a JS fixed-position div appended to `<body>` — immune to `border-collapse: collapse` stacking context clipping that broke the CSS `::after` approach

### Changed
- Connect interactive menu options 2 (switch active) and 4 (set default) now use a TUI list picker instead of text input
- Model multi-select screen has a "Quit" option to cancel without benchmarking
- Web dashboard stat cards, Benchmark Results table container, and table header styling updated to match Generation Speed Comparison gradient-border style
- `compare` no longer requires an exact model name — partial match is now the default behavior

---

## [0.4.0] - 2026-06-10

### Added
- Multi-server support — manage and switch between multiple Ollama/LM Studio server profiles with named configurations
- Web dashboard (`serve` command) with sortable table, expandable rows, bar chart, dark/light theme, CSV/JSON export
- `doctor` command with specific exit codes for programmatic use
- `completions` command for bash, zsh, and fish shell completion scripts
- `--version` flag and `version` subcommand
- `update` command to check for new releases
- `--json` flag on `models`, `dashboard`, `info`, and `compare` for machine-readable output
- `export` command with csv, json, benchstat, and markdown formats

### Changed
- Benchmark results now track server name, host, and provider per result
- `test` command shows epoch progress inline with `\r` overwrite for clean terminal output

---

## [0.3.x] - Earlier

Initial releases with basic Ollama benchmarking, TUI model selection, multi-epoch support, and prompt templates.
