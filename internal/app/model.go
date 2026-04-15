package app

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
)

type Model struct {
	ready  bool
	width  int
	height int
}

func NewModel() Model {
	return Model{}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		}
	}

	return m, nil
}

func (m Model) View() string {
	if !m.ready {
		return "Initializing GitRiot..."
	}

	return fmt.Sprintf(
		"GitRiot\n\nBubble Tea shell is ready.\nPress q to quit.\n\nViewport: %dx%d",
		m.width,
		m.height,
	)
}
