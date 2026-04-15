# GitRiot: Diff Command Center

## Project Definition (Draft v0.1)

## 1) Vision
GitRiot is a terminal user interface (TUI) that gives developers one place to inspect all Git changes across a repository and its submodules, with readable diffs, themes, and code-aware coloring.

## 2) Problem Statement
Developers often switch between multiple commands and directories to understand what changed:
- main repo status and diff
- submodule status and nested diffs
- staged vs unstaged vs untracked views

This creates friction and can hide important changes.

## 3) Product Goal
Provide a fast, visual, and keyboard-first "command center" for Git changes that makes full-repo and submodule review easy.

## 4) Target Users
- Developers working in mono-repos
- Teams using Git submodules
- Engineers who prefer terminal workflows

## 5) Core Use Cases
- See all changed files in main repo and submodules from one screen
- Open a file diff with syntax highlighting
- Filter changes by type (staged, unstaged, untracked, submodule)
- Navigate quickly with keyboard shortcuts
- Switch themes for readability and preference

## 6) MVP Scope
### In Scope
- Repository overview panel
- Submodule change discovery panel
- Diff viewer with syntax coloring
- Basic theming (at least 2 themes)
- Keyboard navigation and search/filter

### Out of Scope (MVP)
- Editing/committing changes directly
- Merge conflict resolution UI
- Remote/PR workflow integrations

## 7) Success Criteria
- User can identify all repo + submodule changes in under 30 seconds
- Diff rendering remains responsive on medium repositories
- Keyboard-only workflow is complete for core actions

## 8) Technical Direction (Initial)
- Language/runtime: TBD
- Git data source: native Git CLI invocation
- Rendering: TUI framework with split-pane support
- Diff highlighting: token/syntax-aware formatter

## 9) Milestones
1. CLI scaffolding + config
2. Repo/submodule change indexer
3. Diff viewer (staged/unstaged)
4. Theme system
5. UX polish + docs + first release

## 10) Open Questions
- Preferred implementation language (Go, Rust, Node, etc.)?
- Should nested submodules be recursively supported in MVP?
- Do we prioritize speed or richer diff UI first?
- Should we include a read-only commit graph panel?

---
Owner: TBD  
Status: Draft  
Last updated: 2026-04-15
