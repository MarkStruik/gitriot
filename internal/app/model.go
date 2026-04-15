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
	RepoPath     string
	Theme        theme.FileTheme
	RecentWindow time.Duration
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
	recentCommits []model.RepoCommit
	recentFiles   []model.CommitFile
	recentVisible []model.CommitFile
	recentWindow  time.Duration
	showRecent    bool
	recentLoaded  bool
	showHunksOnly bool

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

	requestID       int
	lastSelID       string
	activeRef       string
	lastFingerprint string
}

type changeListItem struct {
	change model.ChangeItem
}

func (c changeListItem) Title() string       { return fmt.Sprintf("[%s] %s", c.change.Type, c.change.Path) }
func (c changeListItem) Description() string { return c.change.ScopeLabel() }
func (c changeListItem) FilterValue() string { return c.change.ScopeLabel() + "/" + c.change.Path }

type recentFileListItem struct {
	file model.CommitFile
}

func (r recentFileListItem) Title() string {
	scope := r.file.Scope
	if r.file.IsRoot {
		scope = "root"
	}
	return fmt.Sprintf("[%s %s] %s", scope, shortHash(r.file.CommitHash), r.file.Path)
}

func (r recentFileListItem) Description() string { return r.file.Subject }
func (r recentFileListItem) FilterValue() string {
	return strings.ToLower(r.file.Scope + "/" + r.file.Path + " " + r.file.Subject)
}

type indexLoadedMsg struct {
	requestID   int
	branch      string
	result      git.IndexResult
	fingerprint string
	err         error
}

type diffLoadedMsg struct {
	requestID int
	view      string
	isBinary  bool
	empty     bool
	err       error
}

type refreshTickMsg time.Time
type fingerprintCheckedMsg struct {
	fingerprint string
	err         error
}

type recentLoadedMsg struct {
	requestID int
	result    git.RecentCommitResult
	err       error
}

type keyMap struct {
	Quit            key.Binding
	Refresh         key.Binding
	FocusSwitch     key.Binding
	Open            key.Binding
	FilterStaged    key.Binding
	FilterUnstaged  key.Binding
	FilterUntracked key.Binding
	FilterSubmodule key.Binding
	ToggleHunksOnly key.Binding
	Search          key.Binding
	ToggleRecent    key.Binding
	CloseSearch     key.Binding
	Up              key.Binding
	Down            key.Binding
	PageDown        key.Binding
	PageUp          key.Binding
}

func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.FocusSwitch, k.ToggleRecent, k.ToggleHunksOnly, k.Search, k.Quit}
}

func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.PageUp, k.PageDown, k.FocusSwitch},
		{k.FilterStaged, k.FilterUnstaged, k.FilterUntracked, k.FilterSubmodule, k.ToggleHunksOnly},
		{k.ToggleRecent, k.Search, k.CloseSearch, k.Refresh, k.Quit},
	}
}

var keys = keyMap{
	Quit:            key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
	Refresh:         key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh")),
	FocusSwitch:     key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "switch pane")),
	Open:            key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "reload diff")),
	FilterStaged:    key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "toggle staged")),
	FilterUnstaged:  key.NewBinding(key.WithKeys("u"), key.WithHelp("u", "toggle unstaged")),
	FilterUntracked: key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "toggle untracked")),
	FilterSubmodule: key.NewBinding(key.WithKeys("m"), key.WithHelp("m", "toggle submodule")),
	ToggleHunksOnly: key.NewBinding(key.WithKeys("h"), key.WithHelp("h", "toggle hunks-only")),
	ToggleRecent:    key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "toggle commits")),
	Search:          key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "search")),
	CloseSearch:     key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "close search")),
	Up:              key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up")),
	Down:            key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down")),
	PageDown:        key.NewBinding(key.WithKeys("pgdown"), key.WithHelp("pgdn", "scroll down")),
	PageUp:          key.NewBinding(key.WithKeys("pgup"), key.WithHelp("pgup", "scroll up")),
}

