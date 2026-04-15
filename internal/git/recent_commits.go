package git

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"gitriot/internal/model"
)

type RecentCommitResult struct {
	Commits  []model.RepoCommit
	Files    []model.CommitFile
	Warnings []string
}

func CollectRecentCommits(ctx context.Context, runner Runner, repoRoot string, window time.Duration) (RecentCommitResult, error) {
	rootCommit, err := lastCommit(ctx, runner, repoRoot, "root", true)
	if err != nil {
		return RecentCommitResult{}, err
	}

	result := RecentCommitResult{Commits: []model.RepoCommit{rootCommit}}
	rootFiles, err := commitFiles(ctx, runner, rootCommit)
	if err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("root commit file list unavailable: %v", err))
	} else {
		result.Files = append(result.Files, rootFiles...)
	}

	if window <= 0 {
		return result, nil
	}

	submodules, err := discoverSubmodulePaths(ctx, repoRoot, runner)
	if err != nil {
		return RecentCommitResult{}, err
	}

	for _, submodulePath := range submodules {
		submoduleDir := joinRepoPath(repoRoot, submodulePath)
		commit, err := lastCommit(ctx, runner, submoduleDir, submodulePath, false)
		if err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("submodule %q last commit unavailable: %v", submodulePath, err))
			continue
		}

		if withinWindow(rootCommit.When, commit.When, window) {
			result.Commits = append(result.Commits, commit)
			files, filesErr := commitFiles(ctx, runner, commit)
			if filesErr != nil {
				result.Warnings = append(result.Warnings, fmt.Sprintf("submodule %q commit file list unavailable: %v", submodulePath, filesErr))
				continue
			}
			result.Files = append(result.Files, files...)
		}
	}

	return result, nil
}

func lastCommit(ctx context.Context, runner Runner, repoDir string, scope string, isRoot bool) (model.RepoCommit, error) {
	out, err := runner.Run(ctx, repoDir, "log", "-1", "--pretty=format:%H%x1f%an%x1f%ct%x1f%s")
	if err != nil {
		return model.RepoCommit{}, err
	}

	line := strings.TrimSpace(out)
	parts := strings.SplitN(line, "\x1f", 4)
	if len(parts) != 4 {
		return model.RepoCommit{}, fmt.Errorf("unexpected git log output for %s", scope)
	}

	unix, err := strconv.ParseInt(parts[2], 10, 64)
	if err != nil {
		return model.RepoCommit{}, fmt.Errorf("invalid commit timestamp for %s: %w", scope, err)
	}

	return model.RepoCommit{
		Scope:    scope,
		Hash:     parts[0],
		Author:   parts[1],
		When:     time.Unix(unix, 0).UTC(),
		Subject:  parts[3],
		IsRoot:   isRoot,
		RepoPath: repoDir,
	}, nil
}

func withinWindow(anchor time.Time, candidate time.Time, window time.Duration) bool {
	delta := anchor.Sub(candidate)
	if delta < 0 {
		delta = -delta
	}

	return delta <= window
}

func commitFiles(ctx context.Context, runner Runner, commit model.RepoCommit) ([]model.CommitFile, error) {
	out, err := runner.Run(ctx, commit.RepoPath, "show", "--pretty=format:", "--name-only", commit.Hash)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(out, "\n")
	files := make([]model.CommitFile, 0, len(lines))
	seen := make(map[string]struct{})
	for _, raw := range lines {
		path := strings.TrimSpace(raw)
		if path == "" {
			continue
		}
		if _, ok := seen[path]; ok {
			continue
		}
		seen[path] = struct{}{}

		submodulePath := ""
		if !commit.IsRoot {
			submodulePath = commit.Scope
		}

		files = append(files, model.CommitFile{
			Scope:         commit.Scope,
			CommitHash:    commit.Hash,
			Path:          path,
			Subject:       commit.Subject,
			When:          commit.When,
			Author:        commit.Author,
			IsRoot:        commit.IsRoot,
			RepoPath:      commit.RepoPath,
			SubmodulePath: submodulePath,
		})
	}

	return files, nil
}
