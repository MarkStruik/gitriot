package theme

import (
	"fmt"
	"os"

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

var Default = FileTheme{
	Name: "default",
	Colors: Tokens{
		Bg:             "#0d1117",
		PanelLeftBg:    "#161b22",
		PanelRightBg:   "#1f2430",
		PanelActiveBg:  "#2d333b",
		Fg:             "#c9d1d9",
		Muted:          "#8b949e",
		Accent:         "#58a6ff",
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
