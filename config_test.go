package main

import "testing"

func TestDefaultServerProfile(t *testing.T) {
	got := defaultServerProfile()
	want := ServerProfile{Name: "local-ollama", Host: "http://localhost:11434", Provider: "ollama"}
	if got != want {
		t.Errorf("defaultServerProfile() = %+v, want %+v", got, want)
	}
}

func TestProfileNameForHost(t *testing.T) {
	tests := []struct {
		name     string
		provider string
		host     string
		want     string
	}{
		{"ollama localhost", "ollama", "http://localhost:11434", "ollama-localhost-11434"},
		{"ollama https ip", "ollama", "https://192.168.1.10:11434", "ollama-192-168-1-10-11434"},
		{"empty provider defaults to ollama", "", "http://blackdragon:11434", "ollama-blackdragon-11434"},
		{"lmstudio provider", "lmstudio", "http://10.1.6.30:1234", "lmstudio-10-1-6-30-1234"},
		{"path in host", "ollama", "http://host/ollama", "ollama-host-ollama"},
		{"empty host falls back to server", "ollama", "", "ollama-server"},
		{"trailing slash stripped", "ollama", "http://host:11434/", "ollama-host-11434"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := profileNameForHost(tt.provider, tt.host)
			if got != tt.want {
				t.Errorf("profileNameForHost(%q, %q) = %q, want %q", tt.provider, tt.host, got, tt.want)
			}
		})
	}
}

func TestUniqueProfileName(t *testing.T) {
	tests := []struct {
		name     string
		base     string
		profiles []ServerProfile
		want     string
	}{
		{"no conflicts", "myserver", nil, "myserver"},
		{"one conflict returns -2", "myserver", []ServerProfile{{Name: "myserver"}}, "myserver-2"},
		{"two sequential conflicts returns -3", "myserver", []ServerProfile{{Name: "myserver"}, {Name: "myserver-2"}}, "myserver-3"},
		{"gap in sequence fills lowest", "myserver", []ServerProfile{{Name: "myserver"}, {Name: "myserver-3"}}, "myserver-2"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := uniqueProfileName(tt.base, tt.profiles)
			if got != tt.want {
				t.Errorf("uniqueProfileName(%q, %+v) = %q, want %q", tt.base, tt.profiles, got, tt.want)
			}
		})
	}
}

func TestNormalizeConfig(t *testing.T) {
	tests := []struct {
		name   string
		setup  func()
		want   bool
		verify func(*testing.T)
	}{
		{
			name: "empty profiles creates default",
			setup: func() {
				config = Config{
					Profiles: []ServerProfile{},
				}
			},
			want: true,
			verify: func(t *testing.T) {
				if len(config.Profiles) != 1 {
					t.Fatalf("expected 1 profile, got %d", len(config.Profiles))
				}
				if config.Profiles[0] != defaultServerProfile() {
					t.Errorf("profile = %+v, want %+v", config.Profiles[0], defaultServerProfile())
				}
				if config.ActiveProfile != "local-ollama" {
					t.Errorf("ActiveProfile = %q, want %q", config.ActiveProfile, "local-ollama")
				}
				if config.DefaultProfile != "local-ollama" {
					t.Errorf("DefaultProfile = %q, want %q", config.DefaultProfile, "local-ollama")
				}
			},
		},
		{
			name: "empty provider defaults to ollama",
			setup: func() {
				config = Config{
					Profiles: []ServerProfile{
						{Name: "myserver", Host: "http://localhost:11434"},
					},
					DefaultProfile: "myserver",
					ActiveProfile:  "myserver",
				}
			},
			want: true,
			verify: func(t *testing.T) {
				if config.Profiles[0].Provider != "ollama" {
					t.Errorf("Provider = %q, want %q", config.Profiles[0].Provider, "ollama")
				}
			},
		},
		{
			name: "profile named default is renamed",
			setup: func() {
				config = Config{
					Profiles: []ServerProfile{
						{Name: "default", Host: "http://localhost:11434", Provider: "ollama"},
					},
					DefaultProfile: "default",
					ActiveProfile:  "default",
				}
			},
			want: true,
			verify: func(t *testing.T) {
				if config.Profiles[0].Name == "default" {
					t.Errorf("profile name should not remain 'default'")
				}
				if config.Profiles[0].Name != "ollama-localhost-11434" {
					t.Errorf("Name = %q, want %q", config.Profiles[0].Name, "ollama-localhost-11434")
				}
				if config.DefaultProfile != "ollama-localhost-11434" {
					t.Errorf("DefaultProfile = %q, want %q", config.DefaultProfile, "ollama-localhost-11434")
				}
			},
		},
		{
			name: "already normalized returns false",
			setup: func() {
				config = Config{
					Profiles: []ServerProfile{
						{Name: "myserver", Host: "http://localhost:11434", Provider: "ollama"},
					},
					DefaultProfile: "myserver",
					ActiveProfile:  "myserver",
				}
			},
			want: false,
		},
		{
			name: "empty active profile defaults to default profile",
			setup: func() {
				config = Config{
					Profiles: []ServerProfile{
						{Name: "myserver", Host: "http://localhost:11434", Provider: "ollama"},
					},
					DefaultProfile: "myserver",
					ActiveProfile:  "",
				}
			},
			want: true,
			verify: func(t *testing.T) {
				if config.ActiveProfile != "myserver" {
					t.Errorf("ActiveProfile = %q, want %q", config.ActiveProfile, "myserver")
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			origConfig := config
			defer func() { config = origConfig }()
			tt.setup()
			got := normalizeConfig()
			if got != tt.want {
				t.Errorf("normalizeConfig() = %v, want %v", got, tt.want)
			}
			if tt.verify != nil {
				tt.verify(t)
			}
		})
	}
}

func TestProfileByName(t *testing.T) {
	origConfig := config
	defer func() { config = origConfig }()
	config = Config{
		Profiles: []ServerProfile{
			{Name: "alpha", Host: "http://alpha:11434", Provider: "ollama"},
			{Name: "beta", Host: "http://beta:11434", Provider: "lmstudio"},
		},
	}
	tests := []struct {
		name  string
		want  ServerProfile
		found bool
	}{
		{"alpha", ServerProfile{Name: "alpha", Host: "http://alpha:11434", Provider: "ollama"}, true},
		{"beta", ServerProfile{Name: "beta", Host: "http://beta:11434", Provider: "lmstudio"}, true},
		{"nonexistent", ServerProfile{}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := profileByName(tt.name)
			if ok != tt.found {
				t.Errorf("profileByName(%q) ok = %v, want %v", tt.name, ok, tt.found)
			}
			if got != tt.want {
				t.Errorf("profileByName(%q) = %+v, want %+v", tt.name, got, tt.want)
			}
		})
	}
}

func TestInferProviderFromServerName(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{"ollama-localhost", "ollama"},
		{"lmstudio-desktop", "lmstudio"},
		{"custom-name", ""},
		{"", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := inferProviderFromServerName(tt.name)
			if got != tt.want {
				t.Errorf("inferProviderFromServerName(%q) = %q, want %q", tt.name, got, tt.want)
			}
		})
	}
}
