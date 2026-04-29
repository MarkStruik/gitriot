package app

import (
	"strings"
	"testing"

	"gitriot/internal/git"
	"gitriot/internal/model"
	"gitriot/internal/theme"
)

func TestBuildChangeTreeRowsCompactsSingleChildFolders(t *testing.T) {
	items := []model.ChangeItem{
		{Path: "Presentation/WebApp/wwwroot/Client/app/src/common/controls/map/favorites.js", Type: model.ChangeTypeUnstaged, WorktreeStatus: 'M'},
	}

	rows := buildChangeTreeRows(items, nil, map[string]bool{}, "GitRiot")
	texts := make([]string, 0, len(rows))
	for _, row := range rows {
		texts = append(texts, row.text)
	}
	joined := strings.Join(texts, "\n")

	if !strings.Contains(joined, "Presentation/WebApp/wwwroot/Client/app/src/common/controls/map") {
		t.Fatalf("expected compacted folder chain, got:\n%s", joined)
	}
	if strings.Contains(joined, "\n  ▾ WebApp") {
		t.Fatalf("expected WebApp to be compacted into parent row, got:\n%s", joined)
	}
	if !strings.Contains(joined, "favorites.js") {
		t.Fatalf("expected file row to remain visible, got:\n%s", joined)
	}
}

func TestRenderNumberedLinesHighlightsPartialChangedRows(t *testing.T) {
	m := Model{colors: theme.Default.Colors}
	decor := map[int]git.LineDecoration{
		1: {Added: true},
		2: {Deleted: true, DeletedLines: []string{"old"}},
	}

	rows := m.buildNumberedDiffRows([]string{"new", "kept"}, []string{"new", "kept"}, nil, decor)
	if rows[0].codeBg != ansiBg(theme.Default.Colors.RowAddedBg) {
		t.Fatalf("expected mixed edit added row background, got %q", rows[0].codeBg)
	}
	if rows[1].codeBg != ansiBg(theme.Default.Colors.RowRemovedBg) {
		t.Fatalf("expected mixed edit removed row background, got %q", rows[1].codeBg)
	}
	if rows[0].gutterBg != ansiBg(darkenHexColor(theme.Default.Colors.RowAddedBg, 0.72)) {
		t.Fatalf("expected derived darker added gutter background, got %q", rows[0].gutterBg)
	}
	if rows[1].gutterBg != ansiBg(darkenHexColor(theme.Default.Colors.RowRemovedBg, 0.72)) {
		t.Fatalf("expected derived darker deleted gutter background, got %q", rows[1].gutterBg)
	}

	partialAddOnly := m.buildNumberedDiffRows([]string{"new", "stable"}, []string{"new", "stable"}, nil, map[int]git.LineDecoration{1: {Added: true}})
	if partialAddOnly[0].codeBg != ansiBg(theme.Default.Colors.RowAddedBg) {
		t.Fatalf("expected add row background for partial add-only file, got %q", partialAddOnly[0].codeBg)
	}

	addedOnly := m.buildNumberedDiffRows([]string{"new"}, []string{"new"}, nil, map[int]git.LineDecoration{1: {Added: true}})
	if addedOnly[0].codeBg != "" {
		t.Fatalf("did not expect full-row background for add-only file, got %q", addedOnly[0].codeBg)
	}
}
