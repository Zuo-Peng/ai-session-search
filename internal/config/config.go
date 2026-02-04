package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

type Config struct {
	ClaudeRoot string `toml:"claude_root"`
	CodexRoot  string `toml:"codex_root"`
	DBPath     string `toml:"db_path"`
}

func Load() (*Config, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	cfg := &Config{
		ClaudeRoot: filepath.Join(home, ".claude", "projects"),
		CodexRoot:  filepath.Join(home, ".codex", "sessions"),
		DBPath:     filepath.Join(home, ".config", "ais", "ais.db"),
	}

	cfgPath := filepath.Join(home, ".config", "ais", "config.toml")
	if _, err := os.Stat(cfgPath); err == nil {
		if _, err := toml.DecodeFile(cfgPath, cfg); err != nil {
			return nil, fmt.Errorf("parse config %s: %w", cfgPath, err)
		}
	}

	// expand ~ in paths
	cfg.ClaudeRoot = expandHome(cfg.ClaudeRoot, home)
	cfg.CodexRoot = expandHome(cfg.CodexRoot, home)
	cfg.DBPath = expandHome(cfg.DBPath, home)

	return cfg, nil
}

func expandHome(path, home string) string {
	if len(path) > 1 && path[0] == '~' && path[1] == '/' {
		return filepath.Join(home, path[2:])
	}
	return path
}
