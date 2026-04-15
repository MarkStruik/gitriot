package app

import (
	"testing"

	"gitriot/internal/model"
)

func TestApplyFilters(t *testing.T) {
	items := []model.ChangeItem{
		{Path: "a.go", Type: model.ChangeTypeStaged},
		{Path: "b.go", Type: model.ChangeTypeUnstaged},
		{Path: "c.go", Type: model.ChangeTypeUntracked},
		{Path: "sub/d.go", Type: model.ChangeTypeUnstaged, SubmodulePath: "submodule-a"},
	}

	filters := DefaultFilters()
	filters.ShowUnstaged = false
	filters.Query = "submodule-a"

	out := ApplyFilters(items, filters)
	if len(out) != 0 {
		t.Fatalf("expected 0 items when unstaged disabled, got %d", len(out))
	}

	filters.ShowUnstaged = true
	out = ApplyFilters(items, filters)
	if len(out) != 1 {
		t.Fatalf("expected 1 item after enabling unstaged, got %d", len(out))
	}

	if out[0].SubmodulePath != "submodule-a" {
		t.Fatalf("expected submodule item, got %q", out[0].SubmodulePath)
	}
}
