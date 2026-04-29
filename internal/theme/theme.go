package theme

import (
	"fmt"
	"os"
	"sort"

	"gopkg.in/yaml.v3"
)

type Tokens struct {
	Bg             string `yaml:"bg"`
	PanelLeftBg    string `yaml:"panel_left_bg"`
	PanelRightBg   string `yaml:"panel_right_bg"`
	PanelActiveBg  string `yaml:"panel_active_bg"`
	Fg             string `yaml:"fg"`
	Muted          string `yaml:"muted"`
	Accent         string `yaml:"accent"`
	Added          string `yaml:"added"`
	Removed        string `yaml:"removed"`
	Modified       string `yaml:"modified"`
	Hunk           string `yaml:"hunk"`
	Border         string `yaml:"border"`
	LineSep        string `yaml:"line_sep"`
	RowAddedBg     string `yaml:"row_added_bg"`
	RowRemovedBg   string `yaml:"row_removed_bg"`
	RowModifiedBg  string `yaml:"row_modified_bg"`
	SyntaxPlain    string `yaml:"syntax_plain"`
	SyntaxKeyword  string `yaml:"syntax_keyword"`
	SyntaxString   string `yaml:"syntax_string"`
	SyntaxComment  string `yaml:"syntax_comment"`
	SyntaxType     string `yaml:"syntax_type"`
	SyntaxFunc     string `yaml:"syntax_func"`
	SyntaxNumber   string `yaml:"syntax_number"`
	SyntaxOperator string `yaml:"syntax_operator"`
	SyntaxPunct    string `yaml:"syntax_punct"`
}

type FileTheme struct {
	Name   string `yaml:"name"`
	Colors Tokens `yaml:"colors"`
}

type NamedTheme struct {
	Name  string
	Theme FileTheme
	Kind  string
}

var Default = FileTheme{
	Name: "default",
	Colors: Tokens{
		Bg:             "#0d1117",
		PanelLeftBg:    "#18202a",
		PanelRightBg:   "#1f2430",
		PanelActiveBg:  "#2d333b",
		Fg:             "#c9d1d9",
		Muted:          "#8b949e",
		Accent:         "#6cb6ff",
		Added:          "#3fb950",
		Removed:        "#f85149",
		Modified:       "#d29922",
		Hunk:           "#d29922",
		Border:         "#30363d",
		LineSep:        "#6e7681",
		RowAddedBg:     "#1a472a",
		RowRemovedBg:   "#4b1d1d",
		RowModifiedBg:  "#4b3a19",
		SyntaxPlain:    "#c9d1d9",
		SyntaxKeyword:  "#ff7b72",
		SyntaxString:   "#a5d6ff",
		SyntaxComment:  "#8b949e",
		SyntaxType:     "#ffa657",
		SyntaxFunc:     "#d2a8ff",
		SyntaxNumber:   "#79c0ff",
		SyntaxOperator: "#ff7b72",
		SyntaxPunct:    "#c9d1d9",
	},
}

