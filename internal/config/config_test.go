package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadMissingConfigReturnsDefaults(t *testing.T) {
	base := t.TempDir()
	paths := Paths{
		HomeConfigDir: base,
		ConfigFile:    filepath.Join(base, "config.yaml"),
		ThemesDir:     filepath.Join(base, "themes"),
	}

	cfg, err := Load(paths)
	if err != nil {
		t.Fatalf("expected no error for missing config: %v", err)
	}

	if cfg.Theme != "default" {
		t.Fatalf("expected default theme, got %q", cfg.Theme)
	}
}

func TestLoadParsesConfig(t *testing.T) {
	base := t.TempDir()
	paths := Paths{
		HomeConfigDir: base,
		ConfigFile:    filepath.Join(base, "config.yaml"),
		ThemesDir:     filepath.Join(base, "themes"),
	}

	raw := []byte("theme: ocean\ntheme_file: C:/themes/ocean.yaml\n")
	if err := os.WriteFile(paths.ConfigFile, raw, 0o644); err != nil {
		t.Fatalf("write config failed: %v", err)
	}

	cfg, err := Load(paths)
	if err != nil {
		t.Fatalf("expected config load success: %v", err)
	}

	if cfg.Theme != "ocean" {
		t.Fatalf("expected theme ocean, got %q", cfg.Theme)
	}

	if cfg.ThemeFile != "C:/themes/ocean.yaml" {
		t.Fatalf("expected explicit theme file, got %q", cfg.ThemeFile)
	}
}

func TestSaveWritesConfig(t *testing.T) {
	base := t.TempDir()
	paths := Paths{
		HomeConfigDir: base,
		ConfigFile:    filepath.Join(base, "config.yaml"),
		ThemesDir:     filepath.Join(base, "themes"),
	}
	if err := Save(paths, Config{Theme: "forest"}); err != nil {
		t.Fatalf("save config failed: %v", err)
	}
	cfg, err := Load(paths)
	if err != nil {
		t.Fatalf("reload config failed: %v", err)
	}
	if cfg.Theme != "forest" {
		t.Fatalf("expected saved theme forest, got %q", cfg.Theme)
	}
}
