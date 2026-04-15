package git

import (
	"context"
	"strings"

	"gitriot/internal/model"
)

type DiffLoader struct {
	runner Runner
}

func NewDiffLoader(runner Runner) *DiffLoader {
	return &DiffLoader{runner: runner}
}

func (d *DiffLoader) Load(ctx context.Context, req model.DiffRequest) (model.DiffResult, error) {
	args := make([]string, 0, 4)
	args = append(args, "diff")
	if req.Mode == model.DiffModeStaged {
		args = append(args, "--cached")
	}
	args = append(args, "--", req.Path)

	workingDir := req.RepoRoot
	if req.SubmodulePath != "" {
		workingDir = joinRepoPath(req.RepoRoot, req.SubmodulePath)
	}

	patch, err := d.runner.Run(ctx, workingDir, args...)
	if err != nil {
		return model.DiffResult{}, err
	}

	res := model.DiffResult{
		Request: req,
		Patch:   patch,
		Empty:   strings.TrimSpace(patch) == "",
	}

	if strings.Contains(patch, "Binary files") || strings.Contains(patch, "GIT binary patch") {
		res.IsBinary = true
	}

	return res, nil
}
