package app

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"gitriot/internal/git"
	"gitriot/internal/model"
	"gitriot/internal/theme"
	"gitriot/internal/ui"
)

type paneFocus int

const (
	focusChanges paneFocus = iota
	focusDiff
	focusSearch
)

type Option struct {
	RepoPath string
	Theme    theme.FileTheme
}

type Model struct {
	repoPath string
	branch   string
	styles   ui.Styles
	filters  Filters

	runner *git.CLIRunner
	index  *git.RepositoryIndexer
	diffs  *git.DiffLoader

	allItems      []model.ChangeItem
	filteredItems []model.ChangeItem

	changes list.Model
	diff    viewport.Model
	help    help.Model
	search  textinput.Model
	focus   paneFocus

	width  int
	height int
	ready  bool

	loading bool
	message string
	warn    []string

	requestID int
}

type changeListItem struct {
	change model.ChangeItem
}

func (c changeListItem) Title() string       { return fmt.Sprintf("[%s] %s", c.change.Type, c.change.Path) }
func (c changeListItem) Description() string { return c.change.ScopeLabel() }
func (c changeListItem) FilterValue() string { return c.change.ScopeLabel() + "/" + c.change.Path }

type indexLoadedMsg struct {
	requestID int
	branch    string
	result    git.IndexResult
	err       error
}

type diffLoadedMsg struct {
	requestID int
	result    model.DiffResult
	err       error
}

type refreshTickMsg time.Time

type keyMap struct {
	Quit            key.Binding
	Refresh         key.Binding
	FocusSwitch     key.Binding
	Open            key.Binding
	FilterStaged    key.Binding
	FilterUnstaged  key.Binding
	FilterUntracked key.Binding
	FilterSubmodule key.Binding
	Search          key.Binding
	CloseSearch     key.Binding
	Up              key.Binding
	Down            key.Binding
	PageDown        key.Binding
	PageUp          key.Binding
}

func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.FocusSwitch, k.Open, k.Search, k.Refresh, k.Quit}
}

func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.PageUp, k.PageDown, k.FocusSwitch},
		{k.FilterStaged, k.FilterUnstaged, k.FilterUntracked, k.FilterSubmodule},
		{k.Search, k.CloseSearch, k.Refresh, k.Quit},
	}
}

var keys = keyMap{
	Quit:            key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
	Refresh:         key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh")),
	FocusSwitch:     key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "switch pane")),
	Open:            key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "load diff")),
	FilterStaged:    key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "toggle staged")),
	FilterUnstaged:  key.NewBinding(key.WithKeys("u"), key.WithHelp("u", "toggle unstaged")),
	FilterUntracked: key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "toggle untracked")),
	FilterSubmodule: key.NewBinding(key.WithKeys("m"), key.WithHelp("m", "toggle submodule")),
	Search:          key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "search")),
	CloseSearch:     key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "close search")),
	Up:              key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up")),
	Down:            key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down")),
	PageDown:        key.NewBinding(key.WithKeys("pgdown"), key.WithHelp("pgdn", "scroll down")),
	PageUp:          key.NewBinding(key.WithKeys("pgup"), key.WithHelp("pgup", "scroll up")),
}

