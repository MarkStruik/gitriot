package app

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
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
	scopeBranches map[string]string
	treeCollapsed map[string]bool

	diff   viewport.Model
	help   help.Model
	search textinput.Model
	focus  paneFocus

	treeRows     []treeRow
	selectedTree int
	leftOffset   int
	leftWidth    int
	leftHeight   int

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

type treeRow struct {
	id         string
	nodeID     string
	parentID   string
	depth      int
	kind       string
	text       string
	selectable bool
	change     *model.ChangeItem
	commitFile *model.CommitFile
}

const (
	treeKindScope = "scope"
	treeKindDir   = "dir"
	treeKindFile  = "file"
)

type treeBuildNode struct {
	Name     string
	Path     string
	Folders  map[string]*treeBuildNode
	Files    []model.ChangeItem
	IsRoot   bool
	ScopeKey string
}

type indexLoadedMsg struct {
	requestID   int
	branch      string
	result      git.IndexResult
	scopeBranch map[string]string
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
	requestID   int
	result      git.RecentCommitResult
	scopeBranch map[string]string
	err         error
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
	CollapseNode    key.Binding
	ExpandNode      key.Binding
	ToggleNode      key.Binding
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
	return []key.Binding{k.FocusSwitch, k.ToggleRecent, k.CollapseNode, k.ExpandNode, k.ToggleNode, k.Quit}
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
	CollapseNode:    key.NewBinding(key.WithKeys("left", "h"), key.WithHelp("←/h", "collapse")),
	ExpandNode:      key.NewBinding(key.WithKeys("right", "l"), key.WithHelp("→/l", "expand")),
	ToggleNode:      key.NewBinding(key.WithKeys("space"), key.WithHelp("space", "toggle")),
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
	d := viewport.New(0, 0)
	d.SetContent("Select a file to load a diff.")

	s := textinput.New()
	s.Prompt = "search> "
	s.Placeholder = "path or submodule"
	s.Blur()

	h := help.New()
	h.ShowAll = false

	return Model{
		repoPath:      filepath.Clean(opt.RepoPath),
		styles:        ui.NewStyles(opt.Theme),
		filters:       DefaultFilters(),
		runner:        runner,
		index:         git.NewRepositoryIndexer(runner),
		diffs:         git.NewDiffLoader(runner),
		diff:          d,
		help:          h,
		search:        s,
		focus:         focusChanges,
		message:       "Loading repository status...",
		recentWindow:  opt.RecentWindow,
		selectedTree:  -1,
		scopeBranches: map[string]string{},
		treeCollapsed: map[string]bool{},
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
		m.scopeBranches = copyStringMap(msg.scopeBranch)
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
		if len(msg.scopeBranch) > 0 {
			m.scopeBranches = copyStringMap(msg.scopeBranch)
		}
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
		case key.Matches(msg, keys.CollapseNode):
			if m.focus == focusChanges {
				if ok, preserve := m.collapseTreeAtSelection(); ok {
					m.applyCurrentListWithPreserve(preserve)
					if cmd := m.autoLoadSelectedDiff(); cmd != nil {
						return m, cmd
					}
				}
				return m, nil
			}
		case key.Matches(msg, keys.ExpandNode):
			if m.focus == focusChanges {
				if ok, preserve := m.expandTreeAtSelection(); ok {
					m.applyCurrentListWithPreserve(preserve)
					if cmd := m.autoLoadSelectedDiff(); cmd != nil {
						return m, cmd
					}
				}
				return m, nil
			}
		case key.Matches(msg, keys.ToggleNode):
			if m.focus == focusChanges {
				if ok, preserve := m.toggleTreeAtSelection(); ok {
					m.applyCurrentListWithPreserve(preserve)
					if cmd := m.autoLoadSelectedDiff(); cmd != nil {
						return m, cmd
					}
				}
				return m, nil
			}
		case key.Matches(msg, keys.ToggleHunksOnly):
			if m.focus != focusDiff {
				return m, nil
			}
			m.showHunksOnly = !m.showHunksOnly
			if m.showHunksOnly {
				m.message = "Showing changed hunks with context"
			} else {
				m.message = "Showing full file"
			}
			m.lastSelID = ""
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
		case key.Matches(msg, keys.Up):
			if m.focus == focusChanges {
				m.moveSelection(-1)
				if cmd := m.autoLoadSelectedDiff(); cmd != nil {
					return m, cmd
				}
				return m, nil
			}
		case key.Matches(msg, keys.Down):
			if m.focus == focusChanges {
				m.moveSelection(1)
				if cmd := m.autoLoadSelectedDiff(); cmd != nil {
					return m, cmd
				}
				return m, nil
			}
		case key.Matches(msg, keys.PageUp):
			if m.focus == focusChanges {
				m.pageSelection(-1)
				if cmd := m.autoLoadSelectedDiff(); cmd != nil {
					return m, cmd
				}
				return m, nil
			}
		case key.Matches(msg, keys.PageDown):
			if m.focus == focusChanges {
				m.pageSelection(1)
				if cmd := m.autoLoadSelectedDiff(); cmd != nil {
					return m, cmd
				}
				return m, nil
			}
		}
	}

	var cmd tea.Cmd
	if m.focus == focusDiff {
		m.diff, cmd = m.diff.Update(msg)
	}

	return m, cmd
}