var builtins = map[string]FileTheme{
	"default": Default,
	"midnight": {
		Name: "midnight",
		Colors: Tokens{
			Bg:             "#0b1020",
			PanelLeftBg:    "#131a2e",
			PanelRightBg:   "#1a2138",
			PanelActiveBg:  "#24304f",
			Fg:             "#d7def7",
			Muted:          "#8e9ab7",
			Accent:         "#7cb7ff",
			Added:          "#4cc38a",
			Removed:        "#ff7575",
			Modified:       "#f0b35a",
			Hunk:           "#f0b35a",
			Border:         "#2f3a5b",
			LineSep:        "#7080aa",
			RowAddedBg:     "#153827",
			RowRemovedBg:   "#4a1d28",
			RowModifiedBg:  "#4d3820",
			SyntaxPlain:    "#d7def7",
			SyntaxKeyword:  "#ff8f70",
			SyntaxString:   "#9fd3ff",
			SyntaxComment:  "#7f8baa",
			SyntaxType:     "#ffc97c",
			SyntaxFunc:     "#c6a0ff",
			SyntaxNumber:   "#7cdfff",
			SyntaxOperator: "#ff8f70",
			SyntaxPunct:    "#d7def7",
		},
	},
	"forest": {
		Name: "forest",
		Colors: Tokens{
			Bg:             "#0f1512",
			PanelLeftBg:    "#172019",
			PanelRightBg:   "#1c2720",
			PanelActiveBg:  "#2a3a30",
			Fg:             "#d4dfd1",
			Muted:          "#8a9b88",
			Accent:         "#7fcf9a",
			Added:          "#73d98c",
			Removed:        "#ff7d7d",
			Modified:       "#e0c06c",
			Hunk:           "#e0c06c",
			Border:         "#314036",
			LineSep:        "#7d8f7e",
			RowAddedBg:     "#1b4228",
			RowRemovedBg:   "#4a2020",
			RowModifiedBg:  "#4b4120",
			SyntaxPlain:    "#d4dfd1",
			SyntaxKeyword:  "#f38b6b",
			SyntaxString:   "#a7e6b5",
			SyntaxComment:  "#7f9182",
			SyntaxType:     "#f0cb7a",
			SyntaxFunc:     "#9fd7c2",
			SyntaxNumber:   "#8fd3c8",
			SyntaxOperator: "#f38b6b",
			SyntaxPunct:    "#d4dfd1",
		},
	},
	"violet": {
		Name: "violet",
		Colors: Tokens{
			Bg:             "#11111b",
			PanelLeftBg:    "#1a1828",
			PanelRightBg:   "#211f33",
			PanelActiveBg:  "#302c4a",
			Fg:             "#ddd8f8",
			Muted:          "#9a95ba",
			Accent:         "#a78bfa",
			Added:          "#59d499",
			Removed:        "#ff8080",
			Modified:       "#f0c36e",
			Hunk:           "#f0c36e",
			Border:         "#3b3554",
			LineSep:        "#7d77a8",
			RowAddedBg:     "#183d2d",
			RowRemovedBg:   "#4d2028",
			RowModifiedBg:  "#4a3921",
			SyntaxPlain:    "#ddd8f8",
			SyntaxKeyword:  "#ff9f79",
			SyntaxString:   "#b8e1ff",
			SyntaxComment:  "#8f89b0",
			SyntaxType:     "#f6c97f",
			SyntaxFunc:     "#c5a9ff",
			SyntaxNumber:   "#86d1ff",
			SyntaxOperator: "#ff9f79",
			SyntaxPunct:    "#ddd8f8",
		},
	},
}

func BuiltinNames() []string {
	names := make([]string, 0, len(builtins))
	for name := range builtins {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func LookupBuiltin(name string) (FileTheme, bool) {
	t, ok := builtins[name]
	return t, ok
}

func Builtins() []NamedTheme {
	names := BuiltinNames()
	out := make([]NamedTheme, 0, len(names))
	for _, name := range names {
		out = append(out, NamedTheme{Name: name, Theme: builtins[name], Kind: "builtin"})
	}
	return out
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
	if src.PanelLeftBg != "" {
		dst.PanelLeftBg = src.PanelLeftBg
	}
	if src.PanelRightBg != "" {
		dst.PanelRightBg = src.PanelRightBg
	}
	if src.PanelActiveBg != "" {
		dst.PanelActiveBg = src.PanelActiveBg
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
	if src.Modified != "" {
		dst.Modified = src.Modified
	}
	if src.Hunk != "" {
		dst.Hunk = src.Hunk
	}
	if src.Border != "" {
		dst.Border = src.Border
	}
	if src.LineSep != "" {
		dst.LineSep = src.LineSep
	}
	if src.RowAddedBg != "" {
		dst.RowAddedBg = src.RowAddedBg
	}
	if src.RowRemovedBg != "" {
		dst.RowRemovedBg = src.RowRemovedBg
	}
	if src.RowModifiedBg != "" {
		dst.RowModifiedBg = src.RowModifiedBg
	}
	if src.SyntaxPlain != "" {
		dst.SyntaxPlain = src.SyntaxPlain
	}
	if src.SyntaxKeyword != "" {
		dst.SyntaxKeyword = src.SyntaxKeyword
	}
	if src.SyntaxString != "" {
		dst.SyntaxString = src.SyntaxString
	}
	if src.SyntaxComment != "" {
		dst.SyntaxComment = src.SyntaxComment
	}
	if src.SyntaxType != "" {
		dst.SyntaxType = src.SyntaxType
	}
	if src.SyntaxFunc != "" {
		dst.SyntaxFunc = src.SyntaxFunc
	}
	if src.SyntaxNumber != "" {
		dst.SyntaxNumber = src.SyntaxNumber
	}
	if src.SyntaxOperator != "" {
		dst.SyntaxOperator = src.SyntaxOperator
	}
	if src.SyntaxPunct != "" {
		dst.SyntaxPunct = src.SyntaxPunct
	}
}
