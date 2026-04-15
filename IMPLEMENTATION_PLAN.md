# GitRiot Implementation Plan

## Todo Tracker
- [x] Step 0: Persist implementation plan and initialize Git repository
- [x] Step 1: Bootstrap Go module and Bubble Tea app skeleton
- [x] Step 2: Define core domain models for changes and diffs
- [x] Step 3: Build Git command runner abstraction
- [x] Step 4: Implement main repository status indexer
- [x] Step 5: Implement submodule discovery and indexing
- [ ] Step 6: Build diff loading service (staged/unstaged)
- [ ] Step 7: Add theme token system (single theme, extensible)
- [ ] Step 8: Add config + file-based theme loading (`~/.gitriot`)
- [ ] Step 9: Build base split-pane TUI layout
- [ ] Step 10: Implement keyboard navigation + filters
- [ ] Step 11: Add diff rendering color rules and polish
- [ ] Step 12: Add async loading and responsiveness safeguards
- [ ] Step 13: Add robust empty/error states
- [ ] Step 14: Add CLI flags and startup wiring
- [ ] Step 15: Add tests, docs, and release notes

## Build/Release Rule per Implementation Phase
- Build `publish/gitriot.exe` at natural pause checkpoints ("stop for air" moments) instead of after every single step.
- The `publish/` directory is intentionally excluded from Git tracking.

## Current Status Notes
- Step 1 completed with Bubble Tea shell scaffolding in `cmd/gitriot/main.go` and `internal/app/model.go`.
- Phase artifact generated: `publish/gitriot.exe`.
- Build is executed in Docker using `golang:1.24` to stay aligned with the latest toolchain baseline.
- Step 2 completed with shared domain models in `internal/model/change.go` and `internal/model/diff.go`.
- Step 3 completed with Git CLI runner abstraction in `internal/git/runner.go`.
- Step 4 completed with porcelain status parsing in `internal/git/status.go`.
- Step 5 completed with combined repo/submodule indexing in `internal/git/repository_indexer.go`.

## Delivery Workflow
1. Complete one step from the tracker.
2. Build executable to `publish/gitriot.exe`.
3. Run verification for the completed step.
4. Commit only the relevant tracked files for that step.
5. Update this tracker (checkboxes and notes).

## Detailed Step Plan

### Step 1: Bootstrap Go module and Bubble Tea app skeleton
- Initialize module and dependency graph.
- Add app entrypoint in `cmd/gitriot/main.go`.
- Add base app model in `internal/app`.
- Add minimal key handling (`q`/`ctrl+c`) and render loop.

### Step 2: Define core domain models for changes and diffs
- Add enums/types for staged, unstaged, untracked, submodule changes.
- Add normalized `ChangeItem` model.
- Add `DiffRequest` and related result models.

### Step 3: Build Git command runner abstraction
- Add `Runner` interface for Git execution.
- Add `os/exec` implementation with context support.
- Add robust error wrapping with command and directory info.

### Step 4: Implement main repository status indexer
- Execute `git status --porcelain=v1 -uall`.
- Parse porcelain output into structured records.
- Map records to normalized `ChangeItem` values.

### Step 5: Implement submodule discovery and indexing
- Detect submodules via `.gitmodules` and/or CLI.
- Run status indexer in each submodule.
- Merge root + submodule results into one change list.

### Step 6: Build diff loading service (staged/unstaged)
- Add unstaged loader via `git diff -- <file>`.
- Add staged loader via `git diff --cached -- <file>`.
- Handle binary/no-output diff states.

### Step 7: Add theme token system (single theme, extensible)
- Define token struct for all UI colors.
- Add one built-in default theme in code.
- Route all UI styles through token values.

### Step 8: Add config + file-based theme loading (`~/.gitriot`)
- Add config loader for `~/.gitriot/config.yaml`.
- Add theme loader for `~/.gitriot/themes/<name>.yaml`.
- Support CLI override (`--theme-file`) and fallbacks.

### Step 9: Build base split-pane TUI layout
- Left pane for changes; right pane for diff viewport.
- Top status line (repo, branch, counters).
- Bottom help bar for keymap hints.

### Step 10: Implement keyboard navigation + filters
- Movement (`j/k`, arrows), focus switch (`tab`), open (`enter`).
- Filter toggles for staged/unstaged/untracked/submodule.
- Search activation with `/` and refresh with `r`.

### Step 11: Add diff rendering color rules and polish
- Style added/removed/hunk/header lines.
- Preserve readability for large patches.
- Add placeholders for binary/no-diff states.

### Step 12: Add async loading and responsiveness safeguards
- Run indexing and diff fetching asynchronously.
- Add loading indicators and stale-request guards.
- Keep interaction fluid during background operations.

### Step 13: Add robust empty/error states
- Handle non-repo folder and missing Git binary.
- Handle submodule command failures without app exit.
- Show actionable guidance in status messages.

### Step 14: Add CLI flags and startup wiring
- Add `--repo`, `--theme`, and `--theme-file` flags.
- Wire config + theme resolution into startup path.
- Validate runtime setup before launching UI.

### Step 15: Add tests, docs, and release notes
- Unit tests for parser, filter logic, and config/theme loader.
- README with keybindings and config examples.
- Final implementation notes and next-release backlog.