func (m Model) View() string {
	if !m.ready {
		return "Initializing GitRiot..."
	}

	leftWidth, rightWidth, paneHeight := paneDimensions(m.width, m.height, m.focus == focusSearch)
	top := m.renderTopBar()
	left := m.renderChangesPane(leftWidth, paneHeight)
	right := m.renderDiffPane(rightWidth, paneHeight)
	sep := renderVerticalSep(paneHeight)
	panes := lipgloss.JoinHorizontal(lipgloss.Top, left, sep, right)
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

	contentHeight := maxInt(paneHeight-4, 1)
	m.leftWidth = maxInt(leftWidth-4, 8)
	m.leftHeight = maxInt(contentHeight, 1)
	m.diff.Width = rightWidth - 4
	m.diff.Height = contentHeight
	m.ensureSelectionVisible()
}

func (m *Model) rotateFocus() {
	if m.focus == focusChanges {
		m.focus = focusDiff
		return
	}
	m.focus = focusChanges
}

func (m *Model) renderTopBar() string {
	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#8ec0ff")).Render("GitRiot")
	return m.styles.Status.Width(m.width).Render(title)
}

func (m *Model) renderChangesPane(width int, height int) string {
	titleText := "Changes"
	if m.showRecent {
		titleText = "Commit Files"
	}
	titlePrefix := "  "
	if m.focus == focusChanges {
		titlePrefix = "* "
	}
	title := m.styles.Title.Render(titlePrefix + titleText)
	if m.loading {
		title = title + " " + m.styles.Muted.Render("(loading)")
	}

	body := m.renderTreePanel()
	if m.showRecent && len(m.recentVisible) == 0 {
		body = m.styles.Muted.Render("No files found in recent commits")
	} else if !m.showRecent && len(m.filteredItems) == 0 {
		body = m.styles.Muted.Render("No changes match filters")
	}

	panel := lipgloss.JoinVertical(lipgloss.Left, title, body)
	bg := lipgloss.Color("#22283a")
	if m.focus == focusChanges {
		bg = lipgloss.Color("#2a3147")
	}
	return lipgloss.NewStyle().Background(bg).Width(width).Height(maxInt(height, 3)).Render(panel)
}

func (m *Model) renderDiffPane(width int, height int) string {
	titleText := "Diff"
	if m.showRecent {
		titleText = "Commit Details"
	}
	if m.activeRef != "" {
		titleText = titleText + " - " + truncateText(m.activeRef, maxInt(m.width/2, 24))
	}
	titlePrefix := "  "
	if m.focus == focusDiff {
		titlePrefix = "* "
	}
	title := m.styles.Title.Render(titlePrefix + titleText)

	body := m.diff.View()
	panel := lipgloss.JoinVertical(lipgloss.Left, title, body)
	bg := lipgloss.Color("#2f354b")
	if m.focus == focusDiff {
		bg = lipgloss.Color("#353d55")
	}
	return lipgloss.NewStyle().Background(bg).Width(width).Height(maxInt(height, 3)).Render(panel)
}

