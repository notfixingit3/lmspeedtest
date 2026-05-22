# AGENTS.md — LMSpeedTest

## Project Overview

Single-file Go CLI (`main.go`) that benchmarks Ollama LLM inference speed (tokens/sec) with a TUI for model selection and a dashboard for viewing results.

- **Repository**: https://github.com/notfixingit3/lmspeedtest
- **Language**: Go 1.25.8
- **Binary**: `lmspeedtest` (checked in at repo root)
- **Dependencies**: `charm.land/huh/v2` (TUI forms), `github.com/charmbracelet/lipgloss` (styling)

## Commands

```bash
# Build
go build -o lmspeedtest

# Run (requires Ollama instance)
./lmspeedtest connect                          # Configure Ollama host + optional auth token
./lmspeedtest info                             # Show server version and connection info
./lmspeedtest models                           # List local models with sizes
./lmspeedtest models 8                         # List models ≤ 8 GB
./lmspeedtest models qwen                      # Filter models by name
./lmspeedtest models 8 qwen                    # Combine size + name filter
./lmspeedtest test 8                           # Interactive benchmark of models ≤ 8 GB
./lmspeedtest test 8 64k                       # Benchmark with 64k context
./lmspeedtest test 8 32k qwen                  # Filter + custom context
./lmspeedtest test 8 --all                     # Non-interactive: benchmark all matching models
./lmspeedtest test 8 --epochs 3                # Run 3 epochs per model, keep best result
./lmspeedtest test 8 --template code           # Use code generation prompt
./lmspeedtest test 8 --template chat           # Use short chat prompt
./lmspeedtest dashboard                        # Show latest benchmark results per model
./lmspeedtest export --format csv              # Export all results to CSV
./lmspeedtest export --format json             # Export all results to JSON
./lmspeedtest export --format benchstat        # Export in Go benchstat format
./lmspeedtest compare llama3.2:latest          # Compare all context sizes for a model
```

## Architecture

- **Single package `main`**, all logic in `main.go` (~870 lines)
- **No tests, no CI, no build scripts** — this is a personal/utility tool
- **State stored in `~/.lmspeedtest/`**:
  - `config.json` — Ollama host URL
  - `results.json` — last 3 benchmark results per model (capped at 3)

## Key Implementation Details

- **Ollama API**: Hits `/api/tags` (list models) and `/api/generate` (benchmark)
- **Benchmark prompt**: Configurable templates (`code`, `chat`, `long`) or custom prompt
- **Context size**: Configurable per test (default 32k, supports 64k, 128k, etc.)
- **Token counting**: Uses Ollama's `eval_count` from the final API response chunk for accurate token/sec measurement
- **Multiple epochs**: Run N epochs per model, keep best result
- **Results capped**: Only the 3 most recent runs per model are kept
- **Metrics tracked**: Generation TPS, prompt eval TPS, TTFT (time to first token), model load time
- **TUI quirk**: `huh/v2` ThemeDracula API changed; the code explicitly avoids `.WithTheme()` (see comment at line 239)

## Style & Conventions

- Standard Go formatting (`gofmt`)
- golangci-lint config in `.golangci.yml` (stricter than default)
- gosec for security scanning
- Error handling is explicit (no silent failures)
- Uses `any` (Go 1.18+) for JSON map values
- **Git commit messages must end with a random Scooby-Doo quote** (e.g., "Ruh-roh!", "Zoinks!", "Jinkies!", "Scooby-Dooby-Doo!", "Would you do it for a Scooby Snack?", "Rikes!", "Puppy Power!")

## Adding Features

Because this is a single-file utility, most changes are localized to `main.go`:
- New subcommand → add case in `main()` switch
- New API call → add helper + HTTP logic inline
- New TUI form → use `huh.NewForm(...)` pattern

No external services beyond Ollama. No test suite to maintain.
