package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"gitriot/internal/app"
	"gitriot/internal/config"
	"gitriot/internal/theme"
)

func main() {
	repoFlag := flag.String("repo", ".", "Path to repository root")
	themeFlag := flag.String("theme", "", "Theme name from ~/.gitriot/themes")
	themeFileFlag := flag.String("theme-file", "", "Absolute path to theme YAML file")
	recentWindowFlag := flag.Duration("recent-window", 0, "Time window around root last commit (example: 90m, 2h)")
	flag.Parse()

	paths, err := config.ResolvePaths()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to resolve config paths: %v\n", err)
		os.Exit(1)
	}

	cfg, err := config.Load(paths)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		os.Exit(1)
	}

	themeName := cfg.Theme
	if *themeFlag != "" {
		themeName = *themeFlag
	}
	if themeName == "" {
		themeName = "default"
	}

	themePath := cfg.ThemeFile
	if *themeFileFlag != "" {
		themePath = *themeFileFlag
	}
	if themePath == "" {
		themePath = config.ThemePath(paths, themeName)
	}

	selectedTheme := theme.Default
	if loaded, err := theme.LoadFromFile(themePath); err == nil {
		selectedTheme = loaded
	}

	repoPath, err := filepath.Abs(*repoFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to resolve repo path: %v\n", err)
		os.Exit(1)
	}

	if *recentWindowFlag < 0 {
		fmt.Fprintf(os.Stderr, "invalid recent-window %s: must be >= 0\n", (*recentWindowFlag).String())
		os.Exit(1)
	}

	modelOption := app.Option{
		RepoPath:     repoPath,
		Theme:        selectedTheme,
		RecentWindow: *recentWindowFlag,
	}

	p := tea.NewProgram(app.NewModel(modelOption), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to run GitRiot: %v\n", err)
		os.Exit(1)
	}
}
