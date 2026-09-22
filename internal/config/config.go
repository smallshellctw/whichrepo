package config

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const SchemaURL = "https://raw.githubusercontent.com/smallshellctw/whichrepo/main/configs/whichrepo.schema.json"

type ProjectOverride struct {
	Path        string   `yaml:"path,omitempty" json:"path,omitempty"`
	Description string   `yaml:"description,omitempty" json:"description,omitempty"`
	Aliases     []string `yaml:"aliases,omitempty" json:"aliases,omitempty"`
}

type Workspace struct {
	Name    string   `yaml:"name" json:"name"`
	Include []string `yaml:"include" json:"include"`
	Exclude []string `yaml:"exclude" json:"exclude"`
}

type Index struct {
	Database           string   `yaml:"database" json:"database"`
	MaxFilesPerProject int      `yaml:"max_files_per_project" json:"max_files_per_project"`
	MaxBytesPerFile    int64    `yaml:"max_bytes_per_file" json:"max_bytes_per_file"`
	MaxBytesPerProject int64    `yaml:"max_bytes_per_project" json:"max_bytes_per_project"`
	ExtraExtensions    []string `yaml:"extra_extensions" json:"extra_extensions"`
}

type Routing struct {
	TopK            int    `yaml:"top_k" json:"top_k"`
	Provider        string `yaml:"provider" json:"provider"`
	Model           string `yaml:"model" json:"model"`
	ProviderURL     string `yaml:"provider_url" json:"provider_url"`
	ProviderTimeout string `yaml:"provider_timeout" json:"provider_timeout"`
}

type Dashboard struct {
	Listen string `yaml:"listen" json:"listen"`
	Open   bool   `yaml:"open" json:"open"`
}

type Privacy struct {
	AdditionalExcludeDirs    []string `yaml:"additional_exclude_dirs" json:"additional_exclude_dirs"`
	AdditionalSensitiveFiles []string `yaml:"additional_sensitive_files" json:"additional_sensitive_files"`
}

type Config struct {
	Version   int                        `yaml:"version" json:"version"`
	Workspace Workspace                  `yaml:"workspace" json:"workspace"`
	Index     Index                      `yaml:"index" json:"index"`
	Routing   Routing                    `yaml:"routing" json:"routing"`
	Dashboard Dashboard                  `yaml:"dashboard" json:"dashboard"`
	Privacy   Privacy                    `yaml:"privacy" json:"privacy"`
	Projects  map[string]ProjectOverride `yaml:"projects" json:"projects"`
}

func Default(workspaceName string) Config {
	return Config{
		Version: 1,
		Workspace: Workspace{
			Name:    workspaceName,
			Include: []string{},
			Exclude: []string{"archive-*", "tmp-*"},
		},
		Index: Index{
			Database:           "",
			MaxFilesPerProject: 800,
			MaxBytesPerFile:    12 * 1024,
			MaxBytesPerProject: 4 * 1024 * 1024,
			ExtraExtensions:    []string{},
		},
		Routing: Routing{
			TopK:            8,
			Provider:        "local",
			Model:           "",
			ProviderURL:     "",
			ProviderTimeout: "20s",
		},
		Dashboard: Dashboard{Listen: "127.0.0.1:8787", Open: false},
		Privacy: Privacy{
			AdditionalExcludeDirs:    []string{},
			AdditionalSensitiveFiles: []string{},
		},
		Projects: map[string]ProjectOverride{},
	}
}

func Load(path string) (Config, error) {
	workspaceName := ""
	if path != "" {
		workspaceName = filepath.Base(filepath.Dir(path))
	}
	cfg := Default(workspaceName)
	if err := overlay(path, &cfg); err != nil {
		return Config{}, err
	}
	cfg = normalize(cfg)
	return cfg, Validate(cfg)
}

// LoadWorkspace applies defaults, the committed workspace configuration, and
// the optional local override in that order. An explicit path replaces both
// workspace files.
func LoadWorkspace(workspace, explicit string) (Config, error) {
	cfg := Default(filepath.Base(filepath.Clean(workspace)))
	paths := ConfigPaths(workspace, explicit)
	for _, path := range paths {
		if err := overlay(path, &cfg); err != nil {
			return Config{}, err
		}
	}
	cfg = normalize(cfg)
	return cfg, Validate(cfg)
}

