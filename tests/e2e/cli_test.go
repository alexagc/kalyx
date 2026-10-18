package e2e

import (
	"context"
	"fmt"
	"go/parser"
	"go/token"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/alexagc/kalyx/internal/cli"
	"github.com/alexagc/kalyx/internal/core"
	"github.com/alexagc/kalyx/internal/storage"
	"github.com/alexagc/kalyx/internal/webserver"
)

var binaryPath string

func TestMain(m *testing.M) {
	tempDir, err := os.MkdirTemp("", "kalyxctl-e2e-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create temp dir: %v\n", err)
		os.Exit(1)
	}
	defer os.RemoveAll(tempDir)

	binName := "kalyxctl"
	if os.PathSeparator == '\\' {
		binName += ".exe"
	}
	binaryPath = filepath.Join(tempDir, binName)

	buildCmd := exec.Command("go", "build", "-o", binaryPath, "github.com/alexagc/kalyx/cmd/kalyxctl")
	// If go is in Program Files, try absolute path if 'go' fails
	if out, err := buildCmd.CombinedOutput(); err != nil {
		goExe := `C:\Program Files\Go\bin\go.exe`
		if _, statErr := os.Stat(goExe); statErr == nil {
			fallbackCmd := exec.Command(goExe, "build", "-o", binaryPath, "github.com/alexagc/kalyx/cmd/kalyxctl")
			if fbOut, fbErr := fallbackCmd.CombinedOutput(); fbErr != nil {
				fmt.Fprintf(os.Stderr, "failed to build kalyxctl: %v\nOutput: %s\n", fbErr, string(fbOut))
				os.Exit(1)
			}
		} else {
			fmt.Fprintf(os.Stderr, "failed to build kalyxctl: %v\nOutput: %s\n", err, string(out))
			os.Exit(1)
		}
	}

	code := m.Run()
	os.Exit(code)
}

func setupTestServer(t *testing.T) (*httptest.Server, *storage.MemoryStorage) {
	t.Helper()
	store := storage.NewMemoryStorage()
	ctx := context.Background()

	// Add a deprecated contract to verify RFC 8594 warnings
	depEvent := core.EventDefinition{
		ID:          "ev_users_registered",
		Domain:      "Identity",
		Name:        "UserRegistered",
		Type:        "com.company.users.registered.v1",
		Description: "Legacy user registration event",
		OwnerTeam:   "Identity Core",
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}
	if err := store.CreateEvent(ctx, depEvent); err != nil {
		t.Fatalf("failed to create deprecated event: %v", err)
	}

	depVersion := core.EventVersion{
		ID:         "ver_user_reg_1_0_0",
		EventID:    depEvent.ID,
		SemVer:     "1.0.0",
		Status:     core.StatusDeprecated,
		CreatedAt:  time.Now().UTC(),
		CreatedBy:  "bob@company.com",
		JSONSchema: `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "title": "UserRegisteredPayload",
  "type": "object",
  "properties": {
    "user_id": { "type": "string", "description": "Unique user identifier" },
    "email": { "type": "string", "description": "User email address" }
  },
  "required": ["user_id", "email"]
}`,
	}
	if err := store.CreateEventVersion(ctx, depVersion); err != nil {
		t.Fatalf("failed to create deprecated event version: %v", err)
	}

	server := webserver.NewServer(store)
	ts := httptest.NewServer(server)
	return ts, store
}