func NewModel(opt Option) Model {
	runner := git.NewCLIRunner()
	changes := list.New(nil, list.NewDefaultDelegate(), 0, 0)
	changes.Title = "Changes"
	changes.SetShowStatusBar(false)
	changes.SetFilteringEnabled(false)

	d := viewport.New(0, 0)
	d.SetContent("Select a file to load a diff.")

	s := textinput.New()
	s.Prompt = "search> "
	s.Placeholder = "path or submodule"
	s.Blur()

	h := help.New()
	h.ShowAll = false

	return Model{
		repoPath: filepath.Clean(opt.RepoPath),
		styles:   ui.NewStyles(opt.Theme),
		filters:  DefaultFilters(),
		runner:   runner,
		index:    git.NewRepositoryIndexer(runner),
		diffs:    git.NewDiffLoader(runner),
		changes:  changes,
		diff:     d,
		help:     h,
		search:   s,
		focus:    focusChanges,
		message:  "Loading repository status...",
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(m.loadIndexCmd(), m.periodicRefreshCmd())
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.focus == focusSearch {
		var cmd tea.Cmd
		m.search, cmd = m.search.Update(msg)
		if keyMsg, ok := msg.(tea.KeyMsg); ok {
			switch {
			case key.Matches(keyMsg, keys.CloseSearch):
				m.focus = focusChanges
				m.search.Blur()
				m.applyFiltersToList()
				return m, nil
			case keyMsg.Type == tea.KeyEnter:
				m.focus = focusChanges
				m.search.Blur()
				m.applyFiltersToList()
				return m, nil
			default:
				m.filters.Query = m.search.Value()
				m.applyFiltersToList()
				return m, cmd
			}
		}
		return m, cmd
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.ready = true
		m.width = msg.Width
		m.height = msg.Height
		m.resize()
		return m, nil
	case refreshTickMsg:
		if m.loading {
			return m, m.periodicRefreshCmd()
		}
		return m, tea.Batch(m.loadIndexCmd(), m.periodicRefreshCmd())
	case indexLoadedMsg:
		if msg.requestID != m.requestID {
			return m, nil
		}

		m.loading = false
		if msg.err != nil {
			m.message = "Indexing failed: " + msg.err.Error()
			return m, nil
		}

		m.branch = msg.branch
		m.warn = msg.result.Warnings
		m.allItems = msg.result.Items
		m.applyFiltersToList()
		m.message = fmt.Sprintf("Loaded %d changes", len(m.allItems))
		return m, nil
	case diffLoadedMsg:
		if msg.requestID != m.requestID {
			return m, nil
		}

		if msg.err != nil {
			m.diff.SetContent("Unable to load diff:\n" + msg.err.Error())
			m.message = "Diff loading failed"
			return m, nil
		}

		m.diff.SetContent(m.renderDiff(msg.result))
		if msg.result.IsBinary {
			m.message = "Binary diff loaded"
		} else if msg.result.Empty {
			m.message = "No diff output for selected item"
		} else {
			m.message = "Diff loaded"
		}
		return m, nil
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, keys.Quit):
			return m, tea.Quit
		case key.Matches(msg, keys.FocusSwitch):
			m.rotateFocus()
			return m, nil
		case key.Matches(msg, keys.Search):
			m.focus = focusSearch
			m.search.Focus()
			return m, textinput.Blink
		case key.Matches(msg, keys.Refresh):
			return m, m.loadIndexCmd()
		case key.Matches(msg, keys.FilterStaged):
			m.filters.ShowStaged = !m.filters.ShowStaged
			m.applyFiltersToList()
			return m, nil
		case key.Matches(msg, keys.FilterUnstaged):
			m.filters.ShowUnstaged = !m.filters.ShowUnstaged
			m.applyFiltersToList()
			return m, nil
		case key.Matches(msg, keys.FilterUntracked):
			m.filters.ShowUntracked = !m.filters.ShowUntracked
			m.applyFiltersToList()
			return m, nil
		case key.Matches(msg, keys.FilterSubmodule):
			m.filters.ShowSubmodule = !m.filters.ShowSubmodule
			m.applyFiltersToList()
			return m, nil
		case key.Matches(msg, keys.Open):
			item := m.selectedItem()
			if item == nil {
				return m, nil
			}
			return m, m.loadDiffCmd(*item)
		}
	}

	var cmd tea.Cmd
	if m.focus == focusChanges {
		m.changes, cmd = m.changes.Update(msg)
	} else {
		m.diff, cmd = m.diff.Update(msg)
	}

	return m, cmd
}

func (m Model) View() string {
	if !m.ready {
		return "Initializing GitRiot..."
	}

	top := m.renderTopBar()
	left := m.renderChangesPane()
	right := m.renderDiffPane()
	panes := lipgloss.JoinHorizontal(lipgloss.Top, left, right)
	bottom := m.renderBottomBar()

	base := lipgloss.JoinVertical(lipgloss.Left, top, panes, bottom)
	if m.focus == focusSearch {
		search := m.styles.SearchPrompt.Render("Search: ") + m.search.View()
		base = lipgloss.JoinVertical(lipgloss.Left, base, search)
	}

	return m.styles.Frame.Width(m.width).Height(m.height).Render(base)
}

func (m *Model) resize() {
	if m.width <= 0 || m.height <= 0 {
		return
	}

	contentHeight := maxInt(m.height-5, 6)
	leftWidth := maxInt(m.width/3, 30)
	rightWidth := maxInt(m.width-leftWidth-2, 40)

	m.changes.SetSize(leftWidth-4, contentHeight-2)
	m.diff.Width = rightWidth - 4
	m.diff.Height = contentHeight - 2
}

func (m *Model) rotateFocus() {
	if m.focus == focusChanges {
		m.focus = focusDiff
		return
	}
	m.focus = focusChanges
}

func (m *Model) renderTopBar() string {
	filters := fmt.Sprintf("S:%t U:%t N:%t M:%t", m.filters.ShowStaged, m.filters.ShowUnstaged, m.filters.ShowUntracked, m.filters.ShowSubmodule)
	status := fmt.Sprintf("repo: %s | branch: %s | visible: %d/%d | %s", m.repoPath, valueOr(m.branch, "?"), len(m.filteredItems), len(m.allItems), filters)
	if len(m.warn) > 0 {
		status += fmt.Sprintf(" | warnings: %d", len(m.warn))
	}

	return m.styles.Status.Width(m.width).Render(status)
}

