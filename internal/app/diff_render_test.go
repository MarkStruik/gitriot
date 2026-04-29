package app

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"gitriot/internal/git"
	"gitriot/internal/theme"
	"gitriot/internal/ui"
)

func TestRenderDiffBodyKeepsFixedHeightAndWidthForScrolledChangedRows(t *testing.T) {
	modelPath := filepath.Base(filepath.Join("internal", "app", "model.go"))
	raw, err := os.ReadFile(modelPath)
	if err != nil {
		t.Fatalf("read %s: %v", modelPath, err)
	}

	content := string(raw)
	rawLines := strings.Split(content, "\n")
	highlightedLines := strings.Split(ui.HighlightForPath(modelPath, content, theme.Default.Colors), "\n")
	decor := map[int]git.LineDecoration{
		108: {Added: true},
		114: {Added: true},
		122: {Added: true},
	}

	m := Model{colors: theme.Default.Colors}
	m.diff.Width = 120
	m.diff.Height = 18
	m.diffXOffset = 20
	m.diffRows = m.buildNumberedDiffRows(rawLines, highlightedLines, nil, decor)
	m.diff.SetYOffset(104)

	body := m.renderDiffBody(121, 18, "")
	lines := strings.Split(body, "\n")
	if len(lines) != 18 {
		t.Fatalf("expected 18 visible lines, got %d", len(lines))
	}

	for i, line := range lines {
		if width := ansi.StringWidth(line); width != 121 {
			t.Fatalf("line %d rendered width = %d, want 121\n%q", i, width, ansi.Strip(line))
		}
	}
}

func TestRenderDiffBodyForModelFixtureHasExpectedVisibleRows(t *testing.T) {
	modelPath := filepath.Base(filepath.Join("internal", "app", "model.go"))
	raw, err := os.ReadFile(modelPath)
	if err != nil {
		t.Fatalf("read %s: %v", modelPath, err)
	}

	content := string(raw)
	decor := map[int]git.LineDecoration{
		108: {Added: true},
		114: {Added: true},
		117: {Added: true},
	}
	ranges := []git.LineRange{{Start: 108, End: 108}, {Start: 114, End: 117}}

	m := Model{colors: theme.Default.Colors}
	m.diff.Width = 150
	m.diff.Height = 22
	m.diffRows = append(buildPlainDiffRows([]string{"Path: root/internal/app/model.go", ""}), m.buildFileDiffRows(modelPath, content, ranges, decor, true, 5)...)
	body := m.renderDiffBody(151, 22, "")
	lines := stripRenderedLines(body)
	if len(lines) != 22 {
		t.Fatalf("expected 22 visible lines, got %d\n%s", len(lines), strings.Join(lines, "\n"))
	}

	assertLineSequence(t, lines, []string{
		"Path: root/internal/app/model.go",
		"",
		"~ ~ ~",
		"103 │",
		"104 │    requestID       int",
		"105 │    lastSelID       string",
		"106 │    activeRef       string",
		"107 │    lastFingerprint string",
		"+ 108 │    diffRows        []diffRow",
		"109 │    diffRawLines    []string",
		"110 │    diffXOffset     int",
		"111 │    diffChangeRows  []int",
		"112 │}",
		"113 │",
		"+ 114 │type diffRowKind int",
		"115 │",
		"116 │const (",
		"+ 117 │    diffRowPlain diffRowKind = iota",
	})

	for i := 1; i < len(lines); i++ {
		prev := strings.TrimSpace(lines[i-1])
		curr := strings.TrimSpace(lines[i])
		if prev != "" && curr == "" && !strings.Contains(lines[i-1], "Path:") {
			t.Fatalf("unexpected anonymous blank row between %q and next content", lines[i-1])
		}
	}

	for _, line := range lines {
		if strings.Contains(line, "WorktreeSta") {
			t.Fatalf("unexpected wrapped fragment in rendered output: %q", line)
		}
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || trimmed == "█" || trimmed == "│" || strings.HasPrefix(trimmed, "~") || strings.HasPrefix(trimmed, "Path:") {
			continue
		}
		fields := strings.Fields(trimmed)
		if len(fields) == 0 {
			continue
		}
		if fields[0] == "+" || fields[0] == "-" {
			fields = fields[1:]
		}
		if len(fields) == 0 {
			continue
		}
		if _, err := strconv.Atoi(fields[0]); err != nil {
			t.Fatalf("expected rendered numbered row, got %q", line)
		}
	}
}

func stripRenderedLines(input string) []string {
	rawLines := strings.Split(input, "\n")
	out := make([]string, 0, len(rawLines))
	for _, line := range rawLines {
		out = append(out, strings.TrimRight(ansi.Strip(line), " "))
	}
	return out
}

func assertLineSequence(t *testing.T, lines []string, expected []string) {
	t.Helper()
	start := -1
	for i := 0; i < len(lines)-len(expected)+1; i++ {
		match := true
		for j := range expected {
			if !strings.Contains(normalizeComparisonLine(lines[i+j]), normalizeComparisonLine(expected[j])) {
				match = false
				break
			}
		}
		if match {
			start = i
			break
		}
	}
	if start == -1 {
		t.Fatalf("expected sequence not found in rendered lines:\n%s", strings.Join(lines, "\n"))
	}
}

func normalizeComparisonLine(input string) string {
	return strings.ReplaceAll(input, "\t", "    ")
}
