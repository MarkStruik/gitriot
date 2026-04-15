# GitRiot Release Notes

## v0.1.0-draft
- Added Bubble Tea-based TUI foundation with split-pane navigation.
- Implemented root repository and submodule change indexing.
- Implemented staged and unstaged diff loading via native Git CLI.
- Added filter controls for staged, unstaged, untracked, and submodule scopes.
- Added single-theme token system with file-based overrides from `~/.gitriot`.
- Added CLI flags: `--repo`, `--theme`, `--theme-file`.
- Added baseline tests for status parsing, filtering, and config/theme loading.
