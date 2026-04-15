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

func NewRepositoryIndexer(runner Runner) *RepositoryIndexer {
	return &RepositoryIndexer{
		runner: runner,
		status: NewStatusIndexer(runner),
	}
}

func (r *RepositoryIndexer) IndexAll(ctx context.Context, repoRoot string) ([]model.ChangeItem, error) {
	all := make([]model.ChangeItem, 0)

	rootItems, err := r.status.Index(ctx, repoRoot)
	if err != nil {
		return nil, err
	}
	all = append(all, rootItems...)

	submodules, err := discoverSubmodulePaths(ctx, repoRoot, r.runner)
	if err != nil {
		return nil, err
	}

	for _, submodulePath := range submodules {
		submoduleDir := filepath.Join(repoRoot, filepath.FromSlash(submodulePath))

		out, err := r.runner.Run(ctx, submoduleDir, "-c", "core.quotepath=false", "status", "--porcelain=v1", "-uall")
		if err != nil {
			return nil, fmt.Errorf("indexing submodule %q failed: %w", submodulePath, err)
		}

		subItems, err := ParseStatusPorcelain(repoRoot, submodulePath, out)
		if err != nil {
			return nil, fmt.Errorf("parsing submodule %q status failed: %w", submodulePath, err)
		}

		all = append(all, subItems...)
	}

	return all, nil
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
