// Package baseconfig is the YAML-backed runtime configuration layer for
// the OCS Testbench.
//
// The package exposes two struct shapes:
//
//   - BaseConfig — cross-cutting fields shared by every binary in the
//     OCS family (app name, database URL, logging block, metrics
//     block).
//
//   - Config — the testbench's full configuration. Embeds BaseConfig
//     and adds binary-specific fields: HTTP server settings, the
//     embedded-frontend block, the headless-mode flag, and the
//     Diameter peer-list slot.
//
// Load reads the configuration from a YAML file, applies sensible
// defaults for unset fields, and validates the result. The CONFIG_FILE
// environment variable, when set, overrides the path argument so
// containerised deployments can point at a mounted config without code
// changes.
//
// The YAML schema is consumed only by this binary; it is therefore not
// a "contract" in the framework's sense and may be extended in later
// Features without a decision-issue. Operators are pointed at
// cmd/ocs-testbench/config.yaml as the canonical example.
package baseconfig

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// ConfigFileEnv is the environment variable that, when set to a
// non-empty value, overrides the path argument to Load. Matches the
// charging-domain convention so operators can use the same muscle
// memory across binaries.
const ConfigFileEnv = "CONFIG_FILE"

// LogConfig is the structured-logging block shared across the family.
// Format selects the slog handler ("json" | "text"); Level is the
// slog level name ("debug" | "info" | "warn" | "error").
type LogConfig struct {
	Format string `yaml:"format"`
	Level  string `yaml:"level"`
}

// MetricsConfig is the Prometheus metrics-server block. When Enabled
// is false the metrics server is not started; Addr and Path are
// ignored.
type MetricsConfig struct {
	Enabled bool   `yaml:"enabled"`
	Addr    string `yaml:"addr"`
	Path    string `yaml:"path"`
}

// BaseConfig is the cross-cutting configuration shared by every binary
// in the OCS family. Binary-specific fields (HTTP server, frontend,
// peer-list) are declared on the embedding type.
type BaseConfig struct {
	AppName     string        `yaml:"app_name"`
	DatabaseURL string        `yaml:"database_url"`
	Logging     LogConfig     `yaml:"logging"`
	Metrics     MetricsConfig `yaml:"metrics"`
}

// ServerConfig is the HTTP-server block on Config. Timeouts use Go's
// time.Duration and accept the standard YAML duration syntax (e.g.
// "10s", "1m", "500ms").
type ServerConfig struct {
	Addr         string        `yaml:"addr"`
	ReadTimeout  time.Duration `yaml:"read_timeout"`
	WriteTimeout time.Duration `yaml:"write_timeout"`
	IdleTimeout  time.Duration `yaml:"idle_timeout"`
}

// FrontendConfig is the embedded-frontend block on Config.
// AutoOpenBrowser and the top-level Headless flag together gate the
// auto-open helper invoked during start-up.
type FrontendConfig struct {
	AutoOpenBrowser    bool   `yaml:"auto_open_browser"`
	EmbeddedAssetsPath string `yaml:"embedded_assets_path"`
}

// Peer is one entry in the Diameter peer-list configuration. This
// Feature scopes only an empty []Peer{} placeholder so the config
// schema is in place and downstream code can take a typed slice; the
// concrete fields will be populated by later Features that wire the
// Diameter stack itself.
type Peer struct {
	Name string `yaml:"name"`
	Host string `yaml:"host"`
}

// AIConfig is the AI assistant configuration block. When Endpoint is
// empty the AI assistant feature is disabled — the chat endpoint returns
// 503 and no LLM client is created.
type AIConfig struct {
	Endpoint string `yaml:"endpoint" json:"endpoint"` // e.g. "https://api.openai.com"
	Model    string `yaml:"model"    json:"model"`    // e.g. "gpt-4o"
	APIKey   string `yaml:"api_key"  json:"apiKey"`   // plaintext; operators use env-var injection
	// Thinking enables the model's extended reasoning / chain-of-thought mode
	// (e.g. Qwen3 <think> blocks). Defaults to true; set false to skip
	// reasoning for faster, cheaper responses on straightforward queries.
	Thinking bool `yaml:"thinking" json:"thinking"`
}

// Config is the testbench's full runtime configuration. It embeds the
// BaseConfig cross-cutting fields and adds the HTTP server, frontend,
// peer-list, and the headless-mode flag.
type Config struct {
	BaseConfig `yaml:",inline"`
	Server     ServerConfig   `yaml:"server"`
	Frontend   FrontendConfig `yaml:"frontend"`
	Headless   bool           `yaml:"headless"`
	Peers      []Peer         `yaml:"peers"`
	AI         AIConfig       `yaml:"ai"`
}

