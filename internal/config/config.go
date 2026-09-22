package config

import (
	"errors"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type ProjectOverride struct {
	Path        string   `yaml:"path,omitempty"`
	Description string   `yaml:"description,omitempty"`
	Aliases     []string `yaml:"aliases,omitempty"`
}

type Workspace struct {
	Name    string   `yaml:"name,omitempty"`
	Include []string `yaml:"include,omitempty"`
	Exclude []string `yaml:"exclude,omitempty"`
}

type Config struct {
	Version   int                        `yaml:"version"`
	Workspace Workspace                  `yaml:"workspace,omitempty"`
	Projects  map[string]ProjectOverride `yaml:"projects,omitempty"`
}

func Load(path string) (Config, error) {
	cfg := Config{Version: 1, Projects: map[string]ProjectOverride{}}
	if path == "" {
		return cfg, nil
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return cfg, nil
	}
	if err != nil {
		return Config{}, err
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, err
	}
	if cfg.Projects == nil {
		cfg.Projects = map[string]ProjectOverride{}
	}
	return cfg, nil
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
	local := filepath.Join(workspace, ".whichrepo.local.yaml")
	if _, err := os.Stat(local); err == nil {
		return local
	}
	return filepath.Join(workspace, ".whichrepo.yaml")
}
