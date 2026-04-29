# GitRiot

GitRiot is a keyboard-first terminal diff command center for Git repositories and submodules.

<img width="1984" height="1289" alt="image" src="https://github.com/user-attachments/assets/8afe4db9-bcf9-4845-aee8-1b997f50f0c4" />

## Current MVP
- Unified change list for root repo and submodules
- Staged, unstaged, untracked, and submodule filtering
- Split-pane TUI with file list and diff viewport
- Optional recent commit snapshot across root + submodules by timeframe
- File-based theme loading from `~/.gitriot/themes`

## Run (Docker toolchain)
```bash
docker run --rm -it -v "${PWD}:/src" -w /src golang:1.24 sh -c "go run ./cmd/gitriot --repo ."
```

Run with recent commit window enabled:
```bash
docker run --rm -it -v "${PWD}:/src" -w /src golang:1.24 sh -c "go run ./cmd/gitriot --repo . --recent-window 2h"
```

Run from a specific root commit timestamp to now (partial hash supported):
```bash
docker run --rm -it -v "${PWD}:/src" -w /src golang:1.24 sh -c "go run ./cmd/gitriot --repo . --since-commit a1b2c3d"
```

Run in embedded terminals (disable alternate screen):
```bash
docker run --rm -it -v "${PWD}:/src" -w /src golang:1.24 sh -c "go run ./cmd/gitriot --repo . --no-alt-screen"
```

## Build Windows executable
```bash
docker run --rm -v "${PWD}:/src" -w /src golang:1.24 sh -c "mkdir -p publish && CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o publish/gitriot.exe ./cmd/gitriot"
```

Faster cached build with Docker volumes:
```powershell
./scripts/build.ps1
```

Faster cached tests with Docker volumes:
```powershell
./scripts/test.ps1
```

This uses persistent Docker volumes for:
- `/go/pkg/mod` (module download cache)
- `/root/.cache/go-build` (compiled build cache)

## Configuration
- Config file: `~/.gitriot/config.yaml`
- Theme files: `~/.gitriot/themes/<name>.yaml`

Example `~/.gitriot/config.yaml`:
```yaml
theme: default
# theme_file: C:/Users/you/.gitriot/themes/default.yaml
```

Example theme file:
```yaml
name: default
colors:
  bg: "#0d1117"
  panel_left_bg: "#161b22"
  panel_right_bg: "#1f2430"
  panel_active_bg: "#2d333b"
  fg: "#c9d1d9"
  muted: "#8b949e"
  accent: "#58a6ff"
  added: "#3fb950"
  removed: "#f85149"
  modified: "#d29922"
  hunk: "#d29922"
  border: "#30363d"
  line_sep: "#6e7681"
  row_added_bg: "#1a472a"
  row_removed_bg: "#4b1d1d"
  row_modified_bg: "#4b3a19"
  syntax_plain: "#c9d1d9"
  syntax_keyword: "#ff7b72"
  syntax_string: "#a5d6ff"
  syntax_comment: "#8b949e"
  syntax_type: "#ffa657"
  syntax_func: "#d2a8ff"
  syntax_number: "#79c0ff"
  syntax_operator: "#ff7b72"
  syntax_punct: "#c9d1d9"
```

## Keybindings
- `q` / `ctrl+c`: quit
- `tab`: switch focus between change list and diff pane
- `j` / `k` or arrows: move selection
- `left` / `h` (tree pane): collapse folder or move left
- `right` / `l` (tree pane): expand folder
- `space` (tree pane): toggle folder collapse
- `x` / `X` (tree pane): collapse all / expand all
- `enter`: reload diff for current selection
- `left` / `h` (diff pane): pan left
- `right` / `l` (diff pane): pan right
- `f` (diff pane): toggle full-file vs hunks-only view (hunks show +/- 5 context lines)
- `s`: toggle staged changes
- `u`: toggle unstaged changes
- `n`: toggle untracked changes
- `m`: toggle submodule changes
- `c`: return to current-changes-only view (and re-enable live auto-refresh)
- `[` / `]` (left pane): switch `[Files] | [Commits]` tab
- `enter` (Commits tab): apply selected root commit anchor (timestamp -> now) and switch back to Files tab
- `/`: open search input
- `esc`: close search input
- `r`: refresh index

## Notes
- GitRiot currently shells out to native Git CLI commands.
- Submodule failures are reported as warnings; the app remains interactive.
- `--since-commit <hash>` uses the selected root commit timestamp as anchor and loads root + submodule commits from that time to now.
- Left tabs are always available: Files and Commits. The Commits tab previews selected commit details in the right pane.
- While viewing an older merged range, live background refresh of working changes is paused; pressing `c` returns to current-only and resumes live refresh.
- The files tree shows a scrollbar indicator when more rows exist than fit in the pane viewport.
- In recent mode, the left pane lists files from those commits and selecting a file auto-loads its commit diff.
- Embedded terminals (for example Rider/JetBrains terminal) may render better with `--no-alt-screen`.
