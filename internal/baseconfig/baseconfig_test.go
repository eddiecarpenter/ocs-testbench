package baseconfig

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// validYAML is a complete, valid configuration that exercises every
// documented field. Reused across happy-path tests.
const validYAML = `
app_name: ocs-testbench
database_url: postgres://ocs:secret@localhost:5432/ocs?sslmode=disable
logging:
  format: json
  level: debug
metrics:
  enabled: true
  addr: ":9091"
  path: /metricz
server:
  addr: ":8080"
  read_timeout: 5s
  write_timeout: 15s
  idle_timeout: 90s
frontend:
  auto_open_browser: true
  embedded_assets_path: web/dist
headless: false
peers:
  - name: gateway
    host: 127.0.0.1
`

func writeFile(t *testing.T, name, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", p, err)
	}
	return p
}

// TestLoad_ValidRoundTrip — happy path: every documented field is
// faithfully unmarshalled and validation succeeds.
func TestLoad_ValidRoundTrip(t *testing.T) {
	t.Setenv(ConfigFileEnv, "")
	p := writeFile(t, "config.yaml", validYAML)

	cfg, err := Load(p)
	if err != nil {
		t.Fatalf("Load: unexpected error: %v", err)
	}

	want := &Config{
		BaseConfig: BaseConfig{
			AppName:     "ocs-testbench",
			DatabaseURL: "postgres://ocs:secret@localhost:5432/ocs?sslmode=disable",
			Logging:     LogConfig{Format: "json", Level: "debug"},
			Metrics:     MetricsConfig{Enabled: true, Addr: ":9091", Path: "/metricz"},
		},
		Server: ServerConfig{
			Addr:         ":8080",
			ReadTimeout:  5 * time.Second,
			WriteTimeout: 15 * time.Second,
			IdleTimeout:  90 * time.Second,
		},
		Frontend: FrontendConfig{AutoOpenBrowser: true, EmbeddedAssetsPath: "web/dist"},
		Headless: false,
		Peers:    []Peer{{Name: "gateway", Host: "127.0.0.1"}},
	}
	if cfg.AppName != want.AppName ||
		cfg.DatabaseURL != want.DatabaseURL ||
		cfg.Logging != want.Logging ||
		cfg.Metrics != want.Metrics ||
		cfg.Server != want.Server ||
		cfg.Frontend != want.Frontend ||
		cfg.Headless != want.Headless ||
		len(cfg.Peers) != 1 || cfg.Peers[0] != want.Peers[0] {
		t.Fatalf("Load: unexpected config:\n got = %+v\nwant = %+v", cfg, want)
	}
}

// TestLoad_DefaultsApply — fields omitted from YAML are populated by
// applyDefaults; values supplied by YAML are not clobbered.
func TestLoad_DefaultsApply(t *testing.T) {
	t.Setenv(ConfigFileEnv, "")
	minimal := `
database_url: postgres://x
server:
  addr: ":80"
`
	p := writeFile(t, "minimal.yaml", minimal)

	cfg, err := Load(p)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.AppName != "ocs-testbench" {
		t.Errorf("AppName default: got %q, want %q", cfg.AppName, "ocs-testbench")
	}
	if cfg.Logging.Format != "text" {
		t.Errorf("Logging.Format default: got %q, want %q", cfg.Logging.Format, "text")
	}
	if cfg.Logging.Level != "info" {
		t.Errorf("Logging.Level default: got %q, want %q", cfg.Logging.Level, "info")
	}
	if cfg.Metrics.Addr != ":9090" {
		t.Errorf("Metrics.Addr default: got %q, want %q", cfg.Metrics.Addr, ":9090")
	}
	if cfg.Metrics.Path != "/metrics" {
		t.Errorf("Metrics.Path default: got %q, want %q", cfg.Metrics.Path, "/metrics")
	}
	if cfg.Server.ReadTimeout != 10*time.Second {
		t.Errorf("Server.ReadTimeout default: got %v, want 10s", cfg.Server.ReadTimeout)
	}
	if cfg.Server.WriteTimeout != 30*time.Second {
		t.Errorf("Server.WriteTimeout default: got %v, want 30s", cfg.Server.WriteTimeout)
	}
	if cfg.Server.IdleTimeout != 60*time.Second {
		t.Errorf("Server.IdleTimeout default: got %v, want 60s", cfg.Server.IdleTimeout)
	}
	if cfg.Frontend.EmbeddedAssetsPath != "web/dist" {
		t.Errorf("Frontend.EmbeddedAssetsPath default: got %q, want %q", cfg.Frontend.EmbeddedAssetsPath, "web/dist")
	}
	if cfg.Peers == nil {
		t.Error("Peers should be normalised to non-nil empty slice")
	}
	if len(cfg.Peers) != 0 {
		t.Errorf("Peers length: got %d, want 0", len(cfg.Peers))
	}
}

