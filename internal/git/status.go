package git

import (
	"context"
	"fmt"
	"strings"

	"gitriot/internal/model"
)

type StatusIndexer struct {
	runner Runner
}

func NewStatusIndexer(runner Runner) *StatusIndexer {
	return &StatusIndexer{runner: runner}
}

func (s *StatusIndexer) Index(ctx context.Context, repoRoot string) ([]model.ChangeItem, error) {
	out, err := s.runner.Run(ctx, repoRoot, "-c", "core.quotepath=false", "status", "--porcelain=v1", "-uall")
	if err != nil {
		return nil, err
	}

	items, err := ParseStatusPorcelain(repoRoot, "", out)
	if err != nil {
		return nil, err
	}

	return items, nil
}

func ParseStatusPorcelain(repoRoot string, submodulePath string, output string) ([]model.ChangeItem, error) {
	lines := strings.Split(output, "\n")
	items := make([]model.ChangeItem, 0, len(lines))

	for i, raw := range lines {
		line := strings.TrimRight(raw, "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}

		if strings.HasPrefix(line, "?? ") {
			path := normalizePath(line[3:])
			items = append(items, model.ChangeItem{
				RepoRoot:       repoRoot,
				Path:           path,
				Type:           model.ChangeTypeUntracked,
				SubmodulePath:  submodulePath,
				StagedStatus:   ' ',
				WorktreeStatus: '?',
			})
			continue
		}

		if len(line) < 4 || line[2] != ' ' {
			return nil, fmt.Errorf("invalid porcelain line %d: %q", i+1, line)
		}

		staged := rune(line[0])
		worktree := rune(line[1])
		path := normalizePath(line[3:])

		if staged != ' ' && staged != '?' {
			items = append(items, model.ChangeItem{
				RepoRoot:       repoRoot,
				Path:           path,
				Type:           model.ChangeTypeStaged,
				SubmodulePath:  submodulePath,
				StagedStatus:   staged,
				WorktreeStatus: worktree,
			})
		}

		if worktree != ' ' && worktree != '?' {
			items = append(items, model.ChangeItem{
				RepoRoot:       repoRoot,
				Path:           path,
				Type:           model.ChangeTypeUnstaged,
				SubmodulePath:  submodulePath,
				StagedStatus:   staged,
				WorktreeStatus: worktree,
			})
		}
	}

	return items, nil
}

func normalizePath(path string) string {
	if parts := strings.Split(path, " -> "); len(parts) == 2 {
		return parts[1]
	}

	return strings.TrimSpace(path)
}
