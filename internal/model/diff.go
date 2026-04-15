package model

type DiffMode string

const (
	DiffModeUnstaged DiffMode = "unstaged"
	DiffModeStaged   DiffMode = "staged"
)

type DiffRequest struct {
	RepoRoot      string
	Path          string
	SubmodulePath string
	Mode          DiffMode
}

type DiffResult struct {
	Request  DiffRequest
	Patch    string
	IsBinary bool
	Empty    bool
}
