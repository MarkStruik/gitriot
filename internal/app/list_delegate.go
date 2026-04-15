package app

import (
	"fmt"
	"io"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type changeListDelegate struct {
	normalStyle   lipgloss.Style
	selectedStyle lipgloss.Style
}

type titledItem interface {
	Title() string
}

func newChangeListDelegate(normalStyle lipgloss.Style, selectedStyle lipgloss.Style) changeListDelegate {
	return changeListDelegate{normalStyle: normalStyle, selectedStyle: selectedStyle}
}

func (d changeListDelegate) Height() int {
	return 1
}

func (d changeListDelegate) Spacing() int {
	return 0
}

func (d changeListDelegate) Update(msg tea.Msg, m *list.Model) tea.Cmd {
	return nil
}

func (d changeListDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	line := item.FilterValue()
	if titled, ok := item.(titledItem); ok {
		line = titled.Title()
	}

	if index == m.Index() {
		_, _ = fmt.Fprint(w, d.selectedStyle.Render("> "+line))
		return
	}

	_, _ = fmt.Fprint(w, d.normalStyle.Render("  "+line))
}