func NewModel(opt Option) Model {
	runner := git.NewCLIRunner()
	delegate := newChangeListDelegate(
		lipgloss.NewStyle().Foreground(lipgloss.Color(opt.Theme.Colors.Fg)),
		lipgloss.NewStyle().Foreground(lipgloss.Color(opt.Theme.Colors.Accent)).Bold(true),
	)
	changes := list.New(nil, delegate, 0, 0)
	changes.Title = "Changes"
	changes.SetShowTitle(false)
	changes.SetShowStatusBar(false)
	changes.SetShowPagination(false)
	changes.SetShowHelp(false)
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
		repoPath:     filepath.Clean(opt.RepoPath),
		styles:       ui.NewStyles(opt.Theme),
		filters:      DefaultFilters(),
		runner:       runner,
		index:        git.NewRepositoryIndexer(runner),
		diffs:        git.NewDiffLoader(runner),
		changes:      changes,
		diff:         d,
		help:         h,
		search:       s,
		focus:        focusChanges,
		message:      "Loading repository status...",
		recentWindow: opt.RecentWindow,
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(m.loadIndexCmd(), m.refreshTickCmd())
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
				m.applyCurrentList()
				return m, nil
			case keyMsg.Type == tea.KeyEnter:
				m.focus = focusChanges
				m.search.Blur()
				m.applyCurrentList()
				return m, nil
			default:
				m.filters.Query = m.search.Value()
				m.applyCurrentList()
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
			return m, m.refreshTickCmd()
		}
		return m, m.checkFingerprintCmd()
	case fingerprintCheckedMsg:
		if msg.err != nil {
			m.warn = append(m.warn, "background refresh failed: "+msg.err.Error())
			return m, m.refreshTickCmd()
		}
		if msg.fingerprint == m.lastFingerprint {
			return m, m.refreshTickCmd()
		}
		return m, tea.Batch(m.loadIndexCmd(), m.refreshTickCmd())
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
		m.lastFingerprint = msg.fingerprint
		m.allItems = msg.result.Items
		if len(m.allItems) == 0 && m.recentWindow > 0 {
			m.showRecent = true
		}
		if !m.showRecent {
			m.recentVisible = nil
		}
		m.applyCurrentList()
		if m.showRecent {
			m.diff.SetContent(m.renderRecentSummary())
			m.activeRef = "recent snapshot"
			if m.recentLoaded {
				m.message = fmt.Sprintf("Loaded %d files from recent commits", len(m.recentVisible))
				if cmd := m.autoLoadSelectedDiff(); cmd != nil {
					return m, tea.Batch(cmd, m.refreshTickCmd())
				}
			} else {
				m.message = "Loading recent commits..."
				return m, tea.Batch(m.loadRecentCmd(), m.refreshTickCmd())
			}
		} else {
			m.activeRef = ""
			m.message = fmt.Sprintf("Loaded %d changes", len(m.allItems))
			if cmd := m.autoLoadSelectedDiff(); cmd != nil {
				return m, tea.Batch(cmd, m.refreshTickCmd())
			}
		}
		return m, m.refreshTickCmd()
	case recentLoadedMsg:
		if msg.requestID != m.requestID {
			return m, nil
		}
		if msg.err != nil {
			m.warn = append(m.warn, "recent commits unavailable: "+msg.err.Error())
			m.message = "Unable to load recent commits"
			return m, nil
		}
		m.recentLoaded = true
		m.recentCommits = msg.result.Commits
		m.recentFiles = msg.result.Files
		m.warn = append(m.warn, msg.result.Warnings...)
		m.applyCurrentList()
		m.diff.SetContent(m.renderRecentSummary())
		m.message = fmt.Sprintf("Loaded %d files from recent commits", len(m.recentVisible))
		if cmd := m.autoLoadSelectedDiff(); cmd != nil {
			return m, cmd
		}
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

		m.diff.SetContent(msg.view)
		if msg.isBinary {
			m.message = "Binary diff loaded"
		} else if msg.empty {
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
		case key.Matches(msg, keys.ToggleRecent):
			if m.recentWindow <= 0 {
				m.message = "Recent commit view disabled; use --recent-window"
				return m, nil
			}
			m.showRecent = !m.showRecent
			if m.showRecent {
				m.applyCurrentList()
				m.diff.SetContent(m.renderRecentSummary())
				m.activeRef = "recent snapshot"
				if !m.recentLoaded {
					m.message = "Loading recent commits..."
					return m, m.loadRecentCmd()
				}
				m.message = fmt.Sprintf("Showing %d recent commit files", len(m.recentVisible))
				if cmd := m.autoLoadSelectedDiff(); cmd != nil {
					return m, cmd
				}
			} else {
				m.applyCurrentList()
				m.activeRef = ""
				m.message = "Showing diff view"
				if cmd := m.autoLoadSelectedDiff(); cmd != nil {
					return m, cmd
				}
			}
			return m, nil
		case key.Matches(msg, keys.ToggleHunksOnly):
			m.showHunksOnly = !m.showHunksOnly
			if m.showHunksOnly {
				m.message = "Showing changed hunks with context"
			} else {
				m.message = "Showing full file"
			}
			if cmd := m.autoLoadSelectedDiff(); cmd != nil {
				return m, cmd
			}
			return m, nil
		case key.Matches(msg, keys.Refresh):
			return m, m.loadIndexCmd()
		case key.Matches(msg, keys.FilterStaged):
			m.filters.ShowStaged = !m.filters.ShowStaged
			m.applyCurrentList()
			return m, nil
		case key.Matches(msg, keys.FilterUnstaged):
			m.filters.ShowUnstaged = !m.filters.ShowUnstaged
			m.applyCurrentList()
			return m, nil
		case key.Matches(msg, keys.FilterUntracked):
			m.filters.ShowUntracked = !m.filters.ShowUntracked
			m.applyCurrentList()
			return m, nil
		case key.Matches(msg, keys.FilterSubmodule):
			m.filters.ShowSubmodule = !m.filters.ShowSubmodule
			m.applyCurrentList()
			return m, nil
		case key.Matches(msg, keys.Open):
			if m.showRecent {
				file := m.selectedRecentFile()
				if file == nil {
					return m, nil
				}
				return m, m.loadCommitDiffCmd(*file)
			}
			item := m.selectedItem()
			if item == nil {
				return m, nil
			}
			return m, m.loadDiffCmd(*item)
		}
	}

	var cmd tea.Cmd
	if m.focus == focusChanges {
		before := m.currentSelectionID()
		m.changes, cmd = m.changes.Update(msg)
		after := m.currentSelectionID()
		if after != "" && after != before {
			autoCmd := m.autoLoadSelectedDiff()
			if autoCmd != nil {
				if cmd != nil {
					return m, tea.Batch(cmd, autoCmd)
				}
				return m, autoCmd
			}
		}
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

	return m.styles.Frame.Width(m.width).Height(m.height).MaxWidth(m.width).MaxHeight(m.height).Render(base)
}

