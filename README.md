# GitRiot

GitRiot is a keyboard-first terminal diff command center for Git repositories and submodules.

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

## Build Windows executable
```bash
docker run --rm -v "${PWD}:/src" -w /src golang:1.24 sh -c "mkdir -p publish && CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o publish/gitriot.exe ./cmd/gitriot"
```

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
  bg: "#111418"
  fg: "#E6E9EF"
  muted: "#8A93A6"
  accent: "#5EA1FF"
  added: "#2FBF71"
  removed: "#E05D5D"
  hunk: "#E0B84F"
  border: "#2A3140"
```

## Keybindings
- `q` / `ctrl+c`: quit
- `tab`: switch focus between change list and diff pane
- `j` / `k` or arrows: move selection
- `enter`: load diff for selected change
- `s`: toggle staged changes
- `u`: toggle unstaged changes
- `n`: toggle untracked changes
- `m`: toggle submodule changes
- `c`: toggle recent commit snapshot view (requires `--recent-window`)
- `/`: open search input
- `esc`: close search input
- `r`: refresh index

## Notes
- GitRiot currently shells out to native Git CLI commands.
- Submodule failures are reported as warnings; the app remains interactive.
- Recent commit view is anchored to the root repository last commit and includes submodules whose last commit time is within the provided window.
