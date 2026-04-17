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
	Anchor   model.RepoCommit
	Commits  []model.RepoCommit
	Files    []model.CommitFile
	Warnings []string
}

func CollectRecentCommits(ctx context.Context, runner Runner, repoRoot string, window time.Duration) (RecentCommitResult, error) {
	rootCommit, err := lastCommit(ctx, runner, repoRoot, "root", true)
	if err != nil {
		return RecentCommitResult{}, err
	}
	return CollectRecentCommitsAt(ctx, runner, repoRoot, window, rootCommit.Hash)
}

func CollectRecentCommitsAt(ctx context.Context, runner Runner, repoRoot string, window time.Duration, anchorHash string) (RecentCommitResult, error) {
	anchor, err := commitByHash(ctx, runner, repoRoot, "root", true, anchorHash)
	if err != nil {
		return RecentCommitResult{}, err
	}

	result := RecentCommitResult{Anchor: anchor, Commits: []model.RepoCommit{anchor}}
	rootFiles, err := commitFiles(ctx, runner, anchor)
	if err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("root commit file list unavailable: %v", err))
	} else {
		result.Files = append(result.Files, rootFiles...)
	}

	if window <= 0 {
		result.Files = dedupeCommitFiles(result.Files)
		return result, nil
	}

	submodules, err := discoverSubmodulePaths(ctx, repoRoot, runner)
	if err != nil {
		return RecentCommitResult{}, err
	}

	for _, submodulePath := range submodules {
		submoduleDir := joinRepoPath(repoRoot, submodulePath)
		windowStart := anchor.When.Add(-window)
		windowEnd := anchor.When.Add(window)
		commits, err := commitsWithinWindow(ctx, runner, submoduleDir, submodulePath, false, windowStart, windowEnd)
		if err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("submodule %q commits unavailable: %v", submodulePath, err))
			continue
		}
		for _, commit := range commits {
			result.Commits = append(result.Commits, commit)
			files, filesErr := commitFiles(ctx, runner, commit)
			if filesErr != nil {
				result.Warnings = append(result.Warnings, fmt.Sprintf("submodule %q commit file list unavailable: %v", submodulePath, filesErr))
				continue
			}
			result.Files = append(result.Files, files...)
		}
	}

	result.Files = dedupeCommitFiles(result.Files)
	return result, nil
}

func CollectCommitsSince(ctx context.Context, runner Runner, repoRoot string, anchorHash string) (RecentCommitResult, error) {
	anchor, err := commitByHash(ctx, runner, repoRoot, "root", true, anchorHash)
	if err != nil {
		return RecentCommitResult{}, err
	}

	now := time.Now().UTC()
	result := RecentCommitResult{Anchor: anchor}

	rootCommits, err := commitsWithinWindow(ctx, runner, repoRoot, "root", true, anchor.When, now)
	if err != nil {
		return RecentCommitResult{}, err
	}
	result.Commits = append(result.Commits, rootCommits...)
	if len(result.Commits) == 0 {
		result.Commits = append(result.Commits, anchor)
	}
	for _, commit := range rootCommits {
		files, filesErr := commitFiles(ctx, runner, commit)
		if filesErr != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("root commit %s file list unavailable: %v", shortHash(commit.Hash), filesErr))
			continue
		}
		result.Files = append(result.Files, files...)
	}

	submodules, err := discoverSubmodulePaths(ctx, repoRoot, runner)
	if err != nil {
		return RecentCommitResult{}, err
	}

	for _, submodulePath := range submodules {
		submoduleDir := joinRepoPath(repoRoot, submodulePath)
		commits, subErr := commitsWithinWindow(ctx, runner, submoduleDir, submodulePath, false, anchor.When, now)
		if subErr != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("submodule %q commits unavailable: %v", submodulePath, subErr))
			continue
		}
		for _, commit := range commits {
			result.Commits = append(result.Commits, commit)
			files, filesErr := commitFiles(ctx, runner, commit)
			if filesErr != nil {
				result.Warnings = append(result.Warnings, fmt.Sprintf("submodule %q commit file list unavailable: %v", submodulePath, filesErr))
				continue
			}
			result.Files = append(result.Files, files...)
		}
	}

	result.Files = dedupeCommitFiles(result.Files)
	return result, nil
}

func LoadRootCommitSummary(ctx context.Context, runner Runner, repoRoot string, hash string) (model.CommitSummary, error) {
	if strings.TrimSpace(hash) == "" {
		return model.CommitSummary{}, fmt.Errorf("empty commit hash")
	}

	metaOut, err := runner.Run(ctx, repoRoot, "show", "-s", "--format=%H%x1f%P%x1f%an%x1f%ae%x1f%ct%x1f%s%x1f%b", hash)
	if err != nil {
		return model.CommitSummary{}, err
	}
	metaLine := strings.TrimSpace(metaOut)
	parts := strings.SplitN(metaLine, "\x1f", 7)
	if len(parts) != 7 {
		return model.CommitSummary{}, fmt.Errorf("unexpected commit summary output")
	}
	unix, err := strconv.ParseInt(parts[4], 10, 64)
	if err != nil {
		return model.CommitSummary{}, fmt.Errorf("invalid commit timestamp: %w", err)
	}

	parentParts := []string{}
	for _, p := range strings.Fields(parts[1]) {
		if strings.TrimSpace(p) != "" {
			parentParts = append(parentParts, p)
		}
	}

	shortStatOut, err := runner.Run(ctx, repoRoot, "show", "--shortstat", "--format=", hash)
	if err != nil {
		return model.CommitSummary{}, err
	}
	shortStat := ""
	for _, line := range strings.Split(shortStatOut, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			shortStat = trimmed
			break
		}
	}

	commit := model.RepoCommit{
		Scope:    "root",
		Hash:     parts[0],
		Author:   parts[2],
		Email:    parts[3],
		When:     time.Unix(unix, 0).UTC(),
		Subject:  parts[5],
		IsRoot:   true,
		RepoPath: repoRoot,
	}

	files, filesErr := commitFiles(ctx, runner, commit)
	if filesErr != nil {
		return model.CommitSummary{}, filesErr
	}

	return model.CommitSummary{
		Hash:      parts[0],
		Author:    parts[2],
		Email:     parts[3],
		When:      time.Unix(unix, 0).UTC(),
		Subject:   parts[5],
		Body:      strings.TrimSpace(parts[6]),
		Parents:   parentParts,
		ShortStat: shortStat,
		Files:     files,
	}, nil
}

