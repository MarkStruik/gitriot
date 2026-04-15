package git

import (
	"testing"

	"gitriot/internal/model"
)

func TestParseStatusPorcelain(t *testing.T) {
	input := "M  staged.txt\n M unstaged.txt\nMM both.txt\n?? new.txt\nR  old.txt -> renamed.txt\n"

	items, err := ParseStatusPorcelain("/repo", "", input)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}

	if len(items) != 6 {
		t.Fatalf("expected 6 items, got %d", len(items))
	}

	assertContains(t, items, "staged.txt", model.ChangeTypeStaged)
	assertContains(t, items, "unstaged.txt", model.ChangeTypeUnstaged)
	assertContains(t, items, "both.txt", model.ChangeTypeStaged)
	assertContains(t, items, "both.txt", model.ChangeTypeUnstaged)
	assertContains(t, items, "new.txt", model.ChangeTypeUntracked)
	assertContains(t, items, "renamed.txt", model.ChangeTypeStaged)
}

func TestParseStatusPorcelainInvalidLine(t *testing.T) {
	_, err := ParseStatusPorcelain("/repo", "", "badline")
	if err == nil {
		t.Fatal("expected parse error")
	}
}

func assertContains(t *testing.T, items []model.ChangeItem, path string, changeType model.ChangeType) {
	t.Helper()

	for _, item := range items {
		if item.Path == path && item.Type == changeType {
			return
		}
	}

	t.Fatalf("missing expected item path=%q type=%q", path, changeType)
}
