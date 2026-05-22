package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/charmbracelet/lipgloss"
)

var (
	titleStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("99")).Bold(true).MarginBottom(1)
	headerStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("86")).Bold(true)
	separatorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	modelNameStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	successStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("82"))
	errorStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	warningStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("208"))
	infoStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("39"))
	metricStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("220"))
	ctxStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("183"))
	promptStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("250"))
)

func printError(msg string, err error) {
	if err != nil {
		fmt.Println(errorStyle.Render("❌ "+msg+":") + " " + err.Error())
	} else {
		fmt.Println(errorStyle.Render("❌ " + msg))
	}
}

func printWarning(msg string, err error) {
	if err != nil {
		fmt.Println(warningStyle.Render("⚠️ "+msg+":") + " " + err.Error())
	} else {
		fmt.Println(warningStyle.Render("⚠️ " + msg))
	}
}

func printSuccess(msg string) {
	fmt.Println(successStyle.Render("✅ " + msg))
}

func getConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return configDir + "/config.json"
	}
	return filepath.Join(home, configDir, "config.json")
}

func getResultsPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return configDir + "/results.json"
	}
	return filepath.Join(home, configDir, "results.json")
}
