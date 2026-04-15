package model

type ChangeType string

const (
	ChangeTypeStaged    ChangeType = "staged"
	ChangeTypeUnstaged  ChangeType = "unstaged"
	ChangeTypeUntracked ChangeType = "untracked"
	ChangeTypeSubmodule ChangeType = "submodule"
)

type ChangeItem struct {
	RepoRoot       string
	Path           string
	Type           ChangeType
	SubmodulePath  string
	StagedStatus   rune
	WorktreeStatus rune
}

func (c ChangeItem) IsSubmodule() bool {
	return c.SubmodulePath != ""
}
