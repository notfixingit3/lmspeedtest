package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	huh "charm.land/huh/v2"
)

func connectCmd() {
	if len(os.Args) > 2 {
		switch os.Args[2] {
		case "--add":
			connectAddCmd()
			return
		case "--list":
			connectListCmd()
			return
		case "--use":
			if len(os.Args) > 3 {
				connectUseCmd(os.Args[3])
			} else {
				fmt.Println(errorStyle.Render("Usage: lmspeedtest connect --use <profile-name>"))
			}
			return
		case "--default":
			if len(os.Args) > 3 {
				connectDefaultCmd(os.Args[3])
			} else {
				fmt.Println(errorStyle.Render("Usage: lmspeedtest connect --default <profile-name>"))
			}
			return
		case "--remove":
			if len(os.Args) > 3 {
				connectRemoveCmd(os.Args[3])
			} else {
				fmt.Println(errorStyle.Render("Usage: lmspeedtest connect --remove <profile-name>"))
			}
			return
		}
	}

	connectInteractive()
}

func connectInteractive() {
	fmt.Printf("\n%s\n", titleStyle.Render("🔌 Server Profiles"))
	fmt.Println(separatorStyle.Render(strings.Repeat("─", 60)))

	if len(config.Profiles) == 0 {
		profile := defaultServerProfile()
		config.Profiles = []ServerProfile{profile}
		config.ActiveProfile = profile.Name
		config.DefaultProfile = profile.Name
	}

	fmt.Println(infoStyle.Render("Current profiles:"))
	for _, p := range config.Profiles {
		marker := "  "
		if p.Name == config.ActiveProfile {
			marker = successStyle.Render("→ ")
		}
		defaultMarker := ""
		if p.Name == config.DefaultProfile {
			defaultMarker = infoStyle.Render(" [default]")
		}
		fmt.Printf("%s%s (%s - %s)%s\n", marker, modelNameStyle.Render(p.Name), providerDisplayName(p.Provider), p.Host, defaultMarker)
	}

	fmt.Println()
	fmt.Println("Options:")
	fmt.Println("  1. Add new profile")
	fmt.Println("  2. Switch active profile")
	fmt.Println("  3. Edit current profile")
	fmt.Println("  4. Set default profile")
	fmt.Println("  5. Done")
	fmt.Print(promptStyle.Render("Choice: "))

	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	choice := strings.TrimSpace(scanner.Text())

	switch choice {
	case "1":
		connectAddCmd()
	case "2":
		var options []huh.Option[string]
		for _, p := range config.Profiles {
			label := fmt.Sprintf("%s (%s - %s)", p.Name, providerDisplayName(p.Provider), p.Host)
			if p.Name == config.ActiveProfile {
				label += " [active]"
			}
			options = append(options, huh.NewOption(label, p.Name))
		}
		var selected string
		form := huh.NewForm(huh.NewGroup(
			huh.NewSelect[string]().
				Title("Switch active profile").
				Options(options...).
				Value(&selected),
		))
		if err := form.Run(); err == nil && selected != "" {
			connectUseCmd(selected)
		}
	case "3":
		connectEditCurrent()
	case "4":
		var options []huh.Option[string]
		for _, p := range config.Profiles {
			label := fmt.Sprintf("%s (%s - %s)", p.Name, providerDisplayName(p.Provider), p.Host)
			if p.Name == config.DefaultProfile {
				label += " [default]"
			}
			options = append(options, huh.NewOption(label, p.Name))
		}
		var selected string
		form := huh.NewForm(huh.NewGroup(
			huh.NewSelect[string]().
				Title("Set default profile").
				Options(options...).
				Value(&selected),
		))
		if err := form.Run(); err == nil && selected != "" {
			connectDefaultCmd(selected)
		}
	}
}

func connectAddCmd() {
	scanner := bufio.NewScanner(os.Stdin)

	fmt.Print(promptStyle.Render("Profile name (e.g., desktop-gpu): "))
	scanner.Scan()
	name := strings.TrimSpace(scanner.Text())
	if name == "" {
		printError("Profile name cannot be empty", nil)
		return
	}

	for _, p := range config.Profiles {
		if p.Name == name {
			printError("Profile already exists", nil)
			return
		}
	}

	fmt.Print(promptStyle.Render("Provider (1: Ollama, 2: LM Studio) [default: 1]: "))
	scanner.Scan()
	providerChoice := strings.TrimSpace(scanner.Text())
	provider := "ollama"
	defaultHost := "http://localhost:11434"
	if providerChoice == "2" {
		provider = "lmstudio"
		defaultHost = "http://localhost:1234"
	}

	fmt.Printf(promptStyle.Render("%s Host (default: %s): "), providerDisplayName(provider), defaultHost)
	scanner.Scan()
	host := strings.TrimSpace(scanner.Text())
	if host == "" {
		host = defaultHost
	}
	if !strings.HasPrefix(host, "http") {
		host = "http://" + host
	}
	host = strings.TrimSuffix(host, "/")

	fmt.Print(promptStyle.Render("Auth Token (optional): "))
	scanner.Scan()
	token := strings.TrimSpace(scanner.Text())

	config.Profiles = append(config.Profiles, ServerProfile{
		Name:     name,
		Host:     host,
		Token:    token,
		Provider: provider,
	})
	if config.DefaultProfile == "" {
		config.DefaultProfile = name
	}
	config.ActiveProfile = name
	saveConfig()
	printSuccess(fmt.Sprintf("Profile '%s' added and activated!", name))
}

