package cli

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestLoadGlobalConfig(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")

	yamlContent := `# Kalyx Global Config
current_profile: staging
profiles:
  local:
    server: http://localhost:8080
    token: kalyx_sec_local_123
  staging:
    server: https://staging.kalyx.internal
    token: kalyx_sec_staging_456
`
	if err := os.WriteFile(configPath, []byte(yamlContent), 0600); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	cfg, err := LoadGlobalConfig(configPath)
	if err != nil {
		t.Fatalf("LoadGlobalConfig failed: %v", err)
	}

	if cfg.CurrentProfile != "staging" {
		t.Errorf("expected current_profile to be 'staging', got '%s'", cfg.CurrentProfile)
	}

	if len(cfg.Profiles) != 2 {
		t.Fatalf("expected 2 profiles, got %d", len(cfg.Profiles))
	}

	localProf := cfg.Profiles["local"]
	if localProf.Server != "http://localhost:8080" || localProf.Token != "kalyx_sec_local_123" {
		t.Errorf("unexpected local profile: %+v", localProf)
	}

	stagingProf := cfg.Profiles["staging"]
	if stagingProf.Server != "https://staging.kalyx.internal" || stagingProf.Token != "kalyx_sec_staging_456" {
		t.Errorf("unexpected staging profile: %+v", stagingProf)
	}
}

func TestLoadProjectConfig(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, ".kalyx.yaml")

	yamlContent := `server: https://kalyx.prod.internal
lang: python
out: ./sdk/events
events:
  - com.company.orders.completed.v1
  - com.company.payments.received.v1
`
	if err := os.WriteFile(configPath, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("failed to write project config: %v", err)
	}

	cfg, err := LoadProjectConfig(configPath)
	if err != nil {
		t.Fatalf("LoadProjectConfig failed: %v", err)
	}

	if cfg.Server != "https://kalyx.prod.internal" {
		t.Errorf("expected server 'https://kalyx.prod.internal', got '%s'", cfg.Server)
	}
	if cfg.Lang != "python" {
		t.Errorf("expected lang 'python', got '%s'", cfg.Lang)
	}
	if cfg.Out != "./sdk/events" {
		t.Errorf("expected out './sdk/events', got '%s'", cfg.Out)
	}

	expectedEvents := []string{
		"com.company.orders.completed.v1",
		"com.company.payments.received.v1",
	}
	if !reflect.DeepEqual(cfg.Events, expectedEvents) {
		t.Errorf("expected events %v, got %v", expectedEvents, cfg.Events)
	}
}

func TestSaveToken(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")

	// Save token in new file
	err := SaveToken(configPath, "prod", "https://kalyx.prod.internal", "kalyx_sec_prod_999")
	if err != nil {
		t.Fatalf("SaveToken failed: %v", err)
	}

	cfg, err := LoadGlobalConfig(configPath)
	if err != nil {
		t.Fatalf("LoadGlobalConfig failed: %v", err)
	}

	if cfg.CurrentProfile != "prod" {
		t.Errorf("expected current profile 'prod', got '%s'", cfg.CurrentProfile)
	}

	prof, ok := cfg.Profiles["prod"]
	if !ok {
		t.Fatalf("expected profile 'prod' to exist")
	}
	if prof.Server != "https://kalyx.prod.internal" || prof.Token != "kalyx_sec_prod_999" {
		t.Errorf("unexpected profile saved: %+v", prof)
	}

	// Update existing profile token
	err = SaveToken(configPath, "prod", "", "kalyx_sec_prod_updated")
	if err != nil {
		t.Fatalf("SaveToken update failed: %v", err)
	}

	updatedCfg, err := LoadGlobalConfig(configPath)
	if err != nil {
		t.Fatalf("LoadGlobalConfig failed: %v", err)
	}
	if updatedCfg.Profiles["prod"].Token != "kalyx_sec_prod_updated" {
		t.Errorf("expected updated token 'kalyx_sec_prod_updated', got '%s'", updatedCfg.Profiles["prod"].Token)
	}
	if updatedCfg.Profiles["prod"].Server != "https://kalyx.prod.internal" {
		t.Errorf("expected server to be preserved, got '%s'", updatedCfg.Profiles["prod"].Server)
	}
}