// TestLoad_MissingDatabaseURL — required-field validation returns a
// clear error citing the missing field.
func TestLoad_MissingDatabaseURL(t *testing.T) {
	t.Setenv(ConfigFileEnv, "")
	missing := `
server:
  addr: ":80"
`
	p := writeFile(t, "missing-db.yaml", missing)

	_, err := Load(p)
	if err == nil {
		t.Fatal("Load: expected error for missing database_url; got nil")
	}
	if !strings.Contains(err.Error(), "database_url") {
		t.Errorf("Load: error should mention database_url; got %v", err)
	}
}

// TestLoad_MissingServerAddr — the second required field, exercised
// independently of the first.
func TestLoad_MissingServerAddr(t *testing.T) {
	t.Setenv(ConfigFileEnv, "")
	missing := `
database_url: postgres://x
`
	p := writeFile(t, "missing-addr.yaml", missing)

	_, err := Load(p)
	if err == nil {
		t.Fatal("Load: expected error for missing server.addr; got nil")
	}
	if !strings.Contains(err.Error(), "server.addr") {
		t.Errorf("Load: error should mention server.addr; got %v", err)
	}
}

// TestLoad_InvalidLogFormat — enum-shaped field validation.
func TestLoad_InvalidLogFormat(t *testing.T) {
	t.Setenv(ConfigFileEnv, "")
	bad := `
database_url: postgres://x
server:
  addr: ":80"
logging:
  format: xml
`
	p := writeFile(t, "bad-format.yaml", bad)

	_, err := Load(p)
	if err == nil {
		t.Fatal("Load: expected error for invalid log format; got nil")
	}
	if !strings.Contains(err.Error(), "logging.format") {
		t.Errorf("Load: error should mention logging.format; got %v", err)
	}
}

// TestLoad_InvalidLogLevel — enum-shaped field validation.
func TestLoad_InvalidLogLevel(t *testing.T) {
	t.Setenv(ConfigFileEnv, "")
	bad := `
database_url: postgres://x
server:
  addr: ":80"
logging:
  level: trace
`
	p := writeFile(t, "bad-level.yaml", bad)

	_, err := Load(p)
	if err == nil {
		t.Fatal("Load: expected error for invalid log level; got nil")
	}
	if !strings.Contains(err.Error(), "logging.level") {
		t.Errorf("Load: error should mention logging.level; got %v", err)
	}
}

// TestLoad_AggregatesProblems — multiple problems surface in one
// error so the operator does not need to fix-and-rerun.
func TestLoad_AggregatesProblems(t *testing.T) {
	t.Setenv(ConfigFileEnv, "")
	bad := `
logging:
  format: xml
  level: trace
`
	p := writeFile(t, "many-bad.yaml", bad)

	_, err := Load(p)
	if err == nil {
		t.Fatal("Load: expected error; got nil")
	}
	for _, want := range []string{"database_url", "server.addr", "logging.format", "logging.level"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("Load: aggregated error missing %q: %v", want, err)
		}
	}
}

// TestLoad_MalformedYAML — parse failure surfaces with the path so
// the operator can locate the bad file.
func TestLoad_MalformedYAML(t *testing.T) {
	t.Setenv(ConfigFileEnv, "")
	// Use a structure where keys collide with the typed Config schema —
	// e.g. logging declared as a scalar where the loader expects a
	// mapping. yaml.v3 returns an unmarshal error for this shape.
	p := writeFile(t, "bad.yaml", "logging: not-a-mapping\nserver: also-not-a-mapping\n")

	_, err := Load(p)
	if err == nil {
		t.Fatal("Load: expected parse error; got nil")
	}
	if !strings.Contains(err.Error(), "parse") {
		t.Errorf("Load: error should mention parse failure; got %v", err)
	}
	if !strings.Contains(err.Error(), p) {
		t.Errorf("Load: error should include the offending path %q; got %v", p, err)
	}
}

// TestLoad_FileNotFound — read failure surfaces with the missing path.
func TestLoad_FileNotFound(t *testing.T) {
	t.Setenv(ConfigFileEnv, "")
	missing := filepath.Join(t.TempDir(), "does-not-exist.yaml")

	_, err := Load(missing)
	if err == nil {
		t.Fatal("Load: expected read error; got nil")
	}
	if !errors.Is(err, os.ErrNotExist) {
		t.Errorf("Load: error should wrap os.ErrNotExist; got %v", err)
	}
}

