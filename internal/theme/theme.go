package theme

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Tokens struct {
	Bg      string `yaml:"bg"`
	Fg      string `yaml:"fg"`
	Muted   string `yaml:"muted"`
	Accent  string `yaml:"accent"`
	Added   string `yaml:"added"`
	Removed string `yaml:"removed"`
	Hunk    string `yaml:"hunk"`
	Border  string `yaml:"border"`
}

type FileTheme struct {
	Name   string `yaml:"name"`
	Colors Tokens `yaml:"colors"`
}

var Default = FileTheme{
	Name: "default",
	Colors: Tokens{
		Bg:      "#111418",
		Fg:      "#E6E9EF",
		Muted:   "#8A93A6",
		Accent:  "#5EA1FF",
		Added:   "#2FBF71",
		Removed: "#E05D5D",
		Hunk:    "#E0B84F",
		Border:  "#2A3140",
	},
}

func LoadFromFile(path string) (FileTheme, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return FileTheme{}, fmt.Errorf("read theme file %q failed: %w", path, err)
	}

	var t FileTheme
	if err := yaml.Unmarshal(raw, &t); err != nil {
		return FileTheme{}, fmt.Errorf("parse theme file %q failed: %w", path, err)
	}

	merged := Default
	if t.Name != "" {
		merged.Name = t.Name
	}
	mergeTokens(&merged.Colors, t.Colors)

	return merged, nil
}

func mergeTokens(dst *Tokens, src Tokens) {
	if src.Bg != "" {
		dst.Bg = src.Bg
	}
	if src.Fg != "" {
		dst.Fg = src.Fg
	}
	if src.Muted != "" {
		dst.Muted = src.Muted
	}
	if src.Accent != "" {
		dst.Accent = src.Accent
	}
	if src.Added != "" {
		dst.Added = src.Added
	}
	if src.Removed != "" {
		dst.Removed = src.Removed
	}
	if src.Hunk != "" {
		dst.Hunk = src.Hunk
	}
	if src.Border != "" {
		dst.Border = src.Border
	}
}
