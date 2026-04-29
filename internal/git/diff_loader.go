package git

import (
	"context"
	"os"
	"strconv"
	"strings"
	"time"

	"gitriot/internal/model"
)

type LineRange struct {
	Start int
	End   int
}

type LineDecoration struct {
	Added        bool
	Deleted      bool
	DeletedLines []string
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

type SinceDiffRequest struct {
	RepoRoot      string
	SubmodulePath string
	Path          string
	Since         time.Time
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

func (d *DiffLoader) LoadSince(ctx context.Context, req SinceDiffRequest) (model.DiffResult, error) {
	workingDir := req.RepoRoot
	if req.SubmodulePath != "" {
		workingDir = joinRepoPath(req.RepoRoot, req.SubmodulePath)
	}

	baseHash, err := d.baseCommitBefore(ctx, workingDir, req.Since)
	if err != nil {
		return model.DiffResult{}, err
	}

	patch, err := d.runner.Run(ctx, workingDir, "diff", baseHash+"..HEAD", "--", req.Path)
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

func (d *DiffLoader) LoadHeadFile(ctx context.Context, req SinceDiffRequest) (string, error) {
	workingDir := req.RepoRoot
	if req.SubmodulePath != "" {
		workingDir = joinRepoPath(req.RepoRoot, req.SubmodulePath)
	}
	return d.runner.Run(ctx, workingDir, "show", "HEAD:"+req.Path)
}

func (d *DiffLoader) baseCommitBefore(ctx context.Context, workingDir string, since time.Time) (string, error) {
	// Git's --before/--until matching includes commits at the exact second. Use
	// the previous second so the selected anchor commit remains part of the range.
	exclusiveBefore := since.UTC().Add(-time.Second)
	out, err := d.runner.Run(ctx, workingDir, "rev-list", "-n", "1", "--before="+exclusiveBefore.Format(time.RFC3339), "HEAD")
	if err != nil {
		return "", err
	}
	hash := strings.TrimSpace(out)
	if hash != "" {
		return hash, nil
	}
	root, rootErr := d.runner.Run(ctx, workingDir, "rev-list", "--max-parents=0", "HEAD")
	if rootErr != nil {
		return "", rootErr
	}
	parts := strings.Fields(root)
	if len(parts) == 0 {
		return "", os.ErrNotExist
	}
	return parts[0], nil
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

func ParseLineDecorationsFromPatch(patch string) map[int]LineDecoration {
	lines := strings.Split(patch, "\n")
	decor := make(map[int]LineDecoration)
	oldLine := 0
	newLine := 0
	inHunk := false

	for _, line := range lines {
		if strings.HasPrefix(line, "@@ ") {
			oldStart, _, newStart, _, ok := parseUnifiedHeader(line)
			if !ok {
				inHunk = false
				continue
			}
			oldLine = oldStart
			newLine = newStart
			inHunk = true
			continue
		}

		if !inHunk || line == "" {
			continue
		}

		switch line[0] {
		case '+':
			d := decor[newLine]
			d.Added = true
			decor[newLine] = d
			newLine++
		case '-':
			target := newLine
			if target <= 0 {
				target = 1
			}
			d := decor[target]
			d.Deleted = true
			d.DeletedLines = append(d.DeletedLines, strings.TrimPrefix(line, "-"))
			decor[target] = d
			oldLine++
		case ' ':
			oldLine++
			newLine++
		case '\\':
			continue
		default:
			if oldLine > 0 || newLine > 0 {
				oldLine++
				newLine++
			}
		}
	}

	return decor
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

func parseUnifiedHeader(line string) (oldStart int, oldCount int, newStart int, newCount int, ok bool) {
	if !strings.HasPrefix(line, "@@ ") {
		return 0, 0, 0, 0, false
	}
	parts := strings.Split(line, " ")
	if len(parts) < 4 {
		return 0, 0, 0, 0, false
	}
	if !strings.HasPrefix(parts[1], "-") || !strings.HasPrefix(parts[2], "+") {
		return 0, 0, 0, 0, false
	}
	oldStart, oldCount = parseHunkRange(strings.TrimPrefix(parts[1], "-"))
	newStart, newCount = parseHunkRange(strings.TrimPrefix(parts[2], "+"))
	if oldStart <= 0 || newStart <= 0 {
		return 0, 0, 0, 0, false
	}
	return oldStart, oldCount, newStart, newCount, true
}
