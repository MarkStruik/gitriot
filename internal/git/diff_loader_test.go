package git

import (
	"context"
	"strings"
	"testing"
	"time"
)

type recordingRunner struct {
	calls []recordedGitCall
}

type recordedGitCall struct {
	dir  string
	args []string
}

func (r *recordingRunner) Run(ctx context.Context, dir string, args ...string) (string, error) {
	r.calls = append(r.calls, recordedGitCall{dir: dir, args: append([]string(nil), args...)})
	if len(args) >= 4 && args[0] == "rev-list" {
		return "base123\n", nil
	}
	return "diff --git a/file.go b/file.go\n", nil
}

func TestBaseCommitBeforeExcludesAnchorSecond(t *testing.T) {
	runner := &recordingRunner{}
	loader := NewDiffLoader(runner)
	since := time.Date(2026, 4, 29, 7, 32, 46, 0, time.UTC)

	base, err := loader.baseCommitBefore(context.Background(), "/repo/sub", since)
	if err != nil {
		t.Fatalf("baseCommitBefore failed: %v", err)
	}
	if base != "base123" {
		t.Fatalf("expected base hash, got %q", base)
	}
	if len(runner.calls) != 1 {
		t.Fatalf("expected one git call, got %d", len(runner.calls))
	}

	joinedArgs := strings.Join(runner.calls[0].args, " ")
	if strings.Contains(joinedArgs, "2026-04-29T07:32:46Z") {
		t.Fatalf("expected anchor second to be excluded, got args %q", joinedArgs)
	}
	if !strings.Contains(joinedArgs, "--before=2026-04-29T07:32:45Z") {
		t.Fatalf("expected previous second in rev-list args, got %q", joinedArgs)
	}
}
