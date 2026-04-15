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