func TestResolveConfigPrecedence(t *testing.T) {
	tempDir := t.TempDir()
	globalPath := filepath.Join(tempDir, "config.yaml")
	projectPath := filepath.Join(tempDir, ".kalyx.yaml")

	// 1. Setup global profile
	globalYAML := `current_profile: local
profiles:
  local:
    server: http://global-server:8080
    token: kalyx_sec_global_token
`
	_ = os.WriteFile(globalPath, []byte(globalYAML), 0600)

	// 2. Setup project config
	projectYAML := `server: http://project-server:8080
lang: python
out: ./project-out
events:
  - orders.completed.v1
  - orders.refunded.v1
`
	_ = os.WriteFile(projectPath, []byte(projectYAML), 0644)

	// Case A: No flags -> Project overrides Global
	effective, err := ResolveConfig(ConfigFlags{}, projectPath, globalPath)
	if err != nil {
		t.Fatalf("ResolveConfig failed: %v", err)
	}
	if effective.Server != "http://project-server:8080" {
		t.Errorf("expected project server, got '%s'", effective.Server)
	}
	if effective.Token != "kalyx_sec_global_token" {
		t.Errorf("expected token from global profile, got '%s'", effective.Token)
	}
	if effective.Lang != "python" {
		t.Errorf("expected lang 'python', got '%s'", effective.Lang)
	}
	if effective.Out != "./project-out" {
		t.Errorf("expected out './project-out', got '%s'", effective.Out)
	}
	if len(effective.Events) != 2 {
		t.Errorf("expected 2 events, got %d", len(effective.Events))
	}

	// Case B: Flags override everything
	flags := ConfigFlags{
		Server: "http://override-server:9090",
		Token:  "kalyx_sec_flag_token",
		Lang:   "go",
		Out:    "./flag-out",
		Event:  "single.event.v1",
	}
	effectiveWithFlags, err := ResolveConfig(flags, projectPath, globalPath)
	if err != nil {
		t.Fatalf("ResolveConfig with flags failed: %v", err)
	}
	if effectiveWithFlags.Server != "http://override-server:9090" {
		t.Errorf("expected flag server, got '%s'", effectiveWithFlags.Server)
	}
	if effectiveWithFlags.Token != "kalyx_sec_flag_token" {
		t.Errorf("expected flag token, got '%s'", effectiveWithFlags.Token)
	}
	if effectiveWithFlags.Lang != "go" {
		t.Errorf("expected flag lang 'go', got '%s'", effectiveWithFlags.Lang)
	}
	if effectiveWithFlags.Out != "./flag-out" {
		t.Errorf("expected flag out './flag-out', got '%s'", effectiveWithFlags.Out)
	}
	if len(effectiveWithFlags.Events) != 1 || effectiveWithFlags.Events[0] != "single.event.v1" {
		t.Errorf("expected single flag event, got %v", effectiveWithFlags.Events)
	}
}

func TestResolveConfigDefaults(t *testing.T) {
	// Neither global nor project exists
	effective, err := ResolveConfig(ConfigFlags{}, "non-existent-proj.yaml", "non-existent-global.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if effective.Server != "http://localhost:8080" {
		t.Errorf("expected default server 'http://localhost:8080', got '%s'", effective.Server)
	}
	if effective.Lang != "typescript" {
		t.Errorf("expected default lang 'typescript', got '%s'", effective.Lang)
	}
	if effective.Out != "./generated" {
		t.Errorf("expected default out './generated', got '%s'", effective.Out)
	}
}