func ConfigPaths(workspace, explicit string) []string {
	if strings.TrimSpace(explicit) != "" {
		return []string{explicit}
	}
	return []string{
		filepath.Join(workspace, ".whichrepo.yaml"),
		filepath.Join(workspace, ".whichrepo.local.yaml"),
	}
}

func Render(workspaceName string) ([]byte, error) {
	data, err := yaml.Marshal(Default(workspaceName))
	if err != nil {
		return nil, err
	}
	header := fmt.Sprintf("# yaml-language-server: $schema=%s\n# All optional settings are shown with their defaults.\n# API keys belong in environment variables, never in this file.\n", SchemaURL)
	return append([]byte(header), data...), nil
}

func DefaultDBPath() string {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return filepath.Join(".whichrepo", "index.db")
	}
	return filepath.Join(cacheDir, "whichrepo", "index.db")
}

func ResolveConfig(workspace, explicit string) string {
	if explicit != "" {
		return explicit
	}
	return filepath.Join(workspace, ".whichrepo.yaml")
}

func Validate(cfg Config) error {
	if cfg.Version != 1 {
		return fmt.Errorf("unsupported config version %d", cfg.Version)
	}
	if cfg.Index.MaxFilesPerProject <= 0 || cfg.Index.MaxBytesPerFile <= 0 || cfg.Index.MaxBytesPerProject <= 0 {
		return fmt.Errorf("index limits must be positive")
	}
	if cfg.Routing.TopK < 1 || cfg.Routing.TopK > 20 {
		return fmt.Errorf("routing.top_k must be between 1 and 20")
	}
	switch cfg.Routing.Provider {
	case "local", "jev-vercel", "jev-typesafe", "jev-openrouter":
	default:
		return fmt.Errorf("unsupported routing.provider %q", cfg.Routing.Provider)
	}
	timeout, err := time.ParseDuration(cfg.Routing.ProviderTimeout)
	if err != nil || timeout <= 0 {
		return fmt.Errorf("routing.provider_timeout must be a positive duration")
	}
	host, _, err := net.SplitHostPort(cfg.Dashboard.Listen)
	if err != nil {
		return fmt.Errorf("dashboard.listen: %w", err)
	}
	if host != "127.0.0.1" && host != "localhost" && host != "::1" {
		return fmt.Errorf("dashboard.listen must use a loopback address")
	}
	return nil
}

func overlay(path string, cfg *Config) error {
	if path == "" {
		return nil
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return fmt.Errorf("parse %s: %w", path, err)
	}
	return nil
}

func normalize(cfg Config) Config {
	defaults := Default(cfg.Workspace.Name)
	if cfg.Version == 0 {
		cfg.Version = defaults.Version
	}
	if cfg.Routing.Provider == "" {
		cfg.Routing.Provider = defaults.Routing.Provider
	}
	if cfg.Routing.ProviderTimeout == "" {
		cfg.Routing.ProviderTimeout = defaults.Routing.ProviderTimeout
	}
	if cfg.Dashboard.Listen == "" {
		cfg.Dashboard.Listen = defaults.Dashboard.Listen
	}
	if cfg.Projects == nil {
		cfg.Projects = map[string]ProjectOverride{}
	}
	if cfg.Workspace.Include == nil {
		cfg.Workspace.Include = []string{}
	}
	if cfg.Workspace.Exclude == nil {
		cfg.Workspace.Exclude = defaults.Workspace.Exclude
	}
	if cfg.Index.ExtraExtensions == nil {
		cfg.Index.ExtraExtensions = []string{}
	}
	if cfg.Privacy.AdditionalExcludeDirs == nil {
		cfg.Privacy.AdditionalExcludeDirs = []string{}
	}
	if cfg.Privacy.AdditionalSensitiveFiles == nil {
		cfg.Privacy.AdditionalSensitiveFiles = []string{}
	}
	return cfg
}