func (m *Model) renderBottomBar() string {
	msg := m.message
	if msg == "" {
		msg = "Ready"
	}
	line := m.styles.Muted.Render(msg)
	treeStyle := m.styles.Muted
	diffStyle := m.styles.Muted
	if m.focus == focusChanges {
		treeStyle = m.styles.Title
	} else if m.focus == focusDiff {
		diffStyle = m.styles.Title
	}

	treeSeg := treeStyle.Render("󰙅 TREE  ←/h collapse  →/l expand  ␠ toggle")
	diffSeg := diffStyle.Render("󰈙 DIFF  h hunks/full")
	globalSeg := m.styles.Muted.Render("󰌌 GLOBAL  tab pane  c commits  / search  r refresh  q quit")
	helpLine := treeSeg + m.styles.Muted.Render("   •   ") + diffSeg + m.styles.Muted.Render("   •   ") + globalSeg
	helpLine = truncateText(helpLine, maxInt(m.width-2, 10))
	return lipgloss.JoinVertical(lipgloss.Left, line, helpLine)
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
		scopeBranches := map[string]string{"root": valueOr(branch, "?")}
		submodules := collectSubmodulePathsFromChanges(result.Items)
		for _, submodule := range submodules {
			subDir := gitSubmoduleDir(repoPath, submodule)
			subBranch, subErr := git.CurrentBranch(ctx, m.runner, subDir)
			if subErr != nil {
				scopeBranches[submodule] = "?"
				if err == nil {
					result.Warnings = append(result.Warnings, fmt.Sprintf("branch unavailable for %s: %v", submodule, subErr))
				}
				continue
			}
			scopeBranches[submodule] = valueOr(subBranch, "?")
		}
		if branchErr != nil && err == nil {
			result.Warnings = append(result.Warnings, branchErr.Error())
		}
		if fingerprintErr != nil && err == nil {
			result.Warnings = append(result.Warnings, "fingerprint unavailable: "+fingerprintErr.Error())
		}

		return indexLoadedMsg{requestID: requestID, branch: branch, result: result, scopeBranch: scopeBranches, fingerprint: fingerprint, err: err}
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
			fallback := "Path: " + activeRef + "\n\n" + m.renderDiff(result)
			return diffLoadedMsg{requestID: requestID, view: fallback, empty: result.Empty}
		}

		view := "Path: " + activeRef + "\n\n" + renderFileWithHunks(req.Path, full, git.ParseChangedLineRangesFromPatch(result.Patch), showHunksOnly, 5)
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
		scopeBranches := copyStringMap(m.scopeBranches)
		submodules := collectSubmodulePathsFromCommitFiles(res.Files)
		for _, submodule := range submodules {
			if _, ok := scopeBranches[submodule]; ok {
				continue
			}
			subDir := gitSubmoduleDir(repoPath, submodule)
			subBranch, subErr := git.CurrentBranch(ctx, m.runner, subDir)
			if subErr != nil {
				scopeBranches[submodule] = "?"
				continue
			}
			scopeBranches[submodule] = valueOr(subBranch, "?")
		}
		return recentLoadedMsg{requestID: requestID, result: res, scopeBranch: scopeBranches, err: err}
	}
}

func (m *Model) applyCurrentList() {
	m.applyCurrentListWithPreserve("")
}

func (m *Model) applyCurrentListWithPreserve(previousSelectionID string) {
	if m.showRecent {
		m.applyRecentFilesToList(previousSelectionID)
		return
	}
	m.applyFiltersToList(previousSelectionID)
}

func (m *Model) applyFiltersToList(previousSelectionID string) {
	m.filteredItems = ApplyFilters(m.allItems, m.filters)
	m.treeRows = buildChangeTreeRows(m.filteredItems, m.scopeBranches, m.treeCollapsed)
	m.restoreSelection(previousSelectionID)
}

func (m *Model) applyRecentFilesToList(previousSelectionID string) {
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
	m.treeRows = buildRecentTreeRows(filtered, m.scopeBranches, m.treeCollapsed)
	m.restoreSelection(previousSelectionID)
}

func (m *Model) selectedItem() *model.ChangeItem {
	if m.selectedTree < 0 || m.selectedTree >= len(m.treeRows) {
		return nil
	}
	row := m.treeRows[m.selectedTree]
	if row.change == nil {
		return nil
	}
	copy := *row.change
	return &copy
}

func (m *Model) selectedRecentFile() *model.CommitFile {
	if m.selectedTree < 0 || m.selectedTree >= len(m.treeRows) {
		return nil
	}
	row := m.treeRows[m.selectedTree]
	if row.commitFile == nil {
		return nil
	}
	copy := *row.commitFile
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
			fallback := "Path: " + activeRef + "\n\n" + m.renderDiff(result)
			return diffLoadedMsg{requestID: requestID, view: fallback, empty: result.Empty}
		}

		view := "Path: " + activeRef + "\n\n" + renderFileWithHunks(file.Path, full, git.ParseChangedLineRangesFromPatch(result.Patch), showHunksOnly, 5)
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
	if m.selectedTree < 0 || m.selectedTree >= len(m.treeRows) {
		return ""
	}
	return m.treeRows[m.selectedTree].id
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