func (m *Model) resize() {
	if m.width <= 0 || m.height <= 0 {
		return
	}

	leftWidth, rightWidth, paneHeight := paneDimensions(m.width, m.height, m.focus == focusSearch)

	m.changes.SetSize(leftWidth-4, paneHeight-2)
	m.diff.Width = rightWidth - 4
	m.diff.Height = paneHeight - 2
}

func (m *Model) rotateFocus() {
	if m.focus == focusChanges {
		m.focus = focusDiff
		return
	}
	m.focus = focusChanges
}

func (m *Model) renderTopBar() string {
	filters := fmt.Sprintf("S:%t U:%t N:%t M:%t H:%t", m.filters.ShowStaged, m.filters.ShowUnstaged, m.filters.ShowUntracked, m.filters.ShowSubmodule, m.showHunksOnly)
	visible, total := len(m.filteredItems), len(m.allItems)
	if m.showRecent {
		visible, total = len(m.recentVisible), len(m.recentFiles)
	}
	status := fmt.Sprintf("repo: %s | branch: %s | visible: %d/%d | %s", m.repoPath, valueOr(m.branch, "?"), visible, total, filters)
	if m.recentWindow > 0 {
		status += fmt.Sprintf(" | recent: %d (%s)", len(m.recentCommits), formatDuration(m.recentWindow))
	}
	if len(m.warn) > 0 {
		status += fmt.Sprintf(" | warnings: %d", len(m.warn))
	}
	status = truncateText(status, maxInt(m.width-2, 10))

	return m.styles.Status.Width(m.width).Render(status)
}

