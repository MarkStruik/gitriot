package git

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

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

	rootItems, err := r.IndexRoot(ctx, repoRoot)
	if err != nil {
		return IndexResult{}, err
	}
	result.Items = append(result.Items, rootItems...)

	subResult, err := r.IndexSubmodules(ctx, repoRoot)
	if err != nil {
		return IndexResult{}, err
	}
	result.Items = append(result.Items, subResult.Items...)
	result.Warnings = append(result.Warnings, subResult.Warnings...)

	return result, nil
}

func (r *RepositoryIndexer) IndexRoot(ctx context.Context, repoRoot string) ([]model.ChangeItem, error) {
	return r.status.Index(ctx, repoRoot)
}

func (r *RepositoryIndexer) IndexSubmodules(ctx context.Context, repoRoot string) (IndexResult, error) {
	result := IndexResult{Items: make([]model.ChangeItem, 0)}

	submodules, err := r.DiscoverSubmodules(ctx, repoRoot)
	if err != nil {
		return IndexResult{}, err
	}

	for _, submodulePath := range submodules {
		_ = submodulePath
	}

	type submoduleIndexResult struct {
		path  string
		items []model.ChangeItem
		err   error
	}

	workers := runtime.NumCPU()
	if workers < 2 {
		workers = 2
	}
	if workers > len(submodules) {
		workers = len(submodules)
	}
	if workers == 0 {
		return result, nil
	}

	jobs := make(chan string)
	out := make(chan submoduleIndexResult, len(submodules))
	var wg sync.WaitGroup

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for submodulePath := range jobs {
				if ctx.Err() != nil {
					return
				}
				submoduleDir := joinRepoPath(repoRoot, submodulePath)
				statusOut, runErr := r.runner.Run(ctx, submoduleDir, "-c", "core.quotepath=false", "status", "--porcelain=v1", "-uall")
				if runErr != nil {
					out <- submoduleIndexResult{path: submodulePath, err: fmt.Errorf("indexing failed: %w", runErr)}
					continue
				}

				subItems, parseErr := ParseStatusPorcelain(repoRoot, submodulePath, statusOut)
				if parseErr != nil {
					out <- submoduleIndexResult{path: submodulePath, err: fmt.Errorf("parse failed: %w", parseErr)}
					continue
				}

				out <- submoduleIndexResult{path: submodulePath, items: subItems}
			}
		}()
	}

	for _, submodulePath := range submodules {
		jobs <- submodulePath
	}
	close(jobs)
	wg.Wait()
	close(out)

	for subResult := range out {
		if subResult.err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("submodule %q %v", subResult.path, subResult.err))
			continue
		}
		result.Items = append(result.Items, subResult.items...)
	}

	return result, nil
}

func (r *RepositoryIndexer) DiscoverSubmodules(ctx context.Context, repoRoot string) ([]string, error) {
	return discoverSubmodulePaths(ctx, repoRoot, r.runner)
}

func (r *RepositoryIndexer) IndexSubmodule(ctx context.Context, repoRoot string, submodulePath string) ([]model.ChangeItem, error) {
	submoduleDir := joinRepoPath(repoRoot, submodulePath)
	statusOut, runErr := r.runner.Run(ctx, submoduleDir, "-c", "core.quotepath=false", "status", "--porcelain=v1", "-uall")
	if runErr != nil {
		return nil, fmt.Errorf("indexing failed: %w", runErr)
	}
	items, parseErr := ParseStatusPorcelain(repoRoot, submodulePath, statusOut)
	if parseErr != nil {
		return nil, fmt.Errorf("parse failed: %w", parseErr)
	}
	return items, nil
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
