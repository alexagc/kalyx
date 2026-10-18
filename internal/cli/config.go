package cli

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Profile represents a named target server and credential profile in ~/.kalyx/config.yaml.
type Profile struct {
	Server string `yaml:"server" json:"server"`
	Token  string `yaml:"token" json:"token"`
}

// GlobalConfig represents the global CLI configuration at ~/.kalyx/config.yaml.
type GlobalConfig struct {
	CurrentProfile string             `yaml:"current_profile" json:"current_profile"`
	Profiles       map[string]Profile `yaml:"profiles" json:"profiles"`
	// Fallback fields for flat global configs
	Server string `yaml:"server,omitempty" json:"server,omitempty"`
	Token  string `yaml:"token,omitempty" json:"token,omitempty"`
}

// ProjectConfig represents the local microservice configuration at .kalyx.yaml.
type ProjectConfig struct {
	Server string   `yaml:"server" json:"server"`
	Lang   string   `yaml:"lang" json:"lang"`
	Out    string   `yaml:"out" json:"out"`
	Events []string `yaml:"events" json:"events"`
}

// ConfigFlags captures command-line flags provided to kalyxctl commands.
type ConfigFlags struct {
	Server  string
	Token   string
	Lang    string
	Out     string
	Event   string
	Profile string
}

// EffectiveConfig represents the final resolved configuration after merging flags, project, and global settings.
type EffectiveConfig struct {
	Server string
	Token  string
	Lang   string
	Out    string
	Events []string
}

// DefaultGlobalConfigPath returns the canonical path to ~/.kalyx/config.yaml.
func DefaultGlobalConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}
	return filepath.Join(home, ".kalyx", "config.yaml"), nil
}

// DefaultProjectConfigPath returns the canonical local project config file name.
func DefaultProjectConfigPath() string {
	return ".kalyx.yaml"
}

// LoadGlobalConfig reads the global configuration file. If the file does not exist,
// an empty configuration with an initialized Profiles map is returned without error.
func LoadGlobalConfig(path string) (*GlobalConfig, error) {
	cfg := &GlobalConfig{
		Profiles: make(map[string]Profile),
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return cfg, nil
		}
		return nil, fmt.Errorf("failed to read global config: %w", err)
	}

	// Try JSON first
	if jsonErr := json.Unmarshal(data, cfg); jsonErr == nil && (cfg.CurrentProfile != "" || len(cfg.Profiles) > 0 || cfg.Server != "") {
		if cfg.Profiles == nil {
			cfg.Profiles = make(map[string]Profile)
		}
		return cfg, nil
	}

	// Parse YAML
	scanner := bufio.NewScanner(bytes.NewReader(data))
	var currentProfName string
	inProfiles := false

	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		// Skip comments and blank lines
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		// Strip inline comments
		if idx := strings.Index(trimmed, " #"); idx != -1 {
			trimmed = strings.TrimSpace(trimmed[:idx])
		}

		indent := len(line) - len(strings.TrimLeft(line, " "))

		if indent == 0 {
			inProfiles = false
			currentProfName = ""

			parts := strings.SplitN(trimmed, ":", 2)
			key := strings.TrimSpace(parts[0])
			val := ""
			if len(parts) > 1 {
				val = cleanScalar(parts[1])
			}

			switch key {
			case "current_profile":
				cfg.CurrentProfile = val
			case "server":
				cfg.Server = val
			case "token":
				cfg.Token = val
			case "profiles":
				inProfiles = true
			}
		} else if inProfiles {
			if indent == 2 {
				// Profile name (e.g., "  local:")
				profKey := strings.TrimSuffix(trimmed, ":")
				currentProfName = strings.TrimSpace(profKey)
				if cfg.Profiles[currentProfName].Server == "" && cfg.Profiles[currentProfName].Token == "" {
					cfg.Profiles[currentProfName] = Profile{}
				}
			} else if indent >= 4 && currentProfName != "" {
				// Property inside profile (e.g., "    server: http://...")
				parts := strings.SplitN(trimmed, ":", 2)
				if len(parts) == 2 {
					propKey := strings.TrimSpace(parts[0])
					propVal := cleanScalar(parts[1])
					prof := cfg.Profiles[currentProfName]
					switch propKey {
					case "server":
						prof.Server = propVal
					case "token":
						prof.Token = propVal
					}
					cfg.Profiles[currentProfName] = prof
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to parse global config YAML: %w", err)
	}

	// If top-level server or token exist and current profile is empty, set default profile
	if cfg.CurrentProfile == "" && len(cfg.Profiles) > 0 {
		for name := range cfg.Profiles {
			cfg.CurrentProfile = name
			break
		}
	} else if cfg.CurrentProfile == "" && (cfg.Server != "" || cfg.Token != "") {
		cfg.CurrentProfile = "default"
		cfg.Profiles["default"] = Profile{
			Server: cfg.Server,
			Token:  cfg.Token,
		}
	}

	return cfg, nil
}

// SaveGlobalConfig saves the global configuration to the specified YAML file.
func SaveGlobalConfig(path string, cfg *GlobalConfig) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory %s: %w", dir, err)
	}

	var sb strings.Builder
	sb.WriteString("# Kalyx Global CLI Configuration\n")
	if cfg.CurrentProfile != "" {
		sb.WriteString(fmt.Sprintf("current_profile: %s\n", cfg.CurrentProfile))
	} else {
		sb.WriteString("current_profile: default\n")
	}

	sb.WriteString("profiles:\n")
	if len(cfg.Profiles) == 0 {
		sb.WriteString("  default:\n")
		sb.WriteString("    server: http://localhost:8080\n")
		sb.WriteString("    token: \"\"\n")
	} else {
		for name, prof := range cfg.Profiles {
			sb.WriteString(fmt.Sprintf("  %s:\n", name))
			server := prof.Server
			if server == "" {
				server = "http://localhost:8080"
			}
			sb.WriteString(fmt.Sprintf("    server: %s\n", server))
			sb.WriteString(fmt.Sprintf("    token: %s\n", prof.Token))
		}
	}

	if err := os.WriteFile(path, []byte(sb.String()), 0600); err != nil {
		return fmt.Errorf("failed to write global config to %s: %w", path, err)
	}
	return nil
}

