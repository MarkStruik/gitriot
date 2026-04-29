package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"gitriot/internal/app"
	"gitriot/internal/config"
	"gitriot/internal/theme"
)

var version = "dev"

func main() {
	repoFlag := flag.String("repo", ".", "Path to repository root")
	themeFlag := flag.String("theme", "", "Theme name from built-ins or ~/.gitriot/themes")
	themeFileFlag := flag.String("theme-file", "", "Absolute path to theme YAML file")
	listThemesFlag := flag.Bool("list-themes", false, "List built-in theme names and exit")
	versionFlag := flag.Bool("version", false, "Print version and exit")
	recentWindowFlag := flag.Duration("recent-window", 0, "Time window around root last commit (example: 90m, 2h)")
	sinceCommitFlag := flag.String("since-commit", "", "Root commit hash (partial or full): show changes from that commit timestamp to now")
	noAltScreenFlag := flag.Bool("no-alt-screen", false, "Disable alternate screen mode (useful for embedded terminals)")
	flag.Parse()

	if *versionFlag {
		fmt.Println(version)
		return
	}

	if *listThemesFlag {
		for _, name := range theme.BuiltinNames() {
			fmt.Println(name)
		}
		return
	}

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

	selectedTheme := theme.Default
	if themePath != "" {
		loaded, err := theme.LoadFromFile(themePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to load theme file: %v\n", err)
			os.Exit(1)
		}
		selectedTheme = loaded
	} else {
		candidatePath := config.ThemePath(paths, themeName)
		if loaded, err := theme.LoadFromFile(candidatePath); err == nil {
			selectedTheme = loaded
		} else if builtin, ok := theme.LookupBuiltin(themeName); ok {
			selectedTheme = builtin
		} else {
			fmt.Fprintf(os.Stderr, "unknown theme %q\n", themeName)
			fmt.Fprintf(os.Stderr, "use --list-themes to see built-in theme names\n")
			os.Exit(1)
		}
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
		RepoPath:  repoPath,
		Theme:     selectedTheme,
		ThemeName: themeName,
		Themes:    availableThemes(paths),
		SaveTheme: func(name string) error {
			cfg.Theme = name
			cfg.ThemeFile = ""
			return config.Save(paths, cfg)
		},
		RecentWindow: *recentWindowFlag,
		SinceCommit:  *sinceCommitFlag,
	}

	programOptions := []tea.ProgramOption{}
	if !*noAltScreenFlag && !isEmbeddedTerminal() {
		programOptions = append(programOptions, tea.WithAltScreen())
	}

	p := tea.NewProgram(app.NewModel(modelOption), programOptions...)
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to run GitRiot: %v\n", err)
		os.Exit(1)
	}
}

func isEmbeddedTerminal() bool {
	v := os.Getenv("TERMINAL_EMULATOR")
	if v == "JetBrains-JediTerm" {
		return true
	}

	v = os.Getenv("TERM_PROGRAM")
	return v == "JetBrains-JediTerm"
}

func availableThemes(paths config.Paths) []theme.NamedTheme {
	byName := map[string]theme.NamedTheme{}
	for _, builtin := range theme.Builtins() {
		byName[builtin.Name] = builtin
	}
	entries, err := os.ReadDir(paths.ThemesDir)
	if err == nil {
		for _, entry := range entries {
			if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".yaml") {
				continue
			}
			name := strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
			loaded, loadErr := theme.LoadFromFile(filepath.Join(paths.ThemesDir, entry.Name()))
			if loadErr != nil {
				continue
			}
			loaded.Name = name
			byName[name] = theme.NamedTheme{Name: name, Theme: loaded, Kind: "custom"}
		}
	}
	names := make([]string, 0, len(byName))
	for name := range byName {
		names = append(names, name)
	}
	sort.Strings(names)
	out := make([]theme.NamedTheme, 0, len(names))
	for _, name := range names {
		out = append(out, byName[name])
	}
	return out
}