// Load reads a YAML configuration file, applies defaults, validates
// the result, and returns a populated *Config.
//
// If the CONFIG_FILE environment variable is set to a non-empty value
// it overrides the path argument. This matches the charging-domain
// convention and lets containerised deployments point at a mounted
// config without code changes.
//
// Errors are returned for: missing path with no env override, missing
// or unreadable file, malformed YAML, and missing/invalid mandatory
// fields. Defaults applied here are never permitted to override values
// supplied by the YAML.
func Load(path string) (*Config, error) {
	if env := os.Getenv(ConfigFileEnv); env != "" {
		path = env
	}
	if path == "" {
		return nil, errors.New("baseconfig: no config path supplied and CONFIG_FILE not set")
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("baseconfig: read %q: %w", path, err)
	}

	cfg := &Config{}
	if err := yaml.Unmarshal(raw, cfg); err != nil {
		return nil, fmt.Errorf("baseconfig: parse %q: %w", path, err)
	}

	cfg.applyDefaults()

	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("baseconfig: validate %q: %w", path, err)
	}

	// Normalise an unset peer-list to an empty (non-nil) slice so
	// callers can range over it without a nil check.
	if cfg.Peers == nil {
		cfg.Peers = []Peer{}
	}

	return cfg, nil
}

// aiOverridePath returns the path of the AI settings override file that sits
// alongside the main config file. Respects CONFIG_FILE env var so the override
// lands in the same directory as the config that was actually loaded.
func aiOverridePath(configPath string) string {
	if env := os.Getenv(ConfigFileEnv); env != "" {
		configPath = env
	}
	return filepath.Join(filepath.Dir(configPath), "ai-settings.json")
}

// LoadAIOverride reads the AI settings override file written by SaveAIOverride.
// Returns (zero, false, nil) when the file does not exist.
func LoadAIOverride(configPath string) (AIConfig, bool, error) {
	data, err := os.ReadFile(aiOverridePath(configPath))
	if os.IsNotExist(err) {
		return AIConfig{}, false, nil
	}
	if err != nil {
		return AIConfig{}, false, fmt.Errorf("baseconfig: read ai override: %w", err)
	}
	var cfg AIConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return AIConfig{}, false, fmt.Errorf("baseconfig: parse ai override: %w", err)
	}
	return cfg, true, nil
}

// SaveAIOverride writes the AI config to the override file so it is loaded on
// the next restart. The API key is stored in plaintext — the file should be
// treated with the same care as config.yaml.
func SaveAIOverride(configPath string, cfg AIConfig) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("baseconfig: marshal ai override: %w", err)
	}
	if err := os.WriteFile(aiOverridePath(configPath), data, 0o600); err != nil {
		return fmt.Errorf("baseconfig: write ai override: %w", err)
	}
	return nil
}

// applyDefaults populates unset fields with sensible local-dev values.
// Defaults never override values set in YAML — every branch checks
// the zero value before assigning.
func (c *Config) applyDefaults() {
	if c.AppName == "" {
		c.AppName = "ocs-testbench"
	}
	if c.Logging.Format == "" {
		c.Logging.Format = "text"
	}
	if c.Logging.Level == "" {
		c.Logging.Level = "info"
	}
	if c.Metrics.Addr == "" {
		c.Metrics.Addr = ":9090"
	}
	if c.Metrics.Path == "" {
		c.Metrics.Path = "/metrics"
	}
	if c.Server.ReadTimeout == 0 {
		c.Server.ReadTimeout = 10 * time.Second
	}
	// WriteTimeout intentionally has no default — a zero value means no
	// timeout, which is correct for a server that serves long-lived SSE
	// streams. Operators who need a hard limit can set write_timeout in
	// config.yaml; setting it non-zero will kill streaming connections.
	if c.Server.IdleTimeout == 0 {
		c.Server.IdleTimeout = 60 * time.Second
	}
	if c.Frontend.EmbeddedAssetsPath == "" {
		c.Frontend.EmbeddedAssetsPath = "web/dist"
	}
	// AI config defaults — only applied when an endpoint is set so that
	// deployments without AI config are not affected.
	if c.AI.Endpoint != "" && c.AI.Model == "" {
		c.AI.Model = "gpt-4o"
	}
	// Thinking defaults to false (disabled) — fast responses are preferred
	// for tool-dispatch workloads. Operators can enable it via Settings or
	// by setting `thinking: true` in config.yaml.
}

// validate checks the required fields and the enum-shaped fields. Returns
// a single error aggregating every problem so the operator sees the
// full set of issues in one go rather than having to fix-and-rerun.
func (c *Config) validate() error {
	var problems []string

	if strings.TrimSpace(c.DatabaseURL) == "" {
		problems = append(problems, "database_url must be set")
	}
	if strings.TrimSpace(c.Server.Addr) == "" {
		problems = append(problems, "server.addr must be set")
	}
	switch c.Logging.Format {
	case "json", "text":
	default:
		problems = append(problems, fmt.Sprintf("logging.format %q is not one of: json, text", c.Logging.Format))
	}
	switch c.Logging.Level {
	case "debug", "info", "warn", "error":
	default:
		problems = append(problems, fmt.Sprintf("logging.level %q is not one of: debug, info, warn, error", c.Logging.Level))
	}

	if len(problems) > 0 {
		return errors.New(strings.Join(problems, "; "))
	}
	return nil
}
