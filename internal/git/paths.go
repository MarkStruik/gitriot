package git

import "path/filepath"

func joinRepoPath(repoRoot string, relativePath string) string {
	return filepath.Join(repoRoot, filepath.FromSlash(relativePath))
}
