package app

import (
	"strings"

	"gitriot/internal/model"
)

type Filters struct {
	ShowStaged    bool
	ShowUnstaged  bool
	ShowUntracked bool
	ShowSubmodule bool
	Query         string
}

func DefaultFilters() Filters {
	return Filters{
		ShowStaged:    true,
		ShowUnstaged:  true,
		ShowUntracked: true,
		ShowSubmodule: true,
	}
}

func ApplyFilters(items []model.ChangeItem, filters Filters) []model.ChangeItem {
	out := make([]model.ChangeItem, 0, len(items))
	needle := strings.ToLower(strings.TrimSpace(filters.Query))

	for _, item := range items {
		if !typeEnabled(item, filters) {
			continue
		}

		if !filters.ShowSubmodule && item.IsSubmodule() {
			continue
		}

		if needle != "" {
			label := strings.ToLower(item.ScopeLabel() + "/" + item.Path)
			if !strings.Contains(label, needle) {
				continue
			}
		}

		out = append(out, item)
	}

	return out
}

func typeEnabled(item model.ChangeItem, filters Filters) bool {
	switch item.Type {
	case model.ChangeTypeStaged:
		return filters.ShowStaged
	case model.ChangeTypeUnstaged:
		return filters.ShowUnstaged
	case model.ChangeTypeUntracked:
		return filters.ShowUntracked
	default:
		return true
	}
}
