package app

import (
	"fmt"
	"io"
	"unicode/utf8"

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

	maxWidth := m.Width() - 2
	if maxWidth < 4 {
		maxWidth = 4
	}
	line = truncateRunes(line, maxWidth)

	if index == m.Index() {
		row := fmt.Sprintf("> %-*s", maxWidth, line)
		_, _ = fmt.Fprint(w, d.selectedStyle.Render(row))
		return
	}

	row := fmt.Sprintf("  %-*s", maxWidth, line)
	_, _ = fmt.Fprint(w, d.normalStyle.Render(row))
}

func truncateRunes(s string, max int) string {
	if max <= 0 {
		return ""
	}
	if utf8.RuneCountInString(s) <= max {
		return s
	}

	r := []rune(s)
	if max < 2 {
		return string(r[:max])
	}

	return string(r[:max-1]) + "…"
}
