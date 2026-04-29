package theme

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadFromFileMergesTheme(t *testing.T) {
	path := filepath.Join(t.TempDir(), "theme.yaml")
	raw := []byte("name: custom\ncolors:\n  accent: \"#123456\"\n")
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
		t.Fatal("expected default foreground to remain populated")
	}
}

func TestBuiltinNamesIncludesDefaults(t *testing.T) {
	names := BuiltinNames()
	if len(names) < 4 {
		t.Fatalf("expected multiple built-in themes, got %v", names)
	}
	for _, want := range []string{"default", "forest", "midnight", "violet"} {
		found := false
		for _, got := range names {
			if got == want {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected built-in theme %q in %v", want, names)
		}
	}
}

func TestLookupBuiltin(t *testing.T) {
	builtin, ok := LookupBuiltin("forest")
	if !ok {
		t.Fatal("expected forest built-in theme")
	}
	if builtin.Name != "forest" {
		t.Fatalf("expected forest theme name, got %q", builtin.Name)
	}
	if builtin.Colors.Accent == "" {
		t.Fatal("expected built-in accent color")
	}
	if _, ok := LookupBuiltin("does-not-exist"); ok {
		t.Fatal("unexpected unknown built-in theme lookup success")
	}
}