func (m *Model) renderChangesPane() string {
	titleText := "Changes"
	if m.showRecent {
		titleText = "Commit Files"
	}
	title := m.styles.Title.Render(titleText)
	if m.loading {
		title = title + " " + m.styles.Muted.Render("(loading)")
	}

	pane := m.styles.Pane
	if m.focus == focusChanges {
		pane = m.styles.PaneActive
	}

	body := m.changes.View()
	if m.showRecent && len(m.recentVisible) == 0 {
		body = m.styles.Muted.Render("No files found in recent commits")
	} else if !m.showRecent && len(m.filteredItems) == 0 {
		body = m.styles.Muted.Render("No changes match filters")
	}

	leftWidth, _, paneHeight := paneDimensions(m.width, m.height, m.focus == focusSearch)
	return pane.Width(leftWidth).Height(paneHeight).Render(lipgloss.JoinVertical(lipgloss.Left, title, body))
}

func (m *Model) renderDiffPane() string {
	titleText := "Diff"
	if m.showRecent {
		titleText = "Commit Details"
	}
	if m.activeRef != "" {
		titleText = titleText + " - " + truncateText(m.activeRef, maxInt(m.width/2, 24))
	}
	title := m.styles.Title.Render(titleText)
	pane := m.styles.Pane
	if m.focus == focusDiff {
		pane = m.styles.PaneActive
	}

	body := m.diff.View()

	_, rightWidth, paneHeight := paneDimensions(m.width, m.height, m.focus == focusSearch)
	return pane.Width(rightWidth).Height(paneHeight).Render(lipgloss.JoinVertical(lipgloss.Left, title, body))
}

func (m *Model) renderBottomBar() string {
	msg := m.message
	if msg == "" {
		msg = "Ready"
	}
	line := m.styles.Muted.Render(msg)
	helpLine := "tab switch pane · c commits · h hunks · / search · r refresh · q quit"
	helpLine = truncateText(helpLine, maxInt(m.width-2, 10))
	return lipgloss.JoinVertical(lipgloss.Left, line, m.styles.Muted.Render(helpLine))
}

func (m *Model) loadIndexCmd() tea.Cmd {
	m.loading = true
	m.requestID++
	m.recentLoaded = false
	requestID := m.requestID
	repoPath := m.repoPath

	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
		defer cancel()

		branch, branchErr := git.CurrentBranch(ctx, m.runner, repoPath)
		fingerprint, fingerprintErr := git.WorkingTreeFingerprint(ctx, m.runner, repoPath)
		result, err := m.index.IndexAll(ctx, repoPath)
		if branchErr != nil && err == nil {
			result.Warnings = append(result.Warnings, branchErr.Error())
		}
		if fingerprintErr != nil && err == nil {
			result.Warnings = append(result.Warnings, "fingerprint unavailable: "+fingerprintErr.Error())
		}

		return indexLoadedMsg{requestID: requestID, branch: branch, result: result, fingerprint: fingerprint, err: err}
	}
}

func (m *Model) loadDiffCmd(item model.ChangeItem) tea.Cmd {
	m.requestID++
	requestID := m.requestID
	activeRef := item.ScopeLabel() + "/" + item.Path
	m.activeRef = activeRef
	req := model.DiffRequest{
		RepoRoot:      m.repoPath,
		Path:          item.Path,
		SubmodulePath: item.SubmodulePath,
		Mode:          diffModeForChange(item),
	}

	m.diff.SetContent("Loading diff...")
	showHunksOnly := m.showHunksOnly
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
		defer cancel()

		result, err := m.diffs.Load(ctx, req)
		if err != nil {
			return diffLoadedMsg{requestID: requestID, err: err}
		}

		if result.IsBinary {
			return diffLoadedMsg{requestID: requestID, view: "Binary file diff", isBinary: true, empty: result.Empty}
		}

		full, fullErr := m.diffs.LoadWorkingFile(req)
		if fullErr != nil {
			return diffLoadedMsg{requestID: requestID, err: fullErr}
		}

		view := "Path: " + activeRef + "\n\n" + renderFileWithHunks(full, git.ParseChangedLineRangesFromPatch(result.Patch), showHunksOnly, 5)
		return diffLoadedMsg{requestID: requestID, view: view, empty: result.Empty}
	}
}

func (m *Model) refreshTickCmd() tea.Cmd {
	return tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
		return refreshTickMsg(t)
	})
}

func (m *Model) checkFingerprintCmd() tea.Cmd {
	repoPath := m.repoPath
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		fp, err := git.WorkingTreeFingerprint(ctx, m.runner, repoPath)
		return fingerprintCheckedMsg{fingerprint: fp, err: err}
	}
}