func runCLI(t *testing.T, dir string, env []string, args ...string) (string, error) {
	t.Helper()
	cmd := exec.Command(binaryPath, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	cmd.Env = append(os.Environ(), env...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func TestCLIVersion(t *testing.T) {
	out, err := runCLI(t, "", nil, "version")
	if err != nil {
		t.Fatalf("kalyxctl version failed: %v\nOutput: %s", err, out)
	}
	if !strings.Contains(out, "kalyxctl version 0.1.0") {
		t.Errorf("unexpected version output: %s", out)
	}
}

func TestCLILogin(t *testing.T) {
	tempHome := t.TempDir()
	env := []string{
		"USERPROFILE=" + tempHome,
		"HOME=" + tempHome,
	}

	ts, _ := setupTestServer(t)
	defer ts.Close()

	out, err := runCLI(t, "", env, "login", "--token", "kalyx_sec_e2e_pat_12345", "--server", ts.URL, "--profile", "e2e-profile")
	if err != nil {
		t.Fatalf("kalyxctl login failed: %v\nOutput: %s", err, out)
	}

	if !strings.Contains(out, "Successfully authenticated with Kalyx") {
		t.Errorf("expected success message in output: %s", out)
	}

	// Verify global config file was created
	configPath := filepath.Join(tempHome, ".kalyx", "config.yaml")
	cfg, err := cli.LoadGlobalConfig(configPath)
	if err != nil {
		t.Fatalf("failed to load saved config: %v", err)
	}

	if cfg.CurrentProfile != "e2e-profile" {
		t.Errorf("expected current profile 'e2e-profile', got '%s'", cfg.CurrentProfile)
	}

	prof, ok := cfg.Profiles["e2e-profile"]
	if !ok {
		t.Fatalf("profile 'e2e-profile' not found in saved config")
	}

	if prof.Token != "kalyx_sec_e2e_pat_12345" {
		t.Errorf("expected token 'kalyx_sec_e2e_pat_12345', got '%s'", prof.Token)
	}
	if prof.Server != ts.URL {
		t.Errorf("expected server '%s', got '%s'", ts.URL, prof.Server)
	}
}

func TestCLIGenerateFlags(t *testing.T) {
	ts, _ := setupTestServer(t)
	defer ts.Close()

	tempDir := t.TempDir()

	// 1. Generate TypeScript
	tsOutDir := filepath.Join(tempDir, "ts_sdk")
	out, err := runCLI(t, "", nil,
		"generate",
		"--event", "com.company.orders.completed.v1",
		"--lang", "typescript",
		"--out", tsOutDir,
		"--server", ts.URL,
	)
	if err != nil {
		t.Fatalf("generate typescript failed: %v\nOutput: %s", err, out)
	}

	tsFile := filepath.Join(tsOutDir, "order_completed.ts")
	tsBytes, err := os.ReadFile(tsFile)
	if err != nil {
		t.Fatalf("generated TypeScript file not found at %s: %v", tsFile, err)
	}
	tsContent := string(tsBytes)

	if !strings.Contains(tsContent, "export interface OrderCompletedPayload") {
		t.Errorf("TypeScript file missing OrderCompletedPayload")
	}
	if !strings.Contains(tsContent, "export interface OrderCompletedCloudEvent") {
		t.Errorf("TypeScript file missing OrderCompletedCloudEvent")
	}
	if !strings.Contains(tsContent, "export function createOrderCompletedCloudEvent") {
		t.Errorf("TypeScript file missing createOrderCompletedCloudEvent")
	}
	if !strings.Contains(tsContent, "export function validateOrderCompletedPayload") {
		t.Errorf("TypeScript file missing validateOrderCompletedPayload")
	}

	// 2. Generate Python
	pyOutDir := filepath.Join(tempDir, "py_sdk")
	out, err = runCLI(t, "", nil,
		"generate",
		"--event", "com.company.orders.completed.v1",
		"--lang", "python",
		"--out", pyOutDir,
		"--server", ts.URL,
	)
	if err != nil {
		t.Fatalf("generate python failed: %v\nOutput: %s", err, out)
	}

	pyFile := filepath.Join(pyOutDir, "order_completed.py")
	pyBytes, err := os.ReadFile(pyFile)
	if err != nil {
		t.Fatalf("generated Python file not found at %s: %v", pyFile, err)
	}
	pyContent := string(pyBytes)

	if !strings.Contains(pyContent, "class OrderCompletedPayload(BaseModel):") {
		t.Errorf("Python file missing OrderCompletedPayload")
	}
	if !strings.Contains(pyContent, "class OrderCompletedCloudEvent(BaseModel):") {
		t.Errorf("Python file missing OrderCompletedCloudEvent")
	}
	if !strings.Contains(pyContent, "def create_order_completed_cloud_event(") {
		t.Errorf("Python file missing create_order_completed_cloud_event")
	}

	// 3. Generate Go
	goOutDir := filepath.Join(tempDir, "go_sdk")
	out, err = runCLI(t, "", nil,
		"generate",
		"--event", "com.company.orders.completed.v1",
		"--lang", "go",
		"--out", goOutDir,
		"--server", ts.URL,
	)
	if err != nil {
		t.Fatalf("generate go failed: %v\nOutput: %s", err, out)
	}

	goFile := filepath.Join(goOutDir, "order_completed.go")
	goBytes, err := os.ReadFile(goFile)
	if err != nil {
		t.Fatalf("generated Go file not found at %s: %v", goFile, err)
	}
	goContent := string(goBytes)

	if !strings.Contains(goContent, "type OrderCompletedPayload struct") {
		t.Errorf("Go file missing OrderCompletedPayload")
	}
	if !strings.Contains(goContent, "type OrderCompletedCloudEvent struct") {
		t.Errorf("Go file missing OrderCompletedCloudEvent")
	}
	if !strings.Contains(goContent, "func CreateOrderCompletedCloudEvent(") {
		t.Errorf("Go file missing CreateOrderCompletedCloudEvent")
	}

	// Verify Go syntax validity
	fset := token.NewFileSet()
	_, parseErr := parser.ParseFile(fset, goFile, goContent, parser.AllErrors)
	if parseErr != nil {
		t.Fatalf("generated Go file has syntax errors: %v", parseErr)
	}
}

func TestCLIGenerateFromProjectConfig(t *testing.T) {
	ts, _ := setupTestServer(t)
	defer ts.Close()

	workDir := t.TempDir()

	// Write .kalyx.yaml in working directory
	projectYAML := fmt.Sprintf(`server: %s
lang: typescript
out: ./events_gen
events:
  - com.company.orders.completed.v1
`, ts.URL)

	if err := os.WriteFile(filepath.Join(workDir, ".kalyx.yaml"), []byte(projectYAML), 0644); err != nil {
		t.Fatalf("failed to write .kalyx.yaml: %v", err)
	}

	// Run generate without arguments in workDir
	out, err := runCLI(t, workDir, nil, "generate")
	if err != nil {
		t.Fatalf("kalyxctl generate with .kalyx.yaml failed: %v\nOutput: %s", err, out)
	}

	if !strings.Contains(out, "com.company.orders.completed.v1") {
		t.Errorf("expected event processing in output: %s", out)
	}

	destFile := filepath.Join(workDir, "events_gen", "order_completed.ts")
	if _, err := os.Stat(destFile); err != nil {
		t.Fatalf("expected generated file %s to exist: %v", destFile, err)
	}
}

func TestCLIGenerateDeprecatedEventWarning(t *testing.T) {
	ts, _ := setupTestServer(t)
	defer ts.Close()

	tempDir := t.TempDir()

	out, err := runCLI(t, "", nil,
		"generate",
		"--event", "com.company.users.registered.v1",
		"--lang", "go",
		"--out", filepath.Join(tempDir, "dep_gen"),
		"--server", ts.URL,
	)
	if err != nil {
		t.Fatalf("generate deprecated event failed: %v\nOutput: %s", err, out)
	}

	// Verify RFC 8594 warning printed to terminal
	if !strings.Contains(out, "WARNING [RFC 8594]") || !strings.Contains(out, "DEPRECATED") {
		t.Errorf("expected RFC 8594 warning in output, got:\n%s", out)
	}

	// Verify file was still generated
	genFile := filepath.Join(tempDir, "dep_gen", "user_registered.go")
	content, err := os.ReadFile(genFile)
	if err != nil {
		t.Fatalf("expected file %s to exist: %v", genFile, err)
	}

	fset := token.NewFileSet()
	if _, parseErr := parser.ParseFile(fset, genFile, content, parser.AllErrors); parseErr != nil {
		t.Fatalf("generated deprecated Go file has syntax errors: %v", parseErr)
	}
}