// TestLoad_NoPathNoEnv — empty path with no CONFIG_FILE env returns a
// clear caller-error rather than a misleading file-not-found.
func TestLoad_NoPathNoEnv(t *testing.T) {
	t.Setenv(ConfigFileEnv, "")
	_, err := Load("")
	if err == nil {
		t.Fatal("Load(\"\"): expected error; got nil")
	}
	if !strings.Contains(err.Error(), "no config path") {
		t.Errorf("Load(\"\"): error should mention no path; got %v", err)
	}
}

// TestLoad_ConfigFileEnvOverride — when CONFIG_FILE is set, the
// argument path is ignored and the env-provided path is loaded
// instead.
func TestLoad_ConfigFileEnvOverride(t *testing.T) {
	envPath := writeFile(t, "from-env.yaml", validYAML)
	argPath := writeFile(t, "from-arg.yaml", "this should not be loaded")

	t.Setenv(ConfigFileEnv, envPath)

	cfg, err := Load(argPath)
	if err != nil {
		t.Fatalf("Load: unexpected error with CONFIG_FILE override: %v", err)
	}
	if cfg.DatabaseURL == "" || cfg.Server.Addr == "" {
		t.Fatalf("Load: expected env-provided file to be parsed; got %+v", cfg)
	}
}

// TestLoad_ConfigFileEnvEmpty — empty CONFIG_FILE behaves as if
// unset; the path argument is honoured.
func TestLoad_ConfigFileEnvEmpty(t *testing.T) {
	p := writeFile(t, "from-arg.yaml", validYAML)
	t.Setenv(ConfigFileEnv, "")

	cfg, err := Load(p)
	if err != nil {
		t.Fatalf("Load: unexpected error with empty CONFIG_FILE: %v", err)
	}
	if cfg.DatabaseURL == "" {
		t.Fatalf("Load: expected arg path to be honoured")
	}
}

// TestLoad_DefaultsDoNotOverride — values supplied by YAML are not
// clobbered by applyDefaults. Edge case: a string field set to a
// non-default value, plus a duration set to a non-zero value.
func TestLoad_DefaultsDoNotOverride(t *testing.T) {
	t.Setenv(ConfigFileEnv, "")
	custom := `
app_name: my-binary
database_url: postgres://x
logging:
  format: json
  level: warn
metrics:
  addr: ":7777"
  path: /m
server:
  addr: ":80"
  read_timeout: 1s
  write_timeout: 1s
  idle_timeout: 1s
frontend:
  embedded_assets_path: custom/path
`
	p := writeFile(t, "custom.yaml", custom)

	cfg, err := Load(p)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.AppName != "my-binary" {
		t.Errorf("AppName: defaults overrode YAML; got %q", cfg.AppName)
	}
	if cfg.Logging.Format != "json" || cfg.Logging.Level != "warn" {
		t.Errorf("Logging: defaults overrode YAML; got %+v", cfg.Logging)
	}
	if cfg.Metrics.Addr != ":7777" || cfg.Metrics.Path != "/m" {
		t.Errorf("Metrics: defaults overrode YAML; got %+v", cfg.Metrics)
	}
	if cfg.Server.ReadTimeout != time.Second || cfg.Server.WriteTimeout != time.Second || cfg.Server.IdleTimeout != time.Second {
		t.Errorf("Server timeouts: defaults overrode YAML; got %+v", cfg.Server)
	}
	if cfg.Frontend.EmbeddedAssetsPath != "custom/path" {
		t.Errorf("Frontend.EmbeddedAssetsPath: defaults overrode YAML; got %q", cfg.Frontend.EmbeddedAssetsPath)
	}
}

// TestLoad_DefaultConfigYAML — the in-tree cmd/ocs-testbench/config.yaml
// must itself round-trip through Load. Catches drift between the example
// config and the schema.
func TestLoad_DefaultConfigYAML(t *testing.T) {
	t.Setenv(ConfigFileEnv, "")
	// Resolve relative to the test file location: the test runs from
	// internal/baseconfig/, so two levels up is the repo root.
	repoRoot, err := filepath.Abs("../..")
	if err != nil {
		t.Fatalf("abs: %v", err)
	}
	cfgPath := filepath.Join(repoRoot, "cmd", "ocs-testbench", "config.yaml")

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load(%q): %v", cfgPath, err)
	}
	if cfg.DatabaseURL == "" || cfg.Server.Addr == "" {
		t.Fatalf("Load: default YAML missing mandatory fields; got %+v", cfg)
	}
}
