# GitRiot Release Notes

## v0.1.1
- Fixed hunk rendering so deleted lines do not mark the following unchanged row as removed.
- Preserved deleted rows at the end of files when rendering numbered diffs.

## v0.1.0
- Added Bubble Tea-based TUI foundation with split-pane navigation.
- Implemented root repository and submodule change indexing.
- Implemented staged and unstaged diff loading via native Git CLI.
- Added filter controls for staged, unstaged, untracked, and submodule scopes.
- Added optional recent commit snapshot view (`c`) with CLI timeframe flag (`--recent-window`).
- Added single-theme token system with file-based overrides from `~/.gitriot`.
- Added CLI flags: `--repo`, `--theme`, `--theme-file`, `--recent-window`, and `--version`.
- Added baseline tests for status parsing, filtering, and config/theme loading.
