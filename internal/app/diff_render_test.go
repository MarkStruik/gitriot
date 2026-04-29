package app

import (
	"fmt"
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
	rawLines := strings.Split(content, "\n")
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
	if !strings.Contains(lines[0], "Path: root/internal/app/model.go") {
		t.Fatalf("expected path header on first line, got %q", lines[0])
	}
	if !strings.Contains(lines[2], "~ ~ ~") {
		t.Fatalf("expected hunk gap indicator, got %q", lines[2])
	}
	markerByLine := map[int]string{108: "+", 114: "+", 117: "+"}
	for idx, lineNo := range []int{103, 104, 105, 106, 107, 108, 109, 110, 111, 112, 113, 114, 115, 116, 117, 118, 119, 120, 121} {
		line := normalizeComparisonLine(lines[idx+3])
		wantPrefix := fmt.Sprintf("%d │", lineNo)
		if marker, ok := markerByLine[lineNo]; ok {
			wantPrefix = fmt.Sprintf("%s %d │", marker, lineNo)
		}
		if !strings.Contains(line, wantPrefix) {
			t.Fatalf("expected line %d to contain %q, got %q", lineNo, wantPrefix, line)
		}
		rawText := strings.TrimLeft(rawLines[lineNo-1], "\t ")
		if rawText != "" && !strings.Contains(line, normalizeComparisonLine(rawText)) {
			t.Fatalf("expected line %d to contain source text %q, got %q", lineNo, rawText, line)
		}
	}

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

func TestRenderVisibleDiffRowUsesThemeBackgroundForHighlightedCode(t *testing.T) {
	colors := theme.Default.Colors
	colors.PanelRightBg = "#123456"
	m := Model{colors: colors}
	m.diff.Width = 80
	row := diffRow{
		kind:   diffRowCode,
		lineNo: "1",
		code:   "\x1b[38;2;255;0;0mfunc\x1b[0m value",
	}

	rendered := m.renderVisibleDiffRow(row, 80, 1, 75)
	wantBg := ansiBg(colors.PanelRightBg)
	if !strings.Contains(rendered, wantBg) {
		t.Fatalf("expected themed background %q in rendered row %q", wantBg, rendered)
	}
	if strings.Contains(rendered, "\x1b[0m value") {
		t.Fatalf("expected reset inside highlighted code to preserve background, got %q", rendered)
	}
	if !strings.Contains(rendered, "\x1b[39m value") {
		t.Fatalf("expected foreground reset inside highlighted code, got %q", rendered)
	}
}

func TestBuildNumberedDiffRowsDoesNotMarkFollowingContextAsDeleted(t *testing.T) {
	patch := strings.Join([]string{
		"@@ -11,17 +11,12 @@",
		"     <GridLayout AutoExpandColumn=\"Title\" DefaultSortColumn=\"OccurredOnUtc\" DefaultSortDirection=\"Descending\">",
		"         <ColumnProperty PropertyName=\"Title\" DefaultWidth=\"260\"/>",
		"+        <ColumnProperty PropertyName=\"ProcessingStatus\" DefaultWidth=\"140\"/>",
		"+        <ColumnProperty PropertyName=\"ProcessedOnUtc\" DefaultWidth=\"180\"/>",
		"         <ColumnProperty PropertyName=\"Direction\" DefaultWidth=\"120\"/>",
		"-        <ColumnProperty PropertyName=\"MessageKind\" DefaultWidth=\"120\"/>",
		"         <ColumnProperty PropertyName=\"TransactionType\" DefaultWidth=\"100\"/>",
	}, "\n")
	fullContent := strings.Join([]string{
		"",
		"",
		"",
		"",
		"",
		"",
		"",
		"",
		"",
		"",
		"    <GridLayout AutoExpandColumn=\"Title\" DefaultSortColumn=\"OccurredOnUtc\" DefaultSortDirection=\"Descending\">",
		"        <ColumnProperty PropertyName=\"Title\" DefaultWidth=\"260\"/>",
		"        <ColumnProperty PropertyName=\"ProcessingStatus\" DefaultWidth=\"140\"/>",
		"        <ColumnProperty PropertyName=\"ProcessedOnUtc\" DefaultWidth=\"180\"/>",
		"        <ColumnProperty PropertyName=\"Direction\" DefaultWidth=\"120\"/>",
		"        <ColumnProperty PropertyName=\"TransactionType\" DefaultWidth=\"100\"/>",
		"        <ColumnProperty PropertyName=\"GatewayOri\" DefaultWidth=\"140\"/>",
	}, "\n")

	m := Model{colors: theme.Default.Colors}
	rows := m.buildFileDiffRows("view.xml", fullContent, git.ParseChangedLineRangesFromPatch(patch), git.ParseLineDecorationsFromPatch(patch), true, 5)

	var messageKindRow *diffRow
	var transactionTypeRow *diffRow
	for i := range rows {
		if strings.Contains(rows[i].code, "MessageKind") {
			messageKindRow = &rows[i]
		}
		if strings.Contains(rows[i].code, "TransactionType") {
			transactionTypeRow = &rows[i]
		}
	}
	if messageKindRow == nil || messageKindRow.marker != "-" {
		t.Fatalf("expected inserted deleted MessageKind row, got %#v", messageKindRow)
	}
	if transactionTypeRow == nil {
		t.Fatal("expected TransactionType row")
	}
	if transactionTypeRow.marker != " " || transactionTypeRow.codeBg != "" {
		t.Fatalf("expected TransactionType to remain unchanged, got marker=%q codeBg=%q", transactionTypeRow.marker, transactionTypeRow.codeBg)
	}
}

func TestThemePickerKeepsDiffPaneVisible(t *testing.T) {
	themeFile := theme.Default
	m := NewModel(Option{
		RepoPath:  ".",
		Theme:     themeFile,
		ThemeName: "default",
		Themes:    theme.Builtins(),
		SaveTheme: nil,
	})
	m.ready = true
	m.width = 120
	m.height = 30
	m.diffRows = buildPlainDiffRows([]string{"Path: visible.js", "", "visible-diff-line"})
	m.diffRawLines = []string{"Path: visible.js", "", "visible-diff-line"}
	m.openThemePicker()
	m.resize()

	view := ansi.Strip(m.View())
	if !strings.Contains(view, "Themes") {
		t.Fatalf("expected theme picker pane in view, got %q", view)
	}
	if !strings.Contains(view, "visible-diff-line") {
		t.Fatalf("expected diff pane to remain visible while picking a theme, got %q", view)
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
func normalizeComparisonLine(input string) string {
	return strings.ReplaceAll(input, "\t", "    ")
}