// LoadProjectConfig reads the project-level .kalyx.yaml configuration file.
func LoadProjectConfig(path string) (*ProjectConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	cfg := &ProjectConfig{}

	// Try JSON first
	if jsonErr := json.Unmarshal(data, cfg); jsonErr == nil && (cfg.Server != "" || cfg.Lang != "" || len(cfg.Events) > 0) {
		return cfg, nil
	}

	// Parse YAML
	scanner := bufio.NewScanner(bytes.NewReader(data))
	inEvents := false

	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		if idx := strings.Index(trimmed, " #"); idx != -1 {
			trimmed = strings.TrimSpace(trimmed[:idx])
		}

		indent := len(line) - len(strings.TrimLeft(line, " "))

		if strings.HasPrefix(trimmed, "- ") && inEvents {
			ev := cleanScalar(strings.TrimPrefix(trimmed, "- "))
			if ev != "" {
				cfg.Events = append(cfg.Events, ev)
			}
			continue
		}

		parts := strings.SplitN(trimmed, ":", 2)
		if len(parts) == 0 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		val := ""
		if len(parts) > 1 {
			val = cleanScalar(parts[1])
		}

		if indent == 0 {
			switch key {
			case "server":
				cfg.Server = val
				inEvents = false
			case "lang":
				cfg.Lang = val
				inEvents = false
			case "out":
				cfg.Out = val
				inEvents = false
			case "events":
				inEvents = true
			default:
				inEvents = false
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to parse project config YAML: %w", err)
	}

	return cfg, nil
}

// SaveProjectConfig writes a project configuration file in YAML format.
func SaveProjectConfig(path string, cfg *ProjectConfig) error {
	var sb strings.Builder
	sb.WriteString("# Kalyx Microservice Event Configuration\n")
	if cfg.Server != "" {
		sb.WriteString(fmt.Sprintf("server: %s\n", cfg.Server))
	}
	if cfg.Lang != "" {
		sb.WriteString(fmt.Sprintf("lang: %s\n", cfg.Lang))
	}
	if cfg.Out != "" {
		sb.WriteString(fmt.Sprintf("out: %s\n", cfg.Out))
	}
	if len(cfg.Events) > 0 {
		sb.WriteString("events:\n")
		for _, ev := range cfg.Events {
			sb.WriteString(fmt.Sprintf("  - %s\n", ev))
		}
	}

	if err := os.WriteFile(path, []byte(sb.String()), 0644); err != nil {
		return fmt.Errorf("failed to write project config to %s: %w", path, err)
	}
	return nil
}

// SaveToken records a Personal Access Token (and optional server) into the specified or active profile.
func SaveToken(globalPath, profileName, server, token string) error {
	if globalPath == "" {
		defaultPath, err := DefaultGlobalConfigPath()
		if err != nil {
			return err
		}
		globalPath = defaultPath
	}

	cfg, err := LoadGlobalConfig(globalPath)
	if err != nil {
		cfg = &GlobalConfig{
			Profiles: make(map[string]Profile),
		}
	}

	if profileName == "" {
		if cfg.CurrentProfile != "" {
			profileName = cfg.CurrentProfile
		} else {
			profileName = "default"
		}
	}

	cfg.CurrentProfile = profileName
	if cfg.Profiles == nil {
		cfg.Profiles = make(map[string]Profile)
	}

	prof := cfg.Profiles[profileName]
	if server != "" {
		prof.Server = server
	} else if prof.Server == "" {
		prof.Server = "http://localhost:8080"
	}
	prof.Token = token
	cfg.Profiles[profileName] = prof

	return SaveGlobalConfig(globalPath, cfg)
}

// ResolveConfig resolves the effective configuration by evaluating flags, the local
// project file (.kalyx.yaml), and the global profile (~/.kalyx/config.yaml) in order of precedence:
// Flags > Project Config > Global Profile > Defaults.
func ResolveConfig(flags ConfigFlags, projectPath, globalPath string) (*EffectiveConfig, error) {
	effective := &EffectiveConfig{
		Server: "http://localhost:8080",
		Lang:   "typescript",
		Out:    "./generated",
		Events: []string{},
	}

	// 1. Global profile
	if globalPath == "" {
		if p, err := DefaultGlobalConfigPath(); err == nil {
			globalPath = p
		}
	}
	if globalPath != "" {
		if gCfg, err := LoadGlobalConfig(globalPath); err == nil && gCfg != nil {
			activeProfile := gCfg.CurrentProfile
			if flags.Profile != "" {
				activeProfile = flags.Profile
			}

			if prof, exists := gCfg.Profiles[activeProfile]; exists {
				if prof.Server != "" {
					effective.Server = prof.Server
				}
				if prof.Token != "" {
					effective.Token = prof.Token
				}
			} else if gCfg.Server != "" || gCfg.Token != "" {
				if gCfg.Server != "" {
					effective.Server = gCfg.Server
				}
				if gCfg.Token != "" {
					effective.Token = gCfg.Token
				}
			}
		}
	}

	// 2. Environment variables override global config
	if envServer := os.Getenv("KALYX_SERVER"); envServer != "" {
		effective.Server = envServer
	}
	if envToken := os.Getenv("KALYX_TOKEN"); envToken != "" {
		effective.Token = envToken
	}

	// 3. Project configuration (.kalyx.yaml)
	if projectPath == "" {
		projectPath = DefaultProjectConfigPath()
	}
	if pCfg, err := LoadProjectConfig(projectPath); err == nil && pCfg != nil {
		if pCfg.Server != "" {
			effective.Server = pCfg.Server
		}
		if pCfg.Lang != "" {
			effective.Lang = pCfg.Lang
		}
		if pCfg.Out != "" {
			effective.Out = pCfg.Out
		}
		if len(pCfg.Events) > 0 {
			effective.Events = pCfg.Events
		}
	}

	// 4. Command-line flags (highest precedence)
	if flags.Server != "" {
		effective.Server = flags.Server
	}
	if flags.Token != "" {
		effective.Token = flags.Token
	}
	if flags.Lang != "" {
		effective.Lang = flags.Lang
	}
	if flags.Out != "" {
		effective.Out = flags.Out
	}
	if flags.Event != "" {
		effective.Events = []string{flags.Event}
	}

	return effective, nil
}

func cleanScalar(val string) string {
	val = strings.TrimSpace(val)
	val = strings.Trim(val, `"'`)
	return strings.TrimSpace(val)
}