func (m *Model) loadRecentCmd() tea.Cmd {
	if m.recentWindow <= 0 {
		return nil
	}
	requestID := m.requestID
	repoPath := m.repoPath
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
		defer cancel()

		res, err := git.CollectRecentCommits(ctx, m.runner, repoPath, m.recentWindow)
		return recentLoadedMsg{requestID: requestID, result: res, err: err}
	}
}

func (m *Model) applyCurrentList() {
	if m.showRecent {
		m.applyRecentFilesToList()
		return
	}
	m.applyFiltersToList()
}

func (m *Model) applyFiltersToList() {
	m.filteredItems = ApplyFilters(m.allItems, m.filters)
	items := make([]list.Item, 0, len(m.filteredItems))
	for _, change := range m.filteredItems {
		items = append(items, changeListItem{change: change})
	}
	m.changes.SetItems(items)
	m.lastSelID = ""
}

func (m *Model) applyRecentFilesToList() {
	query := strings.ToLower(strings.TrimSpace(m.filters.Query))
	filtered := make([]model.CommitFile, 0, len(m.recentFiles))
	for _, file := range m.recentFiles {
		if !m.filters.ShowSubmodule && !file.IsRoot {
			continue
		}
		if query != "" {
			haystack := strings.ToLower(file.Scope + "/" + file.Path + " " + file.Subject + " " + file.Author)
			if !strings.Contains(haystack, query) {
				continue
			}
		}
		filtered = append(filtered, file)
	}
	m.recentVisible = filtered

	items := make([]list.Item, 0, len(filtered))
	for _, file := range filtered {
		items = append(items, recentFileListItem{file: file})
	}
	m.changes.SetItems(items)
	m.lastSelID = ""
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

func (m *Model) selectedRecentFile() *model.CommitFile {
	sel := m.changes.SelectedItem()
	if sel == nil {
		return nil
	}

	v, ok := sel.(recentFileListItem)
	if !ok {
		return nil
	}

	copy := v.file
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

func (m *Model) renderRecentSummary() string {
	if m.recentWindow <= 0 {
		return m.styles.Muted.Render("Recent commit view is disabled. Start with --recent-window (example: --recent-window 2h).")
	}

	if len(m.recentCommits) == 0 {
		return m.styles.Muted.Render("No commits found in the selected window.")
	}

	b := strings.Builder{}
	b.WriteString(m.styles.Muted.Render("Recent commit snapshot. Select a file on the left to auto-load its commit diff."))
	b.WriteByte('\n')
	b.WriteByte('\n')

	for _, commit := range m.recentCommits {
		scope := commit.Scope
		if commit.IsRoot {
			scope = "root"
		}

		header := fmt.Sprintf("[%s] %s  %s  %s", scope, shortHash(commit.Hash), commit.When.Local().Format(time.RFC3339), commit.Author)
		b.WriteString(m.styles.DiffMeta.Render(header))
		b.WriteByte('\n')
		b.WriteString(m.styles.DiffNormal.Render("  " + commit.Subject))
		b.WriteByte('\n')
		b.WriteByte('\n')
	}

	return strings.TrimRight(b.String(), "\n")
}

func (m *Model) loadCommitDiffCmd(file model.CommitFile) tea.Cmd {
	m.requestID++
	requestID := m.requestID
	scope := file.Scope
	if file.IsRoot {
		scope = "root"
	}
	activeRef := fmt.Sprintf("%s/%s @%s", scope, file.Path, shortHash(file.CommitHash))
	m.activeRef = activeRef
	req := git.CommitDiffRequest{
		RepoRoot:      m.repoPath,
		SubmodulePath: file.SubmodulePath,
		CommitHash:    file.CommitHash,
		Path:          file.Path,
	}

	m.diff.SetContent("Loading commit diff...")
	showHunksOnly := m.showHunksOnly
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
		defer cancel()

		result, err := m.diffs.LoadCommit(ctx, req)
		if err != nil {
			return diffLoadedMsg{requestID: requestID, err: err}
		}
		if result.IsBinary {
			return diffLoadedMsg{requestID: requestID, view: "Binary file diff", isBinary: true, empty: result.Empty}
		}

		full, fullErr := m.diffs.LoadCommitFile(ctx, req)
		if fullErr != nil {
			return diffLoadedMsg{requestID: requestID, err: fullErr}
		}

		view := "Path: " + activeRef + "\n\n" + renderFileWithHunks(full, git.ParseChangedLineRangesFromPatch(result.Patch), showHunksOnly, 5)
		return diffLoadedMsg{requestID: requestID, view: view, empty: result.Empty}
	}
}

func (m *Model) autoLoadSelectedDiff() tea.Cmd {
	selectionID := m.currentSelectionID()
	if selectionID == "" {
		return nil
	}
	if selectionID == m.lastSelID {
		return nil
	}
	m.lastSelID = selectionID

	if m.showRecent {
		file := m.selectedRecentFile()
		if file == nil {
			return nil
		}
		return m.loadCommitDiffCmd(*file)
	}

	item := m.selectedItem()
	if item == nil {
		return nil
	}
	return m.loadDiffCmd(*item)
}

func (m *Model) currentSelectionID() string {
	if m.showRecent {
		file := m.selectedRecentFile()
		if file == nil {
			return ""
		}
		return file.Scope + "|" + file.CommitHash + "|" + file.Path
	}

	item := m.selectedItem()
	if item == nil {
		return ""
	}
	return item.ScopeLabel() + "|" + string(item.Type) + "|" + item.Path
}

func shortHash(hash string) string {
	if len(hash) <= 8 {
		return hash
	}

	return hash[:8]
}

func formatDuration(d time.Duration) string {
	if d%time.Hour == 0 {
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	if d%time.Minute == 0 {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}

	return d.String()
}

func paneDimensions(totalWidth int, totalHeight int, searchVisible bool) (leftWidth int, rightWidth int, paneHeight int) {
	if totalWidth < 60 {
		leftWidth = maxInt(totalWidth/2, 20)
		rightWidth = maxInt(totalWidth-leftWidth-1, 20)
	} else {
		leftWidth = totalWidth / 3
		if leftWidth < 28 {
			leftWidth = 28
		}
		rightWidth = totalWidth - leftWidth - 1
		if rightWidth < 30 {
			rightWidth = 30
			leftWidth = totalWidth - rightWidth - 1
			if leftWidth < 20 {
				leftWidth = 20
				rightWidth = totalWidth - leftWidth - 1
			}
		}
	}

	if leftWidth < 20 {
		leftWidth = 20
	}
	if rightWidth < 20 {
		rightWidth = 20
	}
	if leftWidth+rightWidth+1 > totalWidth {
		over := leftWidth + rightWidth + 1 - totalWidth
		rightWidth -= over
		if rightWidth < 20 {
			rightWidth = 20
		}
	}

	headerHeight := 1
	footerHeight := 2
	if searchVisible {
		footerHeight++
	}
	paneHeight = totalHeight - headerHeight - footerHeight
	if paneHeight < 5 {
		paneHeight = 5
	}

	return leftWidth, rightWidth, paneHeight
}

func truncateText(input string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}

	r := []rune(input)
	if len(r) <= maxLen {
		return input
	}

	if maxLen < 2 {
		return string(r[:maxLen])
	}

	return string(r[:maxLen-1]) + "…"
}

func renderFileWithHunks(fullContent string, changed []git.LineRange, hunksOnly bool, contextLines int) string {
	lines := strings.Split(fullContent, "\n")
	if !hunksOnly || len(changed) == 0 {
		return renderNumberedLines(lines, nil)
	}

	keep := make([]bool, len(lines))
	for _, h := range changed {
		start := h.Start - contextLines
		end := h.End + contextLines
		if start < 1 {
			start = 1
		}
		if end > len(lines) {
			end = len(lines)
		}
		for i := start; i <= end; i++ {
			keep[i-1] = true
		}
	}

	return renderNumberedLines(lines, keep)
}

func renderNumberedLines(lines []string, keep []bool) string {
	b := strings.Builder{}
	skipped := false
	for i, line := range lines {
		if keep != nil && !keep[i] {
			skipped = true
			continue
		}
		if skipped {
			b.WriteString("...\n")
			skipped = false
		}
		b.WriteString(fmt.Sprintf("%6d | %s\n", i+1, line))
	}

	if b.Len() == 0 {
		return "No lines to display"
	}

	return strings.TrimRight(b.String(), "\n")
}
