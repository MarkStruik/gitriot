package git

import (
	"context"
	"strings"
)

func CurrentBranch(ctx context.Context, runner Runner, repoRoot string) (string, error) {
	out, err := runner.Run(ctx, repoRoot, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", err
	}

	branch := strings.TrimSpace(out)
	if branch == "" {
		return "(detached)", nil
	}

	return branch, nil
}

func WorkingTreeFingerprint(ctx context.Context, runner Runner, repoRoot string) (string, error) {
	out, err := runner.Run(ctx, repoRoot, "-c", "core.quotepath=false", "status", "--porcelain=v1", "-uall")
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(out), nil
}