func connectListCmd() {
	fmt.Printf("\n%s\n", titleStyle.Render("🔌 Server Profiles"))
	fmt.Println(separatorStyle.Render(strings.Repeat("─", 60)))

	if len(config.Profiles) == 0 {
		fmt.Println(warningStyle.Render("No profiles configured."))
		return
	}

	for _, p := range config.Profiles {
		active := ""
		if p.Name == config.ActiveProfile {
			active = successStyle.Render(" (active)")
		}
		defaultMarker := ""
		if p.Name == config.DefaultProfile {
			defaultMarker = infoStyle.Render(" (default)")
		}
		auth := ""
		if p.Token != "" {
			auth = infoStyle.Render(" [auth]")
		}
		fmt.Printf("  %s (%s)%s%s%s\n    %s\n", modelNameStyle.Render(p.Name), providerDisplayName(p.Provider), active, defaultMarker, auth, p.Host)
	}
}

func connectUseCmd(name string) {
	for _, p := range config.Profiles {
		if p.Name == name {
			config.ActiveProfile = name
			saveConfig()
			printSuccess(fmt.Sprintf("Activated profile '%s'", name))
			return
		}
	}
	printError(fmt.Sprintf("Profile '%s' not found", name), nil)
}

func connectDefaultCmd(name string) {
	for _, p := range config.Profiles {
		if p.Name == name {
			config.DefaultProfile = name
			if config.ActiveProfile == "" {
				config.ActiveProfile = name
			}
			saveConfig()
			printSuccess(fmt.Sprintf("Default profile set to '%s'", name))
			return
		}
	}
	printError(fmt.Sprintf("Profile '%s' not found", name), nil)
}

func connectRemoveCmd(name string) {
	if len(config.Profiles) <= 1 {
		printError("Cannot remove the only configured profile", nil)
		return
	}

	var newProfiles []ServerProfile
	found := false
	for _, p := range config.Profiles {
		if p.Name == name {
			found = true
			continue
		}
		newProfiles = append(newProfiles, p)
	}

	if !found {
		printError(fmt.Sprintf("Profile '%s' not found", name), nil)
		return
	}

	config.Profiles = newProfiles
	if config.DefaultProfile == name {
		config.DefaultProfile = config.Profiles[0].Name
	}
	if config.ActiveProfile == name {
		config.ActiveProfile = config.DefaultProfile
	}
	saveConfig()
	printSuccess(fmt.Sprintf("Removed profile '%s'", name))
}

func connectEditCurrent() {
	profile := activeProfile()
	scanner := bufio.NewScanner(os.Stdin)

	fmt.Printf("\n%s %s\n", infoStyle.Render("Editing profile:"), modelNameStyle.Render(profile.Name))

	fmt.Printf("Provider (%s - press Enter to keep current, 1: Ollama, 2: LM Studio): ", providerDisplayName(profile.Provider))
	scanner.Scan()
	provChoice := strings.TrimSpace(scanner.Text())
	switch provChoice {
	case "1":
		profile.Provider = "ollama"
	case "2":
		profile.Provider = "lmstudio"
	}

	fmt.Printf("Host (%s): ", profile.Host)
	scanner.Scan()
	host := strings.TrimSpace(scanner.Text())
	if host != "" {
		if !strings.HasPrefix(host, "http") {
			host = "http://" + host
		}
		profile.Host = strings.TrimSuffix(host, "/")
	}

	fmt.Print("Token (press Enter to keep current): ")
	scanner.Scan()
	token := strings.TrimSpace(scanner.Text())
	if token != "" {
		profile.Token = token
	}

	for i := range config.Profiles {
		if config.Profiles[i].Name == profile.Name {
			config.Profiles[i] = profile
			break
		}
	}
	saveConfig()
	printSuccess("Profile updated!")
}
