# GitRiot

GitRiot is a keyboard-first terminal diff viewer for Git repositories and submodules.

<img width="2504" height="1127" alt="image" src="https://github.com/user-attachments/assets/c74cba6c-7aa2-4c8f-8498-238a4c1b1fcb" />

## Features
- Unified file tree for root repository and submodule changes
- Staged, unstaged, untracked, and submodule filters
- Split-pane TUI with file navigation, syntax-highlighted diffs, and hunk/full-file modes
- Recent commit and since-commit views across root and submodule repositories
- Built-in themes, custom theme files, and an in-app theme picker
- Horizontal diff scrolling, changed-row jumping, and compact folder rendering

## Run
Build GitRiot, put the resulting executable on your `PATH`, then run it from any Git repository:

```bash
gitriot --repo .
```

Run with a recent commit window:

```bash
gitriot --repo . --recent-window 2h
```

Run from a root commit timestamp to now. Partial hashes are supported:

```bash
gitriot --repo . --since-commit a1b2c3d
```

List built-in themes:

```bash
gitriot --list-themes
```

Run with a built-in theme:

```bash
gitriot --repo . --theme forest
```

Disable alternate screen mode for embedded terminals:

```bash
gitriot --repo . --no-alt-screen
```

## Build And Test
Build for the current platform. The output is `publish/gitriot`:

```bash
go build -o publish/gitriot ./cmd/gitriot
```

Build the Windows executable with Docker. The output is `publish/gitriot.exe`:

```bash
docker run --rm -v "${PWD}:/src" -w /src golang:1.24 sh -c "mkdir -p publish && CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o publish/gitriot.exe ./cmd/gitriot"
```

Use the cached helper scripts for normal development:

```powershell
./scripts/build.ps1
```

```powershell
./scripts/test.ps1
```

After building, add the `publish` directory to your `PATH` or copy/rename the executable to a directory already on `PATH`. The examples in this README assume the command is available as `gitriot`.

The helper scripts use persistent Docker volumes for the Go module and build caches:

- `/go/pkg/mod` (module download cache)
- `/root/.cache/go-build` (compiled build cache)

## Configuration And Themes
- Config file: `~/.gitriot/config.yaml`
- Theme files: `~/.gitriot/themes/<name>.yaml`

Built-in themes:
- `default`
- `forest`
- `midnight`
- `violet`

Theme selection order:
- `--theme-file` if provided
- Custom file at `~/.gitriot/themes/<name>.yaml` if it exists
- Built-in theme with that name

Theme usage:

- Start with a theme once: `gitriot --repo . --theme forest`
- Load a specific file: `gitriot --repo . --theme-file C:/Users/you/.gitriot/themes/custom.yaml`
- Change themes in-app with `t`
- Move through the theme picker to preview themes live
- Press `enter` to apply and save the selected theme to `~/.gitriot/config.yaml`
- Press `esc` to cancel and restore the previous theme

Theme files are YAML. Missing color tokens inherit from the default theme, so a custom theme can override only the values it cares about.

Theme colors affect:

- Pane backgrounds, borders, headers, muted text, and accent text
- File status colors for added, removed, and modified files
- Diff row tinting for added and removed lines
- Syntax highlighting for code in the diff pane

Syntax tokens:

- `syntax_plain`: normal identifiers and text
- `syntax_keyword`: language keywords and reserved words
- `syntax_string`: strings and string-like literals
- `syntax_comment`: comments and doc strings
- `syntax_type`: classes, types, tags, and namespaces
- `syntax_func`: functions, built-ins, and decorators
- `syntax_number`: numeric constants
- `syntax_operator`: operators
- `syntax_punct`: punctuation

Example `~/.gitriot/config.yaml`:

```yaml
theme: forest
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
Global:

- `q` / `ctrl+c`: quit
- `tab`: switch focus between file tree and diff pane
- `r`: refresh index
- `/`: open search input
- `esc`: close search or cancel an open popup
- `t`: open theme picker
- `c`: return to current-changes-only view and resume live refresh
- `?` / `f1`: show keybinding help

File tree:

- `j` / `k` or arrows: move selection
- `pgup` / `pgdown`: move selection by a page
- `left` / `h`: collapse folder or move left
- `right` / `l`: expand folder
- `space`: toggle folder collapse
- `x` / `X`: collapse all / expand all
- `enter`: toggle selected folder, reload selected file diff, or open selected commit anchor from the Commits tab
- `[` / `]`: switch between `[Files]` and `[Commits]`
- `s`: toggle staged changes
- `u`: toggle unstaged changes
- `n`: toggle untracked changes
- `m`: toggle submodule changes

Diff pane:

- `left` / `h`: pan left
- `right` / `l`: pan right
- `f`: toggle full-file vs hunks-only view. Hunks mode shows +/- 5 context lines.
- `{` / `}`: jump to previous or next changed row

Search:

- Type to filter the current file or commit list
- `enter`: apply search and return focus to the previous pane
- `esc`: close search

Theme picker:

- Type to filter themes
- `up` / `down`: preview previous or next theme
- `pgup` / `pgdown`: move through themes faster
- `enter`: apply and save selected theme
- `esc` / `t`: cancel and restore the previous theme

## Notes
- GitRiot currently shells out to native Git CLI commands.
- Submodule failures are reported as warnings; the app remains interactive.
- `--since-commit <hash>` uses the selected root commit timestamp as the anchor and loads root plus submodule commits from that time to now.
- While viewing an older merged range, live background refresh is paused. Press `c` to return to current changes and resume refresh.
- In recent mode, the left pane lists files from those commits and selecting a file auto-loads its commit diff.
- Embedded terminals (for example Rider/JetBrains terminal) may render better with `--no-alt-screen`.