func renderFileWithHunks(path string, fullContent string, changed []git.LineRange, hunksOnly bool, contextLines int) string {
	highlighted := ui.HighlightForPath(path, fullContent)
	lines := strings.Split(highlighted, "\n")
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

func (m *Model) renderTreePanel() string {
	if m.leftHeight <= 0 {
		return ""
	}
	bg := lipgloss.Color("#22283a")
	selectedBg := lipgloss.Color("#101725")
	if m.focus == focusChanges {
		bg = lipgloss.Color("#2a3147")
		selectedBg = lipgloss.Color("#0d1424")
	}
	normalStyle := lipgloss.NewStyle().Background(bg)
	selectedStyle := lipgloss.NewStyle().Background(selectedBg).Foreground(lipgloss.Color("#9cc8ff")).Bold(true)
	folderStyle := lipgloss.NewStyle().Background(bg).Foreground(lipgloss.Color("#c9d4ee"))
	fileStyle := lipgloss.NewStyle().Background(bg).Foreground(lipgloss.Color("#e8edf8"))

	start := m.leftOffset
	if start < 0 {
		start = 0
	}
	if len(m.treeRows) == 0 {
		start = 0
	}
	if len(m.treeRows) > 0 && start >= len(m.treeRows) {
		start = len(m.treeRows) - 1
	}
	end := start + m.leftHeight
	if end > len(m.treeRows) {
		end = len(m.treeRows)
	}

	b := strings.Builder{}
	renderWidth := maxInt(m.leftWidth, 8)
	printed := 0
	for i := start; i < end; i++ {
		row := m.treeRows[i]
		prefix := "  "
		if i == m.selectedTree {
			prefix = "❯ "
		}
		line := truncateText(prefix+row.text, maxInt(renderWidth, 4))
		if i == m.selectedTree {
			b.WriteString(selectedStyle.Width(renderWidth).Render(line))
		} else if row.kind == treeKindScope || row.kind == treeKindDir {
			b.WriteString(folderStyle.Width(renderWidth).Render(line))
		} else {
			b.WriteString(fileStyle.Width(renderWidth).Render(line))
		}
		printed++
		if printed < m.leftHeight {
			b.WriteByte('\n')
		}
	}
	for printed < m.leftHeight {
		if printed > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(normalStyle.Width(renderWidth).Render(""))
		printed++
	}

	return b.String()
}

func (m *Model) resetSelection() {
	m.lastSelID = ""
	m.leftOffset = 0
	m.selectedTree = -1
	for i, row := range m.treeRows {
		if row.selectable {
			m.selectedTree = i
			break
		}
	}
	m.ensureSelectionVisible()
}

func (m *Model) restoreSelection(previousSelectionID string) {
	if previousSelectionID == "" {
		m.resetSelection()
		return
	}
	m.lastSelID = ""
	for i, row := range m.treeRows {
		if row.id == previousSelectionID {
			m.selectedTree = i
			m.ensureSelectionVisible()
			return
		}
	}
	m.resetSelection()
}

func (m *Model) moveSelection(delta int) {
	if len(m.treeRows) == 0 || delta == 0 {
		return
	}
	if m.selectedTree < 0 {
		m.resetSelection()
		return
	}

	i := m.selectedTree + delta
	for i >= 0 && i < len(m.treeRows) {
		if m.treeRows[i].selectable {
			m.selectedTree = i
			m.ensureSelectionVisible()
			return
		}
		i += delta
	}
}

func (m *Model) pageSelection(direction int) {
	step := maxInt(m.leftHeight-1, 1)
	if direction < 0 {
		step = -step
	}
	m.moveSelection(step)
}

func (m *Model) ensureSelectionVisible() {
	if m.selectedTree < 0 || m.leftHeight <= 0 {
		return
	}
	if m.selectedTree < m.leftOffset {
		m.leftOffset = m.selectedTree
	}
	if m.selectedTree >= m.leftOffset+m.leftHeight {
		m.leftOffset = m.selectedTree - m.leftHeight + 1
	}
	if m.leftOffset < 0 {
		m.leftOffset = 0
	}
}

func (m *Model) collapseTreeAtSelection() (bool, string) {
	if m.selectedTree < 0 || m.selectedTree >= len(m.treeRows) {
		return false, ""
	}
	row := m.treeRows[m.selectedTree]
	target := row.nodeID
	if row.kind == treeKindFile {
		target = row.parentID
	}
	if target == "" {
		return false, ""
	}
	if m.treeCollapsed[target] {
		parentID := m.parentNodeID(target)
		if parentID == "" {
			return false, ""
		}
		target = parentID
	}
	m.treeCollapsed[target] = true
	return true, target
}

func (m *Model) expandTreeAtSelection() (bool, string) {
	if m.selectedTree < 0 || m.selectedTree >= len(m.treeRows) {
		return false, ""
	}
	row := m.treeRows[m.selectedTree]
	target := row.nodeID
	if row.kind == treeKindFile {
		target = row.parentID
	}
	if target == "" {
		return false, ""
	}
	if !m.treeCollapsed[target] {
		return false, ""
	}
	m.treeCollapsed[target] = false
	return true, row.id
}

func (m *Model) toggleTreeAtSelection() (bool, string) {
	if m.selectedTree < 0 || m.selectedTree >= len(m.treeRows) {
		return false, ""
	}
	row := m.treeRows[m.selectedTree]
	target := row.nodeID
	if row.kind == treeKindFile {
		target = row.parentID
	}
	if target == "" {
		return false, ""
	}
	m.treeCollapsed[target] = !m.treeCollapsed[target]
	if m.treeCollapsed[target] {
		return true, target
	}
	return true, row.id
}

func (m *Model) parentNodeID(nodeID string) string {
	for _, row := range m.treeRows {
		if row.nodeID == nodeID {
			return row.parentID
		}
	}
	return ""
}

func buildChangeTreeRows(changes []model.ChangeItem, scopeBranches map[string]string, collapsed map[string]bool) []treeRow {
	roots := map[string]*treeBuildNode{}
	rootOrder := []string{}
	for _, c := range changes {
		scope := c.ScopeLabel()
		r, ok := roots[scope]
		if !ok {
			r = &treeBuildNode{Name: scope, Folders: map[string]*treeBuildNode{}, ScopeKey: scope, IsRoot: scope == "root"}
			roots[scope] = r
			rootOrder = append(rootOrder, scope)
		}
		insertChangeNode(r, c)
	}
	sort.Strings(rootOrder)

	rows := make([]treeRow, 0, len(changes)+len(rootOrder))
	for _, scope := range rootOrder {
		r := roots[scope]
		nodeID := "scope|" + scope
		expanded := !collapsed[nodeID]
		icon := "▾"
		if !expanded {
			icon = "▸"
		}
		header := scopeNodeLabel(scope, scopeBranches)
		rows = append(rows, treeRow{
			id:       nodeID,
			nodeID:   nodeID,
			parentID: "",
			depth:    0,
			kind:     treeKindScope,
			text:     icon + " " + header,
		})
		if expanded {
			rows = append(rows, flattenChangeNodeRows(r, 1, nodeID, scope, collapsed)...)
		}
	}

	return rows
}

func flattenChangeNodeRows(n *treeBuildNode, depth int, parentID string, scope string, collapsed map[string]bool) []treeRow {
	rows := []treeRow{}
	folderNames := make([]string, 0, len(n.Folders))
	for k := range n.Folders {
		folderNames = append(folderNames, k)
	}
	sort.Strings(folderNames)
	indent := strings.Repeat("  ", depth)
	for _, name := range folderNames {
		child := n.Folders[name]
		nodeID := "dir|" + scope + "|" + child.Path
		expanded := !collapsed[nodeID]
		icon := "▾"
		if !expanded {
			icon = "▸"
		}
		rows = append(rows, treeRow{
			id:       nodeID,
			nodeID:   nodeID,
			parentID: parentID,
			depth:    depth,
			kind:     treeKindDir,
			text:     indent + icon + " " + name,
		})
		if expanded {
			rows = append(rows, flattenChangeNodeRows(child, depth+1, nodeID, scope, collapsed)...)
		}
	}

	sort.SliceStable(n.Files, func(i int, j int) bool {
		return n.Files[i].Path < n.Files[j].Path
	})
	for _, f := range n.Files {
		copy := f
		rows = append(rows, treeRow{
			id:         "file|" + f.ScopeLabel() + "|" + string(f.Type) + "|" + f.Path,
			nodeID:     "file|" + f.ScopeLabel() + "|" + f.Path,
			parentID:   parentID,
			depth:      depth,
			kind:       treeKindFile,
			text:       indent + statusLetter(f.Type) + " " + baseName(f.Path),
			selectable: true,
			change:     &copy,
		})
	}

	return rows
}

func insertChangeNode(root *treeBuildNode, change model.ChangeItem) {
	parts := pathParts(change.Path)
	if len(parts) == 0 {
		root.Files = append(root.Files, change)
		return
	}

	cur := root
	for i := 0; i < len(parts)-1; i++ {
		name := parts[i]
		child, ok := cur.Folders[name]
		if !ok {
			full := strings.Join(parts[:i+1], "/")
			child = &treeBuildNode{Name: name, Path: full, Folders: map[string]*treeBuildNode{}, ScopeKey: root.ScopeKey}
			cur.Folders[name] = child
		}
		cur = child
	}
	cur.Files = append(cur.Files, change)
}

func buildRecentTreeRows(files []model.CommitFile, scopeBranches map[string]string, collapsed map[string]bool) []treeRow {
	changes := make([]model.ChangeItem, 0, len(files))
	fileMap := map[string]model.CommitFile{}
	for _, f := range files {
		ctype := model.ChangeTypeUnstaged
		changes = append(changes, model.ChangeItem{Path: f.Path, Type: ctype, SubmodulePath: f.SubmodulePath})
		key := f.Scope + "|" + f.Path
		fileMap[key] = f
	}
	rows := buildChangeTreeRows(changes, scopeBranches, collapsed)
	for i := range rows {
		if rows[i].change != nil {
			key := rows[i].change.ScopeLabel() + "|" + rows[i].change.Path
			if f, ok := fileMap[key]; ok {
				copy := f
				rows[i].commitFile = &copy
				rows[i].change = nil
				rows[i].id = "recent|" + f.Scope + "|" + f.CommitHash + "|" + f.Path
				rows[i].text = strings.Repeat("  ", rows[i].depth) + shortHash(f.CommitHash) + " " + baseName(f.Path)
			}
		}
	}
	return rows
}

func scopeNodeLabel(scope string, scopeBranches map[string]string) string {
	branch := valueOr(scopeBranches[scope], "?")
	if scope == "root" {
		return fmt.Sprintf("/ (%s)", branch)
	}
	return fmt.Sprintf("%s (%s)", scope, branch)
}

func statusLetter(t model.ChangeType) string {
	switch t {
	case model.ChangeTypeStaged:
		return "S"
	case model.ChangeTypeUnstaged:
		return "M"
	case model.ChangeTypeUntracked:
		return "A"
	default:
		return "?"
	}
}

func baseName(path string) string {
	parts := pathParts(path)
	if len(parts) == 0 {
		return path
	}
	return parts[len(parts)-1]
}

func pathParts(path string) []string {
	trimmed := strings.Trim(path, "/")
	if trimmed == "" {
		return nil
	}
	parts := strings.Split(trimmed, "/")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func collectSubmodulePathsFromChanges(items []model.ChangeItem) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0)
	for _, item := range items {
		if item.SubmodulePath == "" {
			continue
		}
		if _, ok := seen[item.SubmodulePath]; ok {
			continue
		}
		seen[item.SubmodulePath] = struct{}{}
		out = append(out, item.SubmodulePath)
	}
	sort.Strings(out)
	return out
}

func collectSubmodulePathsFromCommitFiles(files []model.CommitFile) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0)
	for _, file := range files {
		if file.SubmodulePath == "" {
			continue
		}
		if _, ok := seen[file.SubmodulePath]; ok {
			continue
		}
		seen[file.SubmodulePath] = struct{}{}
		out = append(out, file.SubmodulePath)
	}
	sort.Strings(out)
	return out
}

func gitSubmoduleDir(repoRoot string, submodulePath string) string {
	return filepath.Join(repoRoot, filepath.FromSlash(submodulePath))
}

func copyStringMap(src map[string]string) map[string]string {
	out := make(map[string]string, len(src))
	for k, v := range src {
		out[k] = v
	}
	return out
}

func renderVerticalSep(height int) string {
	if height <= 0 {
		return ""
	}
	line := strings.Repeat("|\n", height)
	return strings.TrimRight(line, "\n")
}
