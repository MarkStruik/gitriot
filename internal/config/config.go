package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Theme     string `yaml:"theme"`
	ThemeFile string `yaml:"theme_file"`
}

type Paths struct {
	HomeConfigDir string
	ConfigFile    string
	ThemesDir     string
}

func ResolvePaths() (Paths, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Paths{}, fmt.Errorf("resolve user home failed: %w", err)
	}

	base := filepath.Join(home, ".gitriot")
	return Paths{
		HomeConfigDir: base,
		ConfigFile:    filepath.Join(base, "config.yaml"),
		ThemesDir:     filepath.Join(base, "themes"),
	}, nil
}

func Load(paths Paths) (Config, error) {
	cfg := DefaultConfig()

	raw, err := os.ReadFile(paths.ConfigFile)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}

		return Config{}, fmt.Errorf("read config %q failed: %w", paths.ConfigFile, err)
	}

	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config %q failed: %w", paths.ConfigFile, err)
	}

	if cfg.Theme == "" {
		cfg.Theme = "default"
	}

	return cfg, nil
}

func DefaultConfig() Config {
	return Config{Theme: "default"}
}

func ThemePath(paths Paths, themeName string) string {
	return filepath.Join(paths.ThemesDir, themeName+".yaml")
}
