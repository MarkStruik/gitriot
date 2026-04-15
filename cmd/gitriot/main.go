package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"gitriot/internal/app"
)

func main() {
	p := tea.NewProgram(app.NewModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to run GitRiot: %v\n", err)
		os.Exit(1)
	}
}
