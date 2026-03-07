package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Config struct {
	WorkspaceBaseDir string   `json:"workspace_base_dir"`
	RepoRoots        []string `json:"repo_roots"`
	MaxDepth         int      `json:"max_depth"`
	CacheAgeSeconds  int      `json:"cache_age_seconds"`
}

func defaults() Config {
	home := os.Getenv("HOME")
	return Config{
		WorkspaceBaseDir: filepath.Join(home, "workspaces"),
		RepoRoots: []string{
			filepath.Join(home, "work"),
			filepath.Join(home, "StudioProjects"),
		},
		MaxDepth:        3,
		CacheAgeSeconds: 3600,
	}
}

func configDir() string {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		base = filepath.Join(os.Getenv("HOME"), ".config")
	}
	return filepath.Join(base, "space")
}

func ConfigFilePath() string {
	return filepath.Join(configDir(), "config.json")
}

func CacheFilePath() string {
	return filepath.Join(configDir(), "repos.cache")
}

func EnsureAndLoad() (Config, error) {
	dir := configDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return Config{}, fmt.Errorf("cannot create config directory %q: %w", dir, err)
	}
	if err := SaveDefaultsIfMissing(); err != nil {
		return Config{}, fmt.Errorf("cannot write default config to %q: %w", ConfigFilePath(), err)
	}
	cfg, err := Load()
	if err != nil {
		return Config{}, err
	}
	if err := os.MkdirAll(cfg.WorkspaceBaseDir, 0o755); err != nil {
		return Config{}, fmt.Errorf("cannot create workspace base dir %q: %w", cfg.WorkspaceBaseDir, err)
	}
	return cfg, nil
}

func SaveDefaultsIfMissing() error {
	path := ConfigFilePath()
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	return Save(defaults())
}

func Load() (Config, error) {
	path := ConfigFilePath()
	cfg := defaults()
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return cfg, fmt.Errorf("config file not found at %q — run `space config init` to create one", path)
		}
		return cfg, fmt.Errorf("cannot read config file %q: %w", path, err)
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("config file %q is not valid JSON: %w", path, err)
	}
	cfg.WorkspaceBaseDir = expandPath(cfg.WorkspaceBaseDir)
	for i, root := range cfg.RepoRoots {
		cfg.RepoRoots[i] = expandPath(root)
	}
	if cfg.MaxDepth < 1 {
		cfg.MaxDepth = 3
	}
	if cfg.CacheAgeSeconds < 1 {
		cfg.CacheAgeSeconds = 3600
	}
	return cfg, nil
}

func Save(cfg Config) error {
	cfg.WorkspaceBaseDir = expandPath(cfg.WorkspaceBaseDir)
	for i, root := range cfg.RepoRoots {
		cfg.RepoRoots[i] = expandPath(root)
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(ConfigFilePath(), data, 0o644)
}

func expandPath(path string) string {
	if path == "" {
		return path
	}
	if path == "~" {
		return os.Getenv("HOME")
	}
	if strings.HasPrefix(path, "~/") {
		return filepath.Join(os.Getenv("HOME"), strings.TrimPrefix(path, "~/"))
	}
	return path
}
