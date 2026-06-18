package main

import (
	"fmt"
	"os"
	"runtime"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		return
	}

	if os.Args[1] == "--version" || os.Args[1] == "version" {
		printVersion()
		return
	}

	loadConfig()
	loadResults()

	switch os.Args[1] {
	case "connect":
		connectCmd()
	case "models":
		modelsCmd()
	case "test":
		testCmd()
	case "dashboard":
		dashboardCmd()
	case "export":
		exportCmd()
	case "compare":
		compareCmd()
	case "info":
		infoCmd()
	case "prune":
		pruneCmd()
	case "reset":
		resetCmd()
	case "serve":
		serveCmd()
	case "doctor":
		doctorCmd()
	case "completions":
		completionsCmd()
	case "update":
		updateCmd()
	default:
		printUsage()
	}
}

func printVersion() {
	fmt.Printf("LMSpeedTest %s\n", version)
	fmt.Printf("  commit: %s\n", commit)
	fmt.Printf("  built:  %s\n", date)
	fmt.Printf("  go:     %s\n", runtime.Version())
	fmt.Printf("  os:     %s/%s\n", runtime.GOOS, runtime.GOARCH)
}

func printUsage() {
	fmt.Println()
	fmt.Println(titleStyle.Render(fmt.Sprintf("🚀 LMSpeedTest v%s", version)))
	fmt.Println()
	fmt.Println(headerStyle.Render("COMMANDS"))
	fmt.Println()

	commands := []struct {
		cmd  string
		desc string
	}{
		{"connect", "Configure Ollama/LM Studio host and optional auth token"},
		{"connect --default <name>", "Set the default server profile"},
		{"info", "Show server version, host, and auth status"},
		{"models [max_gb] [name_filter]", "List local models. Filter by size (GB) and/or name"},
		{"test <max_gb> [ctx] [filter] [opts]", "Benchmark models matching criteria"},
		{"dashboard [name_filter]", "Show latest benchmark results, optionally filtered by name"},
		{"compare <model_name>", "Compare all context sizes for a model"},
		{"export [--format fmt]", "Export results: csv, json, benchstat, or markdown"},
		{"doctor", "Run diagnostics: check config, connectivity, and permissions"},
		{"completions [shell]", "Generate shell completion scripts (bash, zsh, fish)"},
		{"update", "Check for updates"},
		{"prune", "Remove results for models no longer on the active server"},
		{"reset", "Clear all benchmark results"},
		{"serve [--port N]", "Start web dashboard (default: 8080)"},
	}

	for _, c := range commands {
		fmt.Printf("  %s  %s\n",
			infoStyle.Render(fmt.Sprintf("%-36s", c.cmd)),
			c.desc)
	}

	fmt.Println()
	fmt.Println(headerStyle.Render("TEST OPTIONS"))
	fmt.Println()
	fmt.Printf("  %s  %s\n", infoStyle.Render("--all"), "Benchmark all matching models (skip TUI)")
	fmt.Printf("  %s  %s\n", infoStyle.Render("--epochs N"), "Run N epochs per model, keep best result")
	fmt.Printf("  %s  %s\n", infoStyle.Render("--template T"), "Prompt template: code, chat, long (default)")
	fmt.Printf("  %s  %s\n", infoStyle.Render("--prompt-file path"), "Use custom prompt from file")
	fmt.Printf("  %s  %s\n", infoStyle.Render("--think"), "Enable thinking/reasoning mode (opt-in)")
	fmt.Println()
	fmt.Println(headerStyle.Render("EXAMPLES"))
	fmt.Println()
	fmt.Printf("  %s\n", promptStyle.Render("lmspeedtest test 8"))
	fmt.Printf("  %s\n", promptStyle.Render("lmspeedtest test 8 64k"))
	fmt.Printf("  %s\n", promptStyle.Render("lmspeedtest test 8 32k qwen --epochs 3"))
	fmt.Printf("  %s\n", promptStyle.Render("lmspeedtest test 8 --all --template code"))
	fmt.Printf("  %s\n", promptStyle.Render("lmspeedtest test 8 --epochs 3"))
	fmt.Printf("  %s\n", promptStyle.Render("lmspeedtest test 8 --prompt-file prompt.txt"))
	fmt.Printf("  %s\n", promptStyle.Render("lmspeedtest export --format benchstat"))
	fmt.Printf("  %s\n", promptStyle.Render("lmspeedtest serve"))
	fmt.Println()
}