func RootCommitHistory(ctx context.Context, runner Runner, repoRoot string, limit int) ([]model.RepoCommit, error) {
	if limit <= 0 {
		limit = 100
	}
	args := []string{"log", fmt.Sprintf("-%d", limit), "--pretty=format:%H%x1f%an%x1f%ae%x1f%ct%x1f%s"}
	out, err := runner.Run(ctx, repoRoot, args...)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(out, "\n")
	commits := make([]model.RepoCommit, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		commit, parseErr := parseCommitLine(line, "root", true, repoRoot)
		if parseErr != nil {
			continue
		}
		commits = append(commits, commit)
	}
	return commits, nil
}

func lastCommit(ctx context.Context, runner Runner, repoDir string, scope string, isRoot bool) (model.RepoCommit, error) {
	out, err := runner.Run(ctx, repoDir, "log", "-1", "--pretty=format:%H%x1f%an%x1f%ae%x1f%ct%x1f%s")
	if err != nil {
		return model.RepoCommit{}, err
	}

	line := strings.TrimSpace(out)
	return parseCommitLine(line, scope, isRoot, repoDir)
}

func commitByHash(ctx context.Context, runner Runner, repoDir string, scope string, isRoot bool, hash string) (model.RepoCommit, error) {
	if strings.TrimSpace(hash) == "" {
		return lastCommit(ctx, runner, repoDir, scope, isRoot)
	}
	out, err := runner.Run(ctx, repoDir, "show", "-s", "--pretty=format:%H%x1f%an%x1f%ae%x1f%ct%x1f%s", hash)
	if err != nil {
		return model.RepoCommit{}, err
	}
	line := strings.TrimSpace(out)
	return parseCommitLine(line, scope, isRoot, repoDir)
}

func parseCommitLine(line string, scope string, isRoot bool, repoDir string) (model.RepoCommit, error) {
	parts := strings.SplitN(line, "\x1f", 5)
	if len(parts) != 5 {
		return model.RepoCommit{}, fmt.Errorf("unexpected git log output for %s", scope)
	}

	unix, err := strconv.ParseInt(parts[3], 10, 64)
	if err != nil {
		return model.RepoCommit{}, fmt.Errorf("invalid commit timestamp for %s: %w", scope, err)
	}

	return model.RepoCommit{
		Scope:    scope,
		Hash:     parts[0],
		Author:   parts[1],
		Email:    parts[2],
		When:     time.Unix(unix, 0).UTC(),
		Subject:  parts[4],
		IsRoot:   isRoot,
		RepoPath: repoDir,
	}, nil
}

func commitsWithinWindow(ctx context.Context, runner Runner, repoDir string, scope string, isRoot bool, start time.Time, end time.Time) ([]model.RepoCommit, error) {
	out, err := runner.Run(ctx, repoDir, "log", "--pretty=format:%H%x1f%an%x1f%ae%x1f%ct%x1f%s", "--since="+start.UTC().Format(time.RFC3339), "--until="+end.UTC().Format(time.RFC3339))
	if err != nil {
		return nil, err
	}
	lines := strings.Split(out, "\n")
	commits := make([]model.RepoCommit, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		commit, parseErr := parseCommitLine(line, scope, isRoot, repoDir)
		if parseErr != nil {
			continue
		}
		commits = append(commits, commit)
	}
	return commits, nil
}

func withinWindow(anchor time.Time, candidate time.Time, window time.Duration) bool {
	delta := anchor.Sub(candidate)
	if delta < 0 {
		delta = -delta
	}

	return delta <= window
}

func commitFiles(ctx context.Context, runner Runner, commit model.RepoCommit) ([]model.CommitFile, error) {
	out, err := runner.Run(ctx, commit.RepoPath, "show", "--pretty=format:", "--name-status", "--find-renames", commit.Hash)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(out, "\n")
	files := make([]model.CommitFile, 0, len(lines))
	seen := make(map[string]struct{})
	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		status := normalizeStatus(parts[0])
		path := parts[len(parts)-1]
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
			Status:        status,
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

func normalizeStatus(raw string) string {
	if raw == "" {
		return "M"
	}
	s := strings.ToUpper(strings.TrimSpace(raw))
	if len(s) == 0 {
		return "M"
	}
	first := s[:1]
	switch first {
	case "A", "D", "M":
		return first
	case "R", "C":
		return "M"
	default:
		return "M"
	}
}

func shortHash(hash string) string {
	if len(hash) <= 8 {
		return hash
	}
	return hash[:8]
}

func dedupeCommitFiles(files []model.CommitFile) []model.CommitFile {
	seen := make(map[string]struct{}, len(files))
	out := make([]model.CommitFile, 0, len(files))
	for _, f := range files {
		key := f.Scope + "|" + f.Path
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, f)
	}
	return out
}
