package ui

import (
	"github.com/charmbracelet/lipgloss"
	"gitriot/internal/theme"
)

type Styles struct {
	Frame        lipgloss.Style
	Pane         lipgloss.Style
	PaneActive   lipgloss.Style
	Title        lipgloss.Style
	Muted        lipgloss.Style
	Status       lipgloss.Style
	Added        lipgloss.Style
	Removed      lipgloss.Style
	Hunk         lipgloss.Style
	DiffMeta     lipgloss.Style
	DiffNormal   lipgloss.Style
	SearchPrompt lipgloss.Style
}

func NewStyles(t theme.FileTheme) Styles {
	return Styles{
		Frame: lipgloss.NewStyle().
			Foreground(lipgloss.Color(t.Colors.Fg)).
			Background(lipgloss.Color(t.Colors.Bg)),
		Pane: lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color(t.Colors.Border)).
			Padding(0, 1),
		PaneActive: lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color(t.Colors.Accent)).
			Padding(0, 1),
		Title:        lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(t.Colors.Accent)),
		Muted:        lipgloss.NewStyle().Foreground(lipgloss.Color(t.Colors.Muted)),
		Status:       lipgloss.NewStyle().Foreground(lipgloss.Color(t.Colors.Fg)).Background(lipgloss.Color(t.Colors.Border)).Padding(0, 1),
		Added:        lipgloss.NewStyle().Foreground(lipgloss.Color(t.Colors.Added)),
		Removed:      lipgloss.NewStyle().Foreground(lipgloss.Color(t.Colors.Removed)),
		Hunk:         lipgloss.NewStyle().Foreground(lipgloss.Color(t.Colors.Hunk)).Bold(true),
		DiffMeta:     lipgloss.NewStyle().Foreground(lipgloss.Color(t.Colors.Accent)),
		DiffNormal:   lipgloss.NewStyle().Foreground(lipgloss.Color(t.Colors.Fg)),
		SearchPrompt: lipgloss.NewStyle().Foreground(lipgloss.Color(t.Colors.Accent)).Bold(true),
	}
}
