package git

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gitriot/internal/model"
)

type RepositoryIndexer struct {
	runner Runner
	status *StatusIndexer
}

type IndexResult struct {
	Items    []model.ChangeItem
	Warnings []string
}

func NewRepositoryIndexer(runner Runner) *RepositoryIndexer {
	return &RepositoryIndexer{
		runner: runner,
		status: NewStatusIndexer(runner),
	}
}

func (r *RepositoryIndexer) IndexAll(ctx context.Context, repoRoot string) (IndexResult, error) {
	result := IndexResult{Items: make([]model.ChangeItem, 0)}

	rootItems, err := r.status.Index(ctx, repoRoot)
	if err != nil {
		return IndexResult{}, err
	}
	result.Items = append(result.Items, rootItems...)

	submodules, err := discoverSubmodulePaths(ctx, repoRoot, r.runner)
	if err != nil {
		return IndexResult{}, err
	}

	for _, submodulePath := range submodules {
		submoduleDir := joinRepoPath(repoRoot, submodulePath)

		out, err := r.runner.Run(ctx, submoduleDir, "-c", "core.quotepath=false", "status", "--porcelain=v1", "-uall")
		if err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("submodule %q indexing failed: %v", submodulePath, err))
			continue
		}

		subItems, err := ParseStatusPorcelain(repoRoot, submodulePath, out)
		if err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("submodule %q parse failed: %v", submodulePath, err))
			continue
		}

		result.Items = append(result.Items, subItems...)
	}

	return result, nil
}

func discoverSubmodulePaths(ctx context.Context, repoRoot string, runner Runner) ([]string, error) {
	gitmodules := filepath.Join(repoRoot, ".gitmodules")
	if _, err := os.Stat(gitmodules); os.IsNotExist(err) {
		return nil, nil
	} else if err != nil {
		return nil, fmt.Errorf("checking .gitmodules failed: %w", err)
	}

	out, err := runner.Run(ctx, repoRoot, "config", "--file", ".gitmodules", "--get-regexp", "path")
	if err != nil {
		return nil, err
	}

	lines := strings.Split(out, "\n")
	paths := make([]string, 0, len(lines))
	seen := make(map[string]struct{})

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		path := fields[len(fields)-1]
		if _, ok := seen[path]; ok {
			continue
		}

		seen[path] = struct{}{}
		paths = append(paths, path)
	}

	return paths, nil
}
