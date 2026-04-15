package theme

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadFromFileMergesWithDefaults(t *testing.T) {
	base := t.TempDir()
	path := filepath.Join(base, "custom.yaml")

	raw := []byte("name: custom\ncolors:\n  accent: '#123456'\n  added: '#00AA66'\n")
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatalf("write theme failed: %v", err)
	}

	theme, err := LoadFromFile(path)
	if err != nil {
		t.Fatalf("load theme failed: %v", err)
	}

	if theme.Name != "custom" {
		t.Fatalf("expected custom theme name, got %q", theme.Name)
	}

	if theme.Colors.Accent != "#123456" {
		t.Fatalf("expected accent override, got %q", theme.Colors.Accent)
	}

	if theme.Colors.Fg == "" {
		t.Fatal("expected default fg to remain populated")
	}
}
