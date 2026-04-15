package git

import (
	"context"
	"os"
	"strconv"
	"strings"

	"gitriot/internal/model"
)

type LineRange struct {
	Start int
	End   int
}

type DiffLoader struct {
	runner Runner
}

type CommitDiffRequest struct {
	RepoRoot      string
	SubmodulePath string
	CommitHash    string
	Path          string
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

func (d *DiffLoader) LoadCommit(ctx context.Context, req CommitDiffRequest) (model.DiffResult, error) {
	workingDir := req.RepoRoot
	if req.SubmodulePath != "" {
		workingDir = joinRepoPath(req.RepoRoot, req.SubmodulePath)
	}

	patch, err := d.runner.Run(ctx, workingDir, "show", req.CommitHash, "--", req.Path)
	if err != nil {
		return model.DiffResult{}, err
	}

	res := model.DiffResult{
		Patch: patch,
		Empty: strings.TrimSpace(patch) == "",
	}

	if strings.Contains(patch, "Binary files") || strings.Contains(patch, "GIT binary patch") {
		res.IsBinary = true
	}

	return res, nil
}

func (d *DiffLoader) LoadWorkingFile(req model.DiffRequest) (string, error) {
	filePath := joinRepoPath(req.RepoRoot, req.Path)
	if req.SubmodulePath != "" {
		filePath = joinRepoPath(joinRepoPath(req.RepoRoot, req.SubmodulePath), req.Path)
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}

	return string(content), nil
}

func (d *DiffLoader) LoadCommitFile(ctx context.Context, req CommitDiffRequest) (string, error) {
	workingDir := req.RepoRoot
	if req.SubmodulePath != "" {
		workingDir = joinRepoPath(req.RepoRoot, req.SubmodulePath)
	}

	out, err := d.runner.Run(ctx, workingDir, "show", req.CommitHash+":"+req.Path)
	if err != nil {
		return "", err
	}

	return out, nil
}

func ParseChangedLineRangesFromPatch(patch string) []LineRange {
	lines := strings.Split(patch, "\n")
	ranges := make([]LineRange, 0, 8)
	for _, line := range lines {
		if !strings.HasPrefix(line, "@@ ") {
			continue
		}

		plusIdx := strings.Index(line, "+")
		if plusIdx == -1 {
			continue
		}
		segment := line[plusIdx+1:]
		spaceIdx := strings.Index(segment, " ")
		if spaceIdx == -1 {
			continue
		}
		rangePart := segment[:spaceIdx]

		start, count := parseHunkRange(rangePart)
		if start <= 0 {
			continue
		}
		end := start
		if count > 0 {
			end = start + count - 1
		}
		ranges = append(ranges, LineRange{Start: start, End: end})
	}

	return ranges
}

func parseHunkRange(part string) (start int, count int) {
	count = 1
	pieces := strings.Split(part, ",")
	v, err := strconv.Atoi(strings.TrimSpace(pieces[0]))
	if err != nil {
		return 0, 0
	}
	start = v

	if len(pieces) > 1 {
		c, convErr := strconv.Atoi(strings.TrimSpace(pieces[1]))
		if convErr != nil {
			return 0, 0
		}
		count = c
	}

	return start, count
}
