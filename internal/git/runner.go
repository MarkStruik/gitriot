package git

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

type Runner interface {
	Run(ctx context.Context, dir string, args ...string) (string, error)
}

type CLIRunner struct{}

func NewCLIRunner() *CLIRunner {
	return &CLIRunner{}
}

func (r *CLIRunner) Run(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git command failed in %q with args %q: %w; stderr: %s", dir, strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}

	return stdout.String(), nil
}
