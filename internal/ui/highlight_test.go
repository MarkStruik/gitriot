package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"gitriot/internal/theme"
)

func TestHighlightForPathUsesThemeSyntaxColors(t *testing.T) {
	content := "func answer() string { return \"42\" // note\n}\n"
	colorsA := theme.Default.Colors
	colorsA.SyntaxKeyword = "#ff0000"
	colorsA.SyntaxFunc = "#00ff00"
	colorsA.SyntaxString = "#0000ff"
	colorsA.SyntaxComment = "#ffff00"
	colorsA.SyntaxType = "#ff00ff"
	colorsA.SyntaxNumber = "#00ffff"
	colorsA.SyntaxOperator = "#ffffff"
	colorsA.SyntaxPunct = "#808080"

	colorsB := colorsA
	colorsB.SyntaxKeyword = "#0000ff"
	colorsB.SyntaxFunc = "#ff0000"
	colorsB.SyntaxString = "#00ff00"
	colorsB.SyntaxComment = "#ff00ff"
	colorsB.SyntaxType = "#ffff00"
	colorsB.SyntaxNumber = "#808080"
	colorsB.SyntaxOperator = "#00ffff"
	colorsB.SyntaxPunct = "#ffffff"

	highlightedA := HighlightForPath("example.go", content, colorsA)
	highlightedB := HighlightForPath("example.go", content, colorsB)

	if highlightedA == highlightedB {
		t.Fatal("expected different syntax output when theme syntax colors change")
	}
	if !strings.Contains(highlightedA, "\x1b[") || !strings.Contains(highlightedB, "\x1b[") {
		t.Fatal("expected highlighted output to include ANSI color sequences")
	}
	if got := ansi.Strip(highlightedA); got != content {
		t.Fatalf("highlighting changed text content: got %q want %q", got, content)
	}
}

func TestStripBackgroundSGRPreservesForeground(t *testing.T) {
	input := "\x1b[38;5;196;48;5;22mred\x1b[0m \x1b[48;2;1;2;3;38;2;4;5;6mrgb\x1b[49m"
	got := stripBackgroundSGR(input)

	if strings.Contains(got, "48;5") || strings.Contains(got, "48;2") || strings.Contains(got, "\x1b[49m") {
		t.Fatalf("expected background SGR parameters to be stripped, got %q", got)
	}
	if !strings.Contains(got, "38;5;196") || !strings.Contains(got, "38;2;4;5;6") {
		t.Fatalf("expected foreground SGR parameters to be preserved, got %q", got)
	}
}