func (m *Model) renderChangesPane() string {
	title := m.styles.Title.Render("Changes")
	if m.loading {
		title = title + " " + m.styles.Muted.Render("(loading)")
	}

	pane := m.styles.Pane
	if m.focus == focusChanges {
		pane = m.styles.PaneActive
	}

	body := m.changes.View()
	if len(m.filteredItems) == 0 {
		body = m.styles.Muted.Render("No changes match filters")
	}

	return pane.Width(maxInt(m.width/3, 30)).Height(maxInt(m.height-3, 8)).Render(lipgloss.JoinVertical(lipgloss.Left, title, body))
}

func (m *Model) renderDiffPane() string {
	title := m.styles.Title.Render("Diff")
	pane := m.styles.Pane
	if m.focus == focusDiff {
		pane = m.styles.PaneActive
	}

	return pane.Width(maxInt(m.width-maxInt(m.width/3, 30)-1, 40)).Height(maxInt(m.height-3, 8)).Render(lipgloss.JoinVertical(lipgloss.Left, title, m.diff.View()))
}

func (m *Model) renderBottomBar() string {
	msg := m.message
	if msg == "" {
		msg = "Ready"
	}
	line := m.styles.Muted.Render(msg)
	return lipgloss.JoinVertical(lipgloss.Left, line, m.help.View(keys))
}

func (m *Model) loadIndexCmd() tea.Cmd {
	m.loading = true
	m.requestID++
	requestID := m.requestID
	repoPath := m.repoPath

	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
		defer cancel()

		branch, branchErr := git.CurrentBranch(ctx, m.runner, repoPath)
		result, err := m.index.IndexAll(ctx, repoPath)
		if branchErr != nil && err == nil {
			result.Warnings = append(result.Warnings, branchErr.Error())
		}

		return indexLoadedMsg{requestID: requestID, branch: branch, result: result, err: err}
	}
}

func (m *Model) loadDiffCmd(item model.ChangeItem) tea.Cmd {
	m.requestID++
	requestID := m.requestID
	req := model.DiffRequest{
		RepoRoot:      m.repoPath,
		Path:          item.Path,
		SubmodulePath: item.SubmodulePath,
		Mode:          diffModeForChange(item),
	}

	m.diff.SetContent("Loading diff...")
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
		defer cancel()

		result, err := m.diffs.Load(ctx, req)
		return diffLoadedMsg{requestID: requestID, result: result, err: err}
	}
}

func (m *Model) periodicRefreshCmd() tea.Cmd {
	return tea.Tick(15*time.Second, func(t time.Time) tea.Msg {
		return refreshTickMsg(t)
	})
}

func (m *Model) applyFiltersToList() {
	m.filteredItems = ApplyFilters(m.allItems, m.filters)
	items := make([]list.Item, 0, len(m.filteredItems))
	for _, change := range m.filteredItems {
		items = append(items, changeListItem{change: change})
	}
	m.changes.SetItems(items)
}

func (m *Model) selectedItem() *model.ChangeItem {
	sel := m.changes.SelectedItem()
	if sel == nil {
		return nil
	}

	v, ok := sel.(changeListItem)
	if !ok {
		return nil
	}

	copy := v.change
	return &copy
}

func (m *Model) renderDiff(res model.DiffResult) string {
	if res.IsBinary {
		return m.styles.Muted.Render("Binary file diff")
	}
	if res.Empty {
		return m.styles.Muted.Render("No diff output")
	}

	lines := strings.Split(res.Patch, "\n")
	b := strings.Builder{}
	for _, line := range lines {
		styled := line
		switch {
		case strings.HasPrefix(line, "+++") || strings.HasPrefix(line, "---") || strings.HasPrefix(line, "diff --git"):
			styled = m.styles.DiffMeta.Render(line)
		case strings.HasPrefix(line, "@@"):
			styled = m.styles.Hunk.Render(line)
		case strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++"):
			styled = m.styles.Added.Render(line)
		case strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---"):
			styled = m.styles.Removed.Render(line)
		default:
			styled = m.styles.DiffNormal.Render(line)
		}

		b.WriteString(styled)
		b.WriteByte('\n')
	}

	return b.String()
}

func diffModeForChange(item model.ChangeItem) model.DiffMode {
	if item.Type == model.ChangeTypeStaged {
		return model.DiffModeStaged
	}

	return model.DiffModeUnstaged
}

func maxInt(a int, b int) int {
	if a > b {
		return a
	}

	return b
}

func valueOr(v string, fallback string) string {
	if strings.TrimSpace(v) == "" {
		return fallback
	}

	return v
}
