# LMSpeedTest

<p align="center">
  <img src="logo.png" alt="LMSpeedTest Logo" width="400">
</p>

<p align="center">
  <img src="https://img.shields.io/badge/go-1.25.8-blue?style=for-the-badge&logo=go" alt="Go Version">
  <img src="https://img.shields.io/badge/license-MIT-green?style=for-the-badge" alt="License">
  <a href="https://www.buymeacoffee.com/notfixingit">
    <img src="https://img.shields.io/badge/Buy%20Me%20a%20Coffee-ffdd00?style=for-the-badge&logo=buy-me-a-coffee&logoColor=black" alt="Buy Me A Coffee">
  </a>
</p>

<p align="center">
  <b>A beautiful, interactive CLI for benchmarking Ollama & LM Studio LLM inference speed</b>
</p>

---

## ✨ Features

- 🎯 **Interactive Model Selection** — Beautiful TUI multi-select with filtering by size and name
- 📊 **Rich Metrics** — Generation TPS, prompt eval TPS, TTFT, ITL, and model load time
- 🎨 **Styled Terminal Output** — Powered by [Lipgloss](https://github.com/charmbracelet/lipgloss) and [Huh](https://github.com/charmbracelet/huh)
- 🔁 **Multiple Epochs** — Run N benchmarks per model, keep the best result
- 📝 **Configurable Prompts** — Code generation, chat, long-form, or custom file-based prompts
- 📈 **Export Formats** — CSV, JSON, or Go benchstat-compatible output
- 🔥 **Batch Mode** — Benchmark all matching models without interactive selection
- 🌡️ **Automatic Warmup** — Every model gets a lightweight warmup run before measurement
- 📉 **Stability Analysis** — See variance, min/max across epochs
- 🌐 **Web Dashboard** — Serve results over HTTP with `lmspeedtest serve`
- 🔐 **Auth Support** — Bearer token authentication for remote Ollama/LM Studio instances
- 🖥️ **Remote-Friendly** — Works with local or remote Ollama/LM Studio servers
- 🏢 **Multi-Server Support** — Manage and benchmark multiple Ollama/LM Studio servers

---

## 🚀 Installation

### Pre-built Binaries (Recommended)

Download the latest release for your platform from the [releases page](https://github.com/notfixingit3/lmspeedtest/releases).

**macOS (Intel)**
```bash
curl -L https://github.com/notfixingit3/lmspeedtest/releases/latest/download/lmspeedtest_$(curl -s https://api.github.com/repos/notfixingit3/lmspeedtest/releases/latest | grep tag_name | cut -d '"' -f 4)_darwin_amd64.tar.gz | tar xz
mv lmspeedtest /usr/local/bin/
```

**macOS (Apple Silicon)**
```bash
curl -L https://github.com/notfixingit3/lmspeedtest/releases/latest/download/lmspeedtest_$(curl -s https://api.github.com/repos/notfixingit3/lmspeedtest/releases/latest | grep tag_name | cut -d '"' -f 4)_darwin_arm64.tar.gz | tar xz
mv lmspeedtest /usr/local/bin/
```

**Linux (x86_64)**
```bash
curl -L https://github.com/notfixingit3/lmspeedtest/releases/latest/download/lmspeedtest_$(curl -s https://api.github.com/repos/notfixingit3/lmspeedtest/releases/latest | grep tag_name | cut -d '"' -f 4)_linux_amd64.tar.gz | tar xz
sudo mv lmspeedtest /usr/local/bin/
```

**Linux (ARM64)**
```bash
curl -L https://github.com/notfixingit3/lmspeedtest/releases/latest/download/lmspeedtest_$(curl -s https://api.github.com/repos/notfixingit3/lmspeedtest/releases/latest | grep tag_name | cut -d '"' -f 4)_linux_arm64.tar.gz | tar xz
sudo mv lmspeedtest /usr/local/bin/
```

**Windows (x86_64)**
Download `lmspeedtest_*_windows_amd64.zip` from the [releases page](https://github.com/notfixingit3/lmspeedtest/releases), extract, and add to your PATH.

**Windows (ARM64 - Snapdragon/Qualcomm)**
Download `lmspeedtest_*_windows_arm64.zip` from the [releases page](https://github.com/notfixingit3/lmspeedtest/releases), extract, and add to your PATH.

### Via Go Install

```bash
go install github.com/notfixingit3/lmspeedtest@latest
```

### From Source

```bash
git clone https://github.com/notfixingit3/lmspeedtest.git
cd lmspeedtest
go build -o lmspeedtest
chmod +x lmspeedtest
```

---

## 🎬 Quick Start

```bash
# Configure your Ollama/LM Studio host
./lmspeedtest connect

# List available models
./lmspeedtest models

# Benchmark models ≤ 8 GB with interactive selection
./lmspeedtest test 8

# View your results dashboard
./lmspeedtest dashboard
```

---

## 📖 Usage

### Commands

| Command | Description |
|---------|-------------|
| `connect` | Configure Ollama/LM Studio host and optional auth token |
| `connect --add <name>` | Add a new server profile |
| `connect --list` | List all configured server profiles |
| `connect --default <name>` | Set the default server profile |
| `connect --use <name>` | Switch to a different server profile |
| `connect --remove <name>` | Remove a server profile |
| `info` | Show server version, host, and auth status |
| `models [max_gb] [name_filter]` | List models with metadata (params, quantization) |
| `test <max_gb> [opts]` | Benchmark matching models |
| `dashboard` | Show latest results per model |
| `compare <model>` | Compare all context sizes for a model |
| `export [--format fmt]` | Export results (csv, json, benchstat) |
| `reset` | Clear all benchmark results |
| `serve [--port N]` | Start web dashboard (default: 8080) |

### Test Options

```bash
./lmspeedtest test 8                    # Interactive TUI selection
./lmspeedtest test 8 64k                # Use 64k context window
./lmspeedtest test 8 32k qwen           # Filter by name + context
./lmspeedtest test 8 llama,gemma4       # Multi-filter: match any name
./lmspeedtest test 8 --all              # Benchmark all (skip TUI)
./lmspeedtest test 8 --epochs 3         # Run 3 epochs, keep best
./lmspeedtest test 8 --template code    # Code generation prompt
./lmspeedtest test 8 --template chat    # Short chat prompt
./lmspeedtest test 8 --template long    # Long-form writing (default)
./lmspeedtest test 8 --prompt-file path.txt  # Custom prompt from file
```

### Real-World Examples

```bash
# Compare all your models at once
./lmspeedtest test 100 --all

# Deep benchmark: 5 epochs with long-form prompt
./lmspeedtest test 16 --epochs 5 --template long

# Quick code generation benchmark
./lmspeedtest test 8 --template code --epochs 3

# Deep benchmark with custom prompt
./lmspeedtest test 16 --epochs 5 --prompt-file prompt.txt

# Multi-filter: benchmark llama and gemma models only
./lmspeedtest test 16 llama,gemma --epochs 3

# Export for statistical analysis
./lmspeedtest export --format benchstat > results.bench
benchstat results.bench

# Start web dashboard
./lmspeedtest serve
./lmspeedtest serve --port 3000

# Clear all results
./lmspeedtest reset

# Connect to remote Ollama/LM Studio with auth
./lmspeedtest connect
# Host: https://ollama.example.com or http://10.1.6.30:1234
# Token: sk-abc123
./lmspeedtest info

# Multi-server: add and switch between servers
./lmspeedtest connect --add desktop --host http://192.168.1.10:11434
./lmspeedtest connect --add laptop --host http://192.168.1.11:11434 --token sk-abc
./lmspeedtest connect --list
# → default (http://localhost:11434) [active]
# → desktop (http://192.168.1.10:11434)
# → laptop (http://192.168.1.11:11434)
./lmspeedtest connect --use desktop
./lmspeedtest test 8 --all
./lmspeedtest connect --use laptop
./lmspeedtest test 8 --all
./lmspeedtest compare llama3.2:latest
# Shows results from all servers with server column
```

---

## 📊 Metrics Explained

| Metric | Description |
|--------|-------------|
| **Tokens/sec** | Generation speed — output tokens per second |
| **Prompt TPS** | Input processing speed — prompt eval tokens per second |
| **TTFT** | Time to first token — load + prompt eval duration |
| **ITL** | Inter-token latency — time between consecutive tokens |
| **Load Time** | Time to load model weights into GPU/CPU memory |
| **Stability** | Variance across epochs (stddev, min, max) |

---

## ⚙️ Configuration

State is stored in `~/.lmspeedtest/`:

- **`config.json`** — Server profiles (host URL, auth token, active profile)
- **`results.json`** — Last 3 benchmark results per model per server (capped at 3)

### Multi-Server Setup

Manage multiple Ollama/LM Studio servers with named profiles:

```bash
# Add servers
./lmspeedtest connect --add desktop --host http://192.168.1.10:11434
./lmspeedtest connect --add laptop --host http://192.168.1.11:11434 --token sk-abc

# List profiles
./lmspeedtest connect --list

# Switch active server
./lmspeedtest connect --use desktop

# Remove a profile
./lmspeedtest connect --remove laptop
```

Results are stored per-server, so you can benchmark the same model on different hardware and compare.

---

## 📋 Requirements

- Go 1.25.8+
- Ollama or LM Studio instance (local or remote)

---

## 🤝 Contributing

Contributions are welcome! The codebase is organized into logical files:

- `main.go` — Entry point and command routing
- `types.go` — Structs and data models
- `config.go` — Configuration persistence and migration
- `styles.go` — Terminal styling helpers
- `commands_connect.go` — Server profile management
- `commands_models.go` — Model listing and benchmarking
- `benchmark.go` — Core benchmark logic
- `commands_other.go` — Dashboard, compare, export, serve, reset

- New subcommand → add case in `main()` switch
- New API call → add helper in the relevant command file
- New TUI form → use `huh.NewForm(...)` pattern

Please ensure your code passes `golangci-lint` and `gosec`.

---

## ☕ Support

If you find this tool useful, consider buying me a coffee:

<a href="https://www.buymeacoffee.com/notfixingit">
  <img src="https://img.shields.io/badge/Buy%20Me%20a%20Coffee-ffdd00?style=for-the-badge&logo=buy-me-a-coffee&logoColor=black" alt="Buy Me A Coffee">
</a>

---

## 📄 License

MIT License — see [LICENSE](LICENSE) for details.

---

<p align="center">
  Made with a fucked-up back and extremely questionable sleep habits by <a href="https://github.com/notfixingit3">@notfixingit3</a>
</p>
