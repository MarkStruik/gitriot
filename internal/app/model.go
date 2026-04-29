package app

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
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
	ThemeName    string
	Themes       []theme.NamedTheme
	SaveTheme    func(string) error
	RecentWindow time.Duration
	SinceCommit  string
}

type leftPaneTab int

const (
	leftTabFiles leftPaneTab = iota
	leftTabCommits
)

type Model struct {
	repoPath string
	rootName string
	branch   string
	styles   ui.Styles
	colors   theme.Tokens
	filters  Filters

	runner *git.CLIRunner
	index  *git.RepositoryIndexer
	diffs  *git.DiffLoader

	allItems      []model.ChangeItem
	filteredItems []model.ChangeItem
	recentCommits []model.RepoCommit
	rootHistory   []model.RepoCommit
	anchorIndex   int
	anchorHash    string
	recentFiles   []model.CommitFile
	recentVisible []model.CommitFile
	workingKeys   map[string]struct{}
	recentWindow  time.Duration
	sinceCommit   string
	useSinceMode  bool
	showRecent    bool
	recentLoaded  bool
	showHunksOnly bool
	scopeBranches map[string]string
	treeCollapsed map[string]bool
	leftTab       leftPaneTab
	commitCursor  int
	commitOffset  int
	scanStates    []scanState
	scanActive    bool
	scanSpinner   int
	scanRequestID int

	diff            viewport.Model
	help            help.Model
	search          textinput.Model
	themeSearch     textinput.Model
	focus           paneFocus
	showKeybinds    bool
	showThemePicker bool

	treeRows         []treeRow
	selectedTree     int
	leftOffset       int
	leftWidth        int
	leftHeight       int
	themeCursor      int
	themeList        []theme.NamedTheme
	allThemes        []theme.NamedTheme
	currentTheme     theme.FileTheme
	currentThemeName string
	saveTheme        func(string) error
	pickerThemeName  string
	pickerTheme      theme.FileTheme

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
	diffRows        []diffRow
	diffRawLines    []string
	diffXOffset     int
	diffChangeRows  []int
	cachedDiff      *cachedDiffView
}

type diffRowKind int

const (
	diffRowPlain diffRowKind = iota
	diffRowGap
	diffRowCode
)

type diffRow struct {
	kind     diffRowKind
	marker   string
	lineNo   string
	code     string
	markerFg string
	codeBg   string
	gutterBg string
	isChange bool
}

type cachedDiffView struct {
	header       []string
	path         string
	fullContent  string
	changed      []git.LineRange
	decor        map[int]git.LineDecoration
	hunksOnly    bool
	contextLines int
}

type treeRow struct {
	id         string
	nodeID     string
	parentID   string
	depth      int
	kind       string
	status     string
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
	rootHistory []model.RepoCommit
	result      git.IndexResult
	scopeBranch map[string]string
	fingerprint string
	err         error
}

type diffLoadedMsg struct {
	requestID int
	lines     []string
	rows      []diffRow
	cache     *cachedDiffView
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
	rootHistory []model.RepoCommit
	result      git.RecentCommitResult
	scopeBranch map[string]string
	err         error
}

type submodulesDiscoveredMsg struct {
	scanRequestID int
	paths         []string
	err           error
}

type submoduleIndexedMsg struct {
	scanRequestID int
	path          string
	items         []model.ChangeItem
	err           error
}

type commitSummaryLoadedMsg struct {
	requestID int
	hash      string
	summary   model.CommitSummary
	err       error
}

type spinnerTickMsg time.Time

type scanState struct {
	path   string
	label  string
	status string
	err    string
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
	CollapseAll     key.Binding
	ExpandAll       key.Binding
	PrevAnchor      key.Binding
	NextAnchor      key.Binding
	ToggleHunksOnly key.Binding
	NextChange      key.Binding
	PrevChange      key.Binding
	Search          key.Binding
	ToggleRecent    key.Binding
	ThemePicker     key.Binding
	ShowKeybinds    key.Binding
	CloseSearch     key.Binding
	Up              key.Binding
	Down            key.Binding
	PageDown        key.Binding
	PageUp          key.Binding
}

func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.FocusSwitch, k.ToggleRecent, k.CollapseNode, k.ExpandNode, k.CollapseAll, k.ExpandAll}
}

func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.PageUp, k.PageDown, k.FocusSwitch},
		{k.FilterStaged, k.FilterUnstaged, k.FilterUntracked, k.FilterSubmodule, k.ToggleHunksOnly},
		{k.ToggleRecent, k.ThemePicker, k.Search, k.CloseSearch, k.Refresh, k.Quit},
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
	CollapseNode:    key.NewBinding(key.WithKeys("left", "h"), key.WithHelp("←/h", "collapse/left")),
	ExpandNode:      key.NewBinding(key.WithKeys("right", "l"), key.WithHelp("→/l", "expand")),
	ToggleNode:      key.NewBinding(key.WithKeys("space"), key.WithHelp("space", "toggle")),
	CollapseAll:     key.NewBinding(key.WithKeys("x"), key.WithHelp("x", "collapse all")),
	ExpandAll:       key.NewBinding(key.WithKeys("X"), key.WithHelp("X", "expand all")),
	PrevAnchor:      key.NewBinding(key.WithKeys("["), key.WithHelp("[", "prev tab")),
	NextAnchor:      key.NewBinding(key.WithKeys("]"), key.WithHelp("]", "next tab")),
	ToggleHunksOnly: key.NewBinding(key.WithKeys("f", "F"), key.WithHelp("f", "toggle hunks/full")),
	NextChange:      key.NewBinding(key.WithKeys("}"), key.WithHelp("}", "next change")),
	PrevChange:      key.NewBinding(key.WithKeys("{"), key.WithHelp("{", "prev change")),
	ToggleRecent:    key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "current only")),
	ThemePicker:     key.NewBinding(key.WithKeys("t"), key.WithHelp("t", "themes")),
	ShowKeybinds:    key.NewBinding(key.WithKeys("?", "f1"), key.WithHelp("?", "key help")),
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

	ts := textinput.New()
	ts.Prompt = "theme> "
	ts.Placeholder = "filter themes"
	ts.Blur()

	h := help.New()
	h.ShowAll = false

	return Model{
		repoPath:         filepath.Clean(opt.RepoPath),
		rootName:         filepath.Base(filepath.Clean(opt.RepoPath)),
		styles:           ui.NewStyles(opt.Theme),
		colors:           opt.Theme.Colors,
		currentTheme:     opt.Theme,
		currentThemeName: strings.TrimSpace(opt.ThemeName),
		filters:          DefaultFilters(),
		runner:           runner,
		index:            git.NewRepositoryIndexer(runner),
		diffs:            git.NewDiffLoader(runner),
		diff:             d,
		help:             h,
		search:           s,
		themeSearch:      ts,
		focus:            focusChanges,
		message:          "Loading repository status...",
		recentWindow:     opt.RecentWindow,
		sinceCommit:      strings.TrimSpace(opt.SinceCommit),
		useSinceMode:     strings.TrimSpace(opt.SinceCommit) != "",
		showRecent:       strings.TrimSpace(opt.SinceCommit) != "",
		showHunksOnly:    true,
		anchorIndex:      0,
		anchorHash:       strings.TrimSpace(opt.SinceCommit),
		selectedTree:     -1,
		scopeBranches:    map[string]string{},
		treeCollapsed:    map[string]bool{},
		leftTab:          leftTabFiles,
		commitCursor:     0,
		commitOffset:     0,
		workingKeys:      map[string]struct{}{},
		allThemes:        append([]theme.NamedTheme(nil), opt.Themes...),
		themeList:        append([]theme.NamedTheme(nil), opt.Themes...),
		saveTheme:        opt.SaveTheme,
		scanStates: []scanState{{
			path:   "root",
			label:  "root",
			status: "loading",
		}},
		scanActive:    true,
		scanSpinner:   0,
		scanRequestID: 0,
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(m.loadIndexCmd(), m.refreshTickCmd(), m.spinnerTickCmd())
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.showKeybinds {
		if keyMsg, ok := msg.(tea.KeyMsg); ok {
			if key.Matches(keyMsg, keys.ShowKeybinds) || key.Matches(keyMsg, keys.CloseSearch) || key.Matches(keyMsg, keys.Quit) {
				m.showKeybinds = false
				if key.Matches(keyMsg, keys.Quit) {
					return m, tea.Batch(tea.ClearScreen, tea.Quit)
				}
				return m, nil
			}
		}
		return m, nil
	}

	if m.showThemePicker {
		var cmd tea.Cmd
		m.themeSearch, cmd = m.themeSearch.Update(msg)
		if keyMsg, ok := msg.(tea.KeyMsg); ok {
			switch {
			case key.Matches(keyMsg, keys.CloseSearch), key.Matches(keyMsg, keys.ThemePicker):
				return m, m.closeThemePicker(false)
			case keyMsg.Type == tea.KeyEnter:
				return m, m.closeThemePicker(true)
			case key.Matches(keyMsg, keys.Up):
				return m, m.moveThemeSelection(-1)
			case key.Matches(keyMsg, keys.Down):
				return m, m.moveThemeSelection(1)
			case key.Matches(keyMsg, keys.PageUp):
				return m, m.moveThemeSelection(-5)
			case key.Matches(keyMsg, keys.PageDown):
				return m, m.moveThemeSelection(5)
			default:
				previewCmd := m.filterThemeList(m.themeSearch.Value())
				return m, tea.Batch(cmd, previewCmd)
			}
		}
		previewCmd := m.filterThemeList(m.themeSearch.Value())
		return m, tea.Batch(cmd, previewCmd)
	}

	if m.focus == focusSearch {
		var cmd tea.Cmd
		m.search, cmd = m.search.Update(msg)
		if keyMsg, ok := msg.(tea.KeyMsg); ok {
			switch {
			case key.Matches(keyMsg, keys.ShowKeybinds):
				m.search.Blur()
				m.focus = focusChanges
				m.showKeybinds = true
				return m, nil
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
		if m.showRecent {
			return m, m.refreshTickCmd()
		}
		return m, m.checkFingerprintCmd()
	case spinnerTickMsg:
		if m.scanActive {
			m.scanSpinner = (m.scanSpinner + 1) % len(scanSpinnerFrames)
		}
		return m, m.spinnerTickCmd()
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
			m.markScanStatus("root", "error", msg.err.Error())
			m.scanActive = false
			m.message = "Indexing failed: " + msg.err.Error()
			return m, nil
		}

		m.branch = msg.branch
		if len(msg.rootHistory) > 0 {
			m.rootHistory = msg.rootHistory
			m.anchorIndex = resolveAnchorIndex(m.rootHistory, m.anchorHash)
			m.anchorHash = m.rootHistory[m.anchorIndex].Hash
			m.commitCursor = m.anchorIndex
			m.ensureCommitCursorInBounds()
		}
		m.scopeBranches = copyStringMap(msg.scopeBranch)
		m.warn = msg.result.Warnings
		m.lastFingerprint = msg.fingerprint
		m.allItems = msg.result.Items
		if !m.showRecent {
			m.recentVisible = nil
		}
		m.markScanRootDone()
		discoverCmd := m.discoverSubmodulesCmd(m.scanRequestID)
		m.applyCurrentList()
		if m.showRecent {
			m.setDiffText(m.renderRecentSummary())
			m.activeRef = "recent snapshot"
			if m.recentLoaded {
				m.message = fmt.Sprintf("Loaded %d files from recent commits", len(m.recentVisible))
				if cmd := m.autoLoadSelectedDiff(); cmd != nil {
					return m, tea.Batch(cmd, discoverCmd, m.refreshTickCmd())
				}
			} else {
				if m.effectiveSinceMode() {
					m.message = "Loading files since selected commit..."
				} else {
					m.message = "Loading recent commits..."
				}
				return m, tea.Batch(m.loadRecentCmd(), discoverCmd, m.refreshTickCmd())
			}
		} else {
			m.activeRef = ""
			m.message = fmt.Sprintf("Loaded %d root changes (scanning submodules...)", len(m.allItems))
			if cmd := m.autoLoadSelectedDiff(); cmd != nil {
				return m, tea.Batch(cmd, discoverCmd, m.refreshTickCmd())
			}
		}
		return m, tea.Batch(discoverCmd, m.refreshTickCmd())
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
		if len(msg.rootHistory) > 0 {
			m.rootHistory = msg.rootHistory
			m.anchorIndex = resolveAnchorIndex(m.rootHistory, msg.result.Anchor.Hash)
			m.anchorHash = msg.result.Anchor.Hash
			m.commitCursor = m.anchorIndex
			m.ensureCommitCursorInBounds()
		}
		m.recentCommits = msg.result.Commits
		m.recentFiles = msg.result.Files
		if len(msg.scopeBranch) > 0 {
			m.scopeBranches = copyStringMap(msg.scopeBranch)
		}
		m.warn = append(m.warn, msg.result.Warnings...)
		m.applyCurrentList()
		if m.leftTab == leftTabCommits {
			m.ensureCommitCursorInBounds()
			if cmd := m.loadSelectedCommitSummaryCmd(); cmd != nil {
				return m, cmd
			}
		} else {
			m.setDiffText(m.renderRecentSummary())
		}
		m.message = fmt.Sprintf("Loaded %d files from recent commits", len(m.recentVisible))
		if cmd := m.autoLoadSelectedDiff(); cmd != nil {
			return m, cmd
		}
		return m, nil
	case submodulesDiscoveredMsg:
		if msg.scanRequestID != m.scanRequestID {
			return m, nil
		}
		if msg.err != nil {
			m.warn = append(m.warn, "submodule discovery unavailable: "+msg.err.Error())
			m.scanActive = false
			return m, nil
		}
		if len(msg.paths) == 0 {
			m.scanActive = false
			if !m.showRecent {
				m.message = fmt.Sprintf("Loaded %d changes", len(m.allItems))
			}
			return m, nil
		}
		m.setSubmoduleScanPaths(msg.paths)
		cmds := make([]tea.Cmd, 0, len(msg.paths))
		for _, path := range msg.paths {
			m.markScanStatus(path, "loading", "")
			cmds = append(cmds, m.indexSubmoduleCmd(msg.scanRequestID, path))
		}
		m.message = fmt.Sprintf("Scanning %d submodules...", len(msg.paths))
		return m, tea.Batch(cmds...)
	case submoduleIndexedMsg:
		if msg.scanRequestID != m.scanRequestID {
			return m, nil
		}
		if msg.err != nil {
			m.warn = append(m.warn, fmt.Sprintf("submodule %q %v", msg.path, msg.err))
			m.markScanStatus(msg.path, "error", msg.err.Error())
		} else {
			m.markScanStatus(msg.path, "done", "")
			if len(msg.items) > 0 {
				previous := m.currentSelectionID()
				m.allItems = append(m.allItems, msg.items...)
				m.applyCurrentListWithPreserve(previous)
			}
		}
		if m.scanAllDone() {
			m.scanActive = false
			if !m.showRecent {
				m.message = fmt.Sprintf("Loaded %d total changes", len(m.allItems))
			}
			return m, nil
		}
		done, total := m.scanProgressCounts()
		m.message = fmt.Sprintf("Scanning submodules... (%d/%d)", done, total)
		return m, nil
	case commitSummaryLoadedMsg:
		if msg.requestID != m.requestID {
			return m, nil
		}
		if selected := m.selectedRootCommit(); selected != nil && selected.Hash != msg.hash {
			return m, nil
		}
		if msg.err != nil {
			m.setDiffText("Unable to load commit details:\n" + msg.err.Error())
			m.message = "Commit detail load failed"
			return m, nil
		}
		m.activeRef = "commit " + shortHash(msg.hash)
		m.setDiffText(renderCommitSummary(msg.summary))
		m.message = "Commit details loaded"
		return m, nil
	case diffLoadedMsg:
		if msg.requestID != m.requestID {
			return m, nil
		}

		if msg.err != nil {
			m.setDiffText("Unable to load diff:\n" + msg.err.Error())
			m.message = "Diff loading failed"
			return m, nil
		}

		m.cachedDiff = msg.cache
		if len(msg.rows) > 0 {
			m.diffRows = msg.rows
			m.diffRawLines = diffRowsToRawLines(msg.rows)
		} else {
			m.diffRawLines = msg.lines
			m.diffRows = buildPlainDiffRows(msg.lines)
		}
		m.diffXOffset = 0
		m.diffChangeRows = collectChangeRowIndexes(m.diffRows)
		m.refreshDiffViewport()
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
		case key.Matches(msg, keys.ShowKeybinds):
			m.showKeybinds = true
			return m, nil
		case key.Matches(msg, keys.Quit):
			return m, tea.Batch(tea.ClearScreen, tea.Quit)
		case key.Matches(msg, keys.FocusSwitch):
			m.rotateFocus()
			return m, nil
		case key.Matches(msg, keys.Search):
			m.focus = focusSearch
			m.search.Focus()
			return m, textinput.Blink
		case key.Matches(msg, keys.ToggleRecent):
			m.showRecent = false
			m.leftTab = leftTabFiles
			m.applyCurrentList()
			m.activeRef = ""
			m.message = "Current changes only"
			if cmd := m.autoLoadSelectedDiff(); cmd != nil {
				return m, tea.Batch(cmd, m.loadIndexCmd())
			}
			return m, m.loadIndexCmd()
		case key.Matches(msg, keys.ThemePicker):
			return m, tea.Batch(textinput.Blink, m.openThemePicker())
		case key.Matches(msg, keys.PrevAnchor):
			if m.focus != focusChanges {
				return m, nil
			}
			m.prevLeftTab()
			if cmd := m.previewCmdForActiveTab(); cmd != nil {
				return m, cmd
			}
			return m, nil
		case key.Matches(msg, keys.NextAnchor):
			if m.focus != focusChanges {
				return m, nil
			}
			m.nextLeftTab()
			if cmd := m.previewCmdForActiveTab(); cmd != nil {
				return m, cmd
			}
			return m, nil
		case key.Matches(msg, keys.CollapseNode):
			if m.focus == focusChanges {
				if m.leftTab == leftTabCommits {
					return m, nil
				}
				if ok, preserve := m.collapseTreeAtSelection(); ok {
					m.applyCurrentListWithPreserve(preserve)
					if cmd := m.autoLoadSelectedDiff(); cmd != nil {
						return m, cmd
					}
				}
				return m, nil
			}
			if m.focus == focusDiff {
				m.shiftDiffX(-8)
				return m, nil
			}
		case key.Matches(msg, keys.ExpandNode):
			if m.focus == focusChanges {
				if m.leftTab == leftTabCommits {
					return m, nil
				}
				if ok, preserve := m.expandTreeAtSelection(); ok {
					m.applyCurrentListWithPreserve(preserve)
					if cmd := m.autoLoadSelectedDiff(); cmd != nil {
						return m, cmd
					}
				}
				return m, nil
			}
			if m.focus == focusDiff {
				m.shiftDiffX(8)
				return m, nil
			}
		case key.Matches(msg, keys.ToggleNode):
			if m.focus == focusChanges {
				if m.leftTab == leftTabCommits {
					return m, nil
				}
				if ok, preserve := m.toggleTreeAtSelection(); ok {
					m.applyCurrentListWithPreserve(preserve)
					if cmd := m.autoLoadSelectedDiff(); cmd != nil {
						return m, cmd
					}
				}
				return m, nil
			}
		case key.Matches(msg, keys.CollapseAll):
			if m.focus == focusChanges {
				if m.leftTab == leftTabCommits {
					return m, nil
				}
				if m.collapseAllTreeNodes() {
					m.applyCurrentListWithPreserve("")
					if cmd := m.autoLoadSelectedDiff(); cmd != nil {
						return m, cmd
					}
				}
				return m, nil
			}
		case key.Matches(msg, keys.ExpandAll):
			if m.focus == focusChanges {
				if m.leftTab == leftTabCommits {
					return m, nil
				}
				if m.expandAllTreeNodes() {
					m.applyCurrentListWithPreserve(m.currentSelectionID())
					if cmd := m.autoLoadSelectedDiff(); cmd != nil {
						return m, cmd
					}
				}
				return m, nil
			}
		case key.Matches(msg, keys.ToggleHunksOnly):
			if !m.currentSelectionIsFile() {
				m.message = "Hunks mode applies to file selections only"
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
		case key.Matches(msg, keys.NextChange):
			if m.focus == focusDiff {
				if m.jumpToChange(true) {
					m.message = "Jumped to next change"
				} else {
					m.message = "No more changes"
				}
				return m, nil
			}
		case key.Matches(msg, keys.PrevChange):
			if m.focus == focusDiff {
				if m.jumpToChange(false) {
					m.message = "Jumped to previous change"
				} else {
					m.message = "No previous changes"
				}
				return m, nil
			}
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
			if m.leftTab == leftTabCommits {
				commit := m.selectedRootCommit()
				if commit == nil {
					return m, nil
				}
				m.anchorHash = commit.Hash
				m.anchorIndex = m.commitCursor
				m.useSinceMode = true
				m.showRecent = true
				m.recentLoaded = false
				m.leftTab = leftTabFiles
				m.message = "Loading files since selected commit..."
				m.setDiffText("Loading commit range...")
				return m, m.loadRecentCmd()
			}
			if m.selectedTree >= 0 && m.selectedTree < len(m.treeRows) {
				row := m.treeRows[m.selectedTree]
				if row.kind == treeKindScope || row.kind == treeKindDir {
					if ok, preserve := m.toggleTreeAtSelection(); ok {
						m.applyCurrentListWithPreserve(preserve)
					}
					return m, nil
				}
			}
			item := m.selectedItem()
			if item == nil {
				file := m.selectedRecentFile()
				if file == nil {
					return m, nil
				}
				return m, m.loadCommitDiffCmd(*file)
			}
			if m.showRecent {
				if _, ok := m.workingKeys[item.ScopeLabel()+"|"+item.Path]; !ok {
					file := m.selectedRecentFile()
					if file != nil {
						return m, m.loadCommitDiffCmd(*file)
					}
				}
			}
			return m, m.loadDiffCmd(*item)
		case key.Matches(msg, keys.Up):
			if m.focus == focusChanges {
				if m.leftTab == leftTabCommits {
					m.moveCommitSelection(-1)
					if cmd := m.loadSelectedCommitSummaryCmd(); cmd != nil {
						return m, cmd
					}
					return m, nil
				}
				m.moveSelection(-1)
				if cmd := m.autoLoadSelectedDiff(); cmd != nil {
					return m, cmd
				}
				return m, nil
			}
		case key.Matches(msg, keys.Down):
			if m.focus == focusChanges {
				if m.leftTab == leftTabCommits {
					m.moveCommitSelection(1)
					if cmd := m.loadSelectedCommitSummaryCmd(); cmd != nil {
						return m, cmd
					}
					return m, nil
				}
				m.moveSelection(1)
				if cmd := m.autoLoadSelectedDiff(); cmd != nil {
					return m, cmd
				}
				return m, nil
			}
		case key.Matches(msg, keys.PageUp):
			if m.focus == focusChanges {
				if m.leftTab == leftTabCommits {
					m.pageCommitSelection(-1)
					if cmd := m.loadSelectedCommitSummaryCmd(); cmd != nil {
						return m, cmd
					}
					return m, nil
				}
				m.pageSelection(-1)
				if cmd := m.autoLoadSelectedDiff(); cmd != nil {
					return m, cmd
				}
				return m, nil
			}
		case key.Matches(msg, keys.PageDown):
			if m.focus == focusChanges {
				if m.leftTab == leftTabCommits {
					m.pageCommitSelection(1)
					if cmd := m.loadSelectedCommitSummaryCmd(); cmd != nil {
						return m, cmd
					}
					return m, nil
				}
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
	leftContent := m.renderChangesPane(leftWidth, paneHeight)
	if m.showThemePicker {
		leftContent = m.renderThemePickerPane(leftWidth, paneHeight)
	}
	left := lipgloss.NewStyle().Height(paneHeight).MaxHeight(paneHeight).Render(leftContent)
	right := lipgloss.NewStyle().Height(paneHeight).MaxHeight(paneHeight).Render(m.renderDiffPane(rightWidth, paneHeight))
	sep := lipgloss.NewStyle().Foreground(lipgloss.Color(m.colors.LineSep)).Render(renderVerticalSep(paneHeight))
	panes := lipgloss.NewStyle().Height(paneHeight).MaxHeight(paneHeight).Render(lipgloss.JoinHorizontal(lipgloss.Top, left, sep, right))
	bottom := m.renderBottomBar()

	base := lipgloss.JoinVertical(lipgloss.Left, top, panes, bottom)
	if m.focus == focusSearch {
		search := m.styles.SearchPrompt.Render("Search: ") + m.search.View()
		base = lipgloss.JoinVertical(lipgloss.Left, base, search)
	}
	if m.showKeybinds {
		base = lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, m.renderKeybindModal(), lipgloss.WithWhitespaceBackground(lipgloss.Color(m.colors.Bg)))
	}

	return lipgloss.Place(m.width, m.height, lipgloss.Left, lipgloss.Top, base, lipgloss.WithWhitespaceBackground(lipgloss.Color(m.colors.Bg)))
}

func (m *Model) resize() {
	if m.width <= 0 || m.height <= 0 {
		return
	}

	leftWidth, rightWidth, paneHeight := paneDimensions(m.width, m.height, m.focus == focusSearch)

	m.leftWidth = maxInt(leftWidth, 8)
	m.leftHeight = maxInt(paneHeight-2, 1)
	m.diff.Width = maxInt(rightWidth-1, 8)
	m.diff.Height = maxInt(paneHeight-1, 1)
	if m.leftTab == leftTabCommits {
		m.ensureCommitCursorInBounds()
	} else {
		m.ensureSelectionVisible()
	}
	m.refreshDiffViewport()
}

func (m *Model) rotateFocus() {
	if m.focus == focusChanges {
		m.focus = focusDiff
		return
	}
	m.focus = focusChanges
}

func (m *Model) renderTopBar() string {
	barBg := lipgloss.Color(m.colors.Accent)
	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(m.colors.Bg)).Background(barBg).Render(" GitRiot ")
	return lipgloss.NewStyle().Foreground(lipgloss.Color(m.colors.Bg)).Background(barBg).Width(m.width).Render(title)
}

func (m *Model) renderChangesPane(width int, height int) string {
	bg := lipgloss.Color(m.colors.PanelLeftBg)
	titleText := "Changes"
	titlePrefix := "  "
	if m.focus == focusChanges {
		titlePrefix = "* "
	}
	titleLine := titlePrefix + titleText
	if m.loading {
		titleLine += " (loading)"
	}
	titleBg := bg
	titleFg := lipgloss.Color(m.colors.Muted)
	if m.focus == focusChanges {
		titleBg = lipgloss.Color(darkenHexColor(m.colors.Accent, 0.72))
		titleFg = lipgloss.Color(m.colors.Bg)
	}
	title := lipgloss.NewStyle().Bold(true).Foreground(titleFg).Background(titleBg).Width(width).Render(titleLine)

	tabLine := m.renderRecentTabs(width, bg)
	body := m.renderTreePanel()
	if m.leftTab == leftTabCommits {
		body = m.renderCommitPanel()
	} else if len(m.treeRows) == 0 {
		body = lipgloss.NewStyle().Foreground(lipgloss.Color(m.colors.Muted)).Background(bg).Width(width).Render("No changes match filters")
	}
	panelParts := []string{title}
	panelParts = append(panelParts, tabLine)
	panelParts = append(panelParts, body)
	panel := lipgloss.JoinVertical(lipgloss.Left, panelParts...)
	return lipgloss.NewStyle().Background(bg).Width(width).Height(maxInt(height, 3)).Render(panel)
}

func (m *Model) renderDiffPane(width int, height int) string {
	titleText := "Diff"
	if m.leftTab == leftTabCommits {
		titleText = "Commit Preview"
	} else if m.showRecent {
		titleText = "Commit Details"
	}
	if m.showRecent || m.leftTab == leftTabCommits {
		if len(m.rootHistory) > 0 && m.anchorIndex >= 0 && m.anchorIndex < len(m.rootHistory) {
			anchor := m.rootHistory[m.anchorIndex]
			titleText += " @" + shortHash(anchor.Hash)
		}
	}
	if m.activeRef != "" {
		titleText = titleText + " - " + truncateText(m.activeRef, maxInt(m.width/2, 24))
	}
	if m.currentSelectionIsFile() && m.leftTab != leftTabCommits {
		if m.showHunksOnly {
			titleText += " [Hunks+5]"
		} else {
			titleText += " [Full]"
		}
	}
	titlePrefix := "  "
	if m.focus == focusDiff {
		titlePrefix = "* "
	}
	bg := lipgloss.Color(m.colors.PanelRightBg)
	titleBg := bg
	titleFg := lipgloss.Color(m.colors.Muted)
	if m.focus == focusDiff {
		titleBg = lipgloss.Color(darkenHexColor(m.colors.Accent, 0.72))
		titleFg = lipgloss.Color(m.colors.Bg)
	}
	title := lipgloss.NewStyle().Bold(true).Foreground(titleFg).Background(titleBg).Width(width).Render(titlePrefix + titleText)
	body := m.renderDiffBody(width, maxInt(height-1, 1), bg)
	panel := lipgloss.JoinVertical(lipgloss.Left, title, body)
	return lipgloss.NewStyle().Background(bg).Width(width).Height(maxInt(height, 3)).Render(panel)
}

func (m *Model) renderBottomBar() string {
	msg := m.message
	if msg == "" {
		msg = "Ready"
	}
	line := truncateText(msg, maxInt(m.width-2, 10))
	barBg := lipgloss.Color(m.colors.Bg)
	line = lipgloss.NewStyle().Foreground(lipgloss.Color(m.colors.Muted)).Background(barBg).Width(m.width).Render(line)

	active := m.activeKeybindSummary()
	active = truncateText(active+"  |  ? help", maxInt(m.width-2, 10))
	activeLine := lipgloss.NewStyle().Foreground(lipgloss.Color(m.colors.Accent)).Background(barBg).Bold(true).Width(m.width).Render(active)

	return lipgloss.JoinVertical(lipgloss.Left, line, activeLine)
}

func (m *Model) activeKeybindSummary() string {
	if m.focus == focusSearch {
		return "SEARCH  type filter  enter apply  esc close"
	}
	if m.showThemePicker {
		return "THEMES  ↑/↓ preview  enter apply  esc cancel"
	}
	if m.focus == focusDiff {
		return "DIFF  f hunks/full  { prev change  } next change  h/← pan left  l/→ pan right"
	}
	if m.leftTab == leftTabCommits {
		return "COMMITS  ↑/↓ move  enter apply+switch  [ ] tabs"
	}
	if m.showRecent {
		return "FILES  ↑/↓ move  enter open  ←/h collapse  →/l expand  x/X all  c current only"
	}
	return "FILES  ↑/↓ move  enter open  ←/h collapse  →/l expand  x/X all  c current only"
}

func (m *Model) renderKeybindModal() string {
	content := strings.Join([]string{
		"Keybindings",
		"",
		"Global: q quit | tab switch pane | r refresh | / search | t themes | c current only | ? help",
		"Tree:   ↑/↓ move | enter toggle folder | ←/h collapse | →/l expand | space toggle | x/X all",
		"Tabs:   [ prev tab | ] next tab | Commits tab: enter apply anchor and switch to Files",
		"Mode:   c return to current-only view (live updates enabled)",
		"Diff:   f hunks/full | { prev change | } next change | h/← pan left | l/→ pan right",
		"Themes: t open picker | type filter | ↑/↓ preview | enter apply | esc cancel",
		"Search: type query | enter apply | esc close",
		"",
		"Press ? or esc to close",
	}, "\n")

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(m.colors.Accent)).
		Background(lipgloss.Color(m.colors.PanelRightBg)).
		Foreground(lipgloss.Color(m.colors.Fg)).
		Padding(1, 2).
		Width(maxInt(minInt(m.width-8, 120), 40)).
		Render(content)
	return box
}

func (m *Model) renderThemePickerPane(width int, height int) string {
	bg := lipgloss.Color(m.colors.PanelLeftBg)
	titleBg := lipgloss.Color(darkenHexColor(m.colors.Accent, 0.72))
	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(m.colors.Bg)).Background(titleBg).Width(width).Render("* Themes")
	search := lipgloss.NewStyle().Background(bg).Width(width).Render(m.styles.SearchPrompt.Render("Search: ") + m.themeSearch.View())
	bodyHeight := maxInt(height-4, 1)
	lines := make([]string, 0, bodyHeight)
	if len(m.themeList) == 0 {
		lines = append(lines, m.styles.Muted.Render("No themes match filter"))
	} else {
		start := 0
		if m.themeCursor >= bodyHeight {
			start = m.themeCursor - bodyHeight + 1
		}
		end := minInt(start+bodyHeight, len(m.themeList))
		for i := start; i < end; i++ {
			entry := m.themeList[i]
			prefix := "  "
			style := lipgloss.NewStyle().Foreground(lipgloss.Color(m.colors.Fg)).Background(bg)
			if i == m.themeCursor {
				prefix = "❯ "
				style = style.Bold(true).Foreground(lipgloss.Color(m.colors.Accent))
			}
			label := entry.Name
			if entry.Kind != "" {
				label += "  [" + entry.Kind + "]"
			}
			lines = append(lines, style.Width(width).Render(truncateText(prefix+label, maxInt(width-1, 1))))
		}
	}
	for len(lines) < bodyHeight {
		lines = append(lines, lipgloss.NewStyle().Background(bg).Width(width).Render(""))
	}
	body := lipgloss.NewStyle().Background(bg).Width(width).Height(bodyHeight).Render(strings.Join(lines, "\n"))
	help := lipgloss.NewStyle().Foreground(lipgloss.Color(m.colors.Muted)).Background(bg).Width(width).Render("enter apply | esc cancel")
	panel := lipgloss.JoinVertical(lipgloss.Left, title, search, body, help)
	return lipgloss.NewStyle().Background(bg).Width(width).Height(maxInt(height, 3)).Render(panel)
}

func (m *Model) openThemePicker() tea.Cmd {
	m.showThemePicker = true
	m.themeSearch.SetValue("")
	m.themeSearch.Focus()
	if strings.TrimSpace(m.currentThemeName) == "" && m.currentTheme.Name != "" {
		m.currentThemeName = m.currentTheme.Name
	}
	m.pickerTheme = m.currentTheme
	m.pickerThemeName = m.currentThemeName
	previewCmd := m.filterThemeList("")
	return previewCmd
}

func (m *Model) closeThemePicker(apply bool) tea.Cmd {
	m.showThemePicker = false
	m.themeSearch.Blur()
	if !apply {
		m.applyTheme(m.pickerThemeName, m.pickerTheme)
		m.message = "Theme preview cancelled"
		return nil
	}
	if len(m.themeList) == 0 {
		m.applyTheme(m.pickerThemeName, m.pickerTheme)
		m.message = "No theme selected"
		return nil
	}
	entry := m.themeList[m.themeCursor]
	m.applyTheme(entry.Name, entry.Theme)
	if m.saveTheme != nil {
		if err := m.saveTheme(entry.Name); err != nil {
			m.warn = append(m.warn, "save theme failed: "+err.Error())
			m.message = "Theme applied, but config save failed"
			return nil
		}
	}
	m.message = "Theme applied: " + entry.Name
	return nil
}

func (m *Model) filterThemeList(query string) tea.Cmd {
	q := strings.ToLower(strings.TrimSpace(query))
	filtered := make([]theme.NamedTheme, 0, len(m.allThemes))
	for _, entry := range m.allThemes {
		hay := strings.ToLower(entry.Name + " " + entry.Kind + " " + entry.Theme.Name)
		if q != "" && !strings.Contains(hay, q) {
			continue
		}
		filtered = append(filtered, entry)
	}
	m.themeList = filtered
	if len(filtered) == 0 {
		m.themeCursor = 0
		m.applyTheme(m.pickerThemeName, m.pickerTheme)
		return nil
	}
	for i, entry := range filtered {
		if entry.Name == m.currentThemeName {
			m.themeCursor = i
			m.previewSelectedTheme()
			return nil
		}
	}
	if m.themeCursor >= len(filtered) {
		m.themeCursor = len(filtered) - 1
		if m.themeCursor < 0 {
			m.themeCursor = 0
		}
	}
	m.previewSelectedTheme()
	return nil
}

func (m *Model) moveThemeSelection(delta int) tea.Cmd {
	if len(m.themeList) == 0 || delta == 0 {
		return nil
	}
	m.themeCursor += delta
	if m.themeCursor < 0 {
		m.themeCursor = 0
	}
	if m.themeCursor >= len(m.themeList) {
		m.themeCursor = len(m.themeList) - 1
	}
	m.previewSelectedTheme()
	return nil
}

func (m *Model) previewSelectedTheme() {
	if len(m.themeList) == 0 || m.themeCursor < 0 || m.themeCursor >= len(m.themeList) {
		return
	}
	entry := m.themeList[m.themeCursor]
	m.applyTheme(entry.Name, entry.Theme)
	m.message = "Previewing theme: " + entry.Name
}

func (m *Model) applyTheme(name string, selected theme.FileTheme) {
	m.currentTheme = selected
	m.currentThemeName = name
	m.colors = selected.Colors
	m.styles = ui.NewStyles(selected)
	if m.rebuildCachedDiffRows() {
		return
	}
	m.refreshDiffViewport()
}

func (m *Model) loadIndexCmd() tea.Cmd {
	m.loading = true
	m.requestID++
	m.scanRequestID++
	m.scanActive = true
	m.scanSpinner = 0
	m.scanStates = []scanState{{path: "root", label: m.rootScanLabel(), status: "loading"}}
	m.recentLoaded = false
	requestID := m.requestID
	repoPath := m.repoPath

	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
		defer cancel()

		branch, branchErr := git.CurrentBranch(ctx, m.runner, repoPath)
		fingerprint, fingerprintErr := git.WorkingTreeFingerprint(ctx, m.runner, repoPath)
		rootHistory, historyErr := git.RootCommitHistory(ctx, m.runner, repoPath, 120)
		rootItems, err := m.index.IndexRoot(ctx, repoPath)
		result := git.IndexResult{Items: rootItems}
		scopeBranches := map[string]string{"root": valueOr(branch, "?")}
		if branchErr != nil && err == nil {
			result.Warnings = append(result.Warnings, branchErr.Error())
		}
		if fingerprintErr != nil && err == nil {
			result.Warnings = append(result.Warnings, "fingerprint unavailable: "+fingerprintErr.Error())
		}
		if historyErr != nil && err == nil {
			result.Warnings = append(result.Warnings, "commit history unavailable: "+historyErr.Error())
		}

		return indexLoadedMsg{requestID: requestID, branch: branch, rootHistory: rootHistory, result: result, scopeBranch: scopeBranches, fingerprint: fingerprint, err: err}
	}
}

func (m *Model) discoverSubmodulesCmd(scanRequestID int) tea.Cmd {
	repoPath := m.repoPath
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
		defer cancel()
		paths, err := m.index.DiscoverSubmodules(ctx, repoPath)
		return submodulesDiscoveredMsg{scanRequestID: scanRequestID, paths: paths, err: err}
	}
}

func (m *Model) indexSubmoduleCmd(scanRequestID int, path string) tea.Cmd {
	repoPath := m.repoPath
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
		defer cancel()
		items, err := m.index.IndexSubmodule(ctx, repoPath, path)
		return submoduleIndexedMsg{scanRequestID: scanRequestID, path: path, items: items, err: err}
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

	m.setDiffText("Loading diff...")
	showHunksOnly := m.showHunksOnly
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
		defer cancel()

		result, err := m.diffs.Load(ctx, req)
		if err != nil {
			return diffLoadedMsg{requestID: requestID, err: err}
		}

		if result.IsBinary {
			return diffLoadedMsg{requestID: requestID, lines: []string{"Binary file diff"}, isBinary: true, empty: result.Empty}
		}

		full, fullErr := m.diffs.LoadWorkingFile(req)
		if fullErr != nil {
			fallback := "Path: " + activeRef + "\n\n" + m.renderDiff(result)
			return diffLoadedMsg{requestID: requestID, lines: strings.Split(fallback, "\n"), empty: result.Empty}
		}

		ranges := git.ParseChangedLineRangesFromPatch(result.Patch)
		decor := git.ParseLineDecorationsFromPatch(result.Patch)
		cache := &cachedDiffView{header: []string{"Path: " + activeRef, ""}, path: req.Path, fullContent: full, changed: ranges, decor: decor, hunksOnly: showHunksOnly, contextLines: 5}
		rows := m.buildCachedDiffRows(cache)
		return diffLoadedMsg{requestID: requestID, rows: rows, cache: cache, empty: result.Empty}
	}
}

func (m *Model) refreshTickCmd() tea.Cmd {
	return tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
		return refreshTickMsg(t)
	})
}

func (m *Model) spinnerTickCmd() tea.Cmd {
	return tea.Tick(120*time.Millisecond, func(t time.Time) tea.Msg {
		return spinnerTickMsg(t)
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
	requestID := m.requestID
	repoPath := m.repoPath
	anchorHash := m.anchorHash
	sinceMode := m.effectiveSinceMode()
	currentHistory := append([]model.RepoCommit(nil), m.rootHistory...)
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
		defer cancel()
		history := currentHistory
		if len(history) == 0 {
			loadedHistory, historyErr := git.RootCommitHistory(ctx, m.runner, repoPath, 120)
			if historyErr != nil {
				return recentLoadedMsg{requestID: requestID, err: historyErr}
			}
			history = loadedHistory
		}
		if len(history) == 0 {
			return recentLoadedMsg{requestID: requestID, err: fmt.Errorf("no root commits found")}
		}
		if strings.TrimSpace(anchorHash) == "" {
			anchorHash = history[0].Hash
		}

		var res git.RecentCommitResult
		var err error
		if sinceMode {
			res, err = git.CollectCommitsSince(ctx, m.runner, repoPath, anchorHash)
		} else {
			res, err = git.CollectRecentCommitsAt(ctx, m.runner, repoPath, m.recentWindow, anchorHash)
		}
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
		return recentLoadedMsg{requestID: requestID, rootHistory: history, result: res, scopeBranch: scopeBranches, err: err}
	}
}

func (m *Model) applyCurrentList() {
	m.applyCurrentListWithPreserve("")
}

func (m *Model) applyCurrentListWithPreserve(previousSelectionID string) {
	if m.leftTab == leftTabCommits {
		m.applyFiltersToList(previousSelectionID)
		return
	}
	if m.showRecent {
		m.applyMergedFilesToList(previousSelectionID)
		return
	}
	m.applyFiltersToList(previousSelectionID)
}

func (m *Model) applyFiltersToList(previousSelectionID string) {
	m.filteredItems = ApplyFilters(m.allItems, m.filters)
	m.treeRows = buildChangeTreeRows(m.filteredItems, m.scopeBranches, m.treeCollapsed, m.rootName)
	m.restoreSelection(previousSelectionID)
}

func (m *Model) applyMergedFilesToList(previousSelectionID string) {
	m.filteredItems = ApplyFilters(m.allItems, m.filters)
	m.workingKeys = make(map[string]struct{}, len(m.filteredItems))
	for _, item := range m.filteredItems {
		key := item.ScopeLabel() + "|" + item.Path
		m.workingKeys[key] = struct{}{}
	}

	query := strings.ToLower(strings.TrimSpace(m.filters.Query))
	filtered := make([]model.CommitFile, 0, len(m.recentFiles))
	fileMap := make(map[string]model.CommitFile, len(m.recentFiles))
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
		key := file.Scope + "|" + file.Path
		if _, ok := fileMap[key]; !ok {
			fileMap[key] = file
		}
	}
	m.recentVisible = filtered

	combined := make([]model.ChangeItem, 0, len(m.filteredItems)+len(filtered))
	combined = append(combined, m.filteredItems...)
	for _, file := range filtered {
		key := file.Scope + "|" + file.Path
		if _, ok := m.workingKeys[key]; ok {
			continue
		}
		combined = append(combined, commitFileToChangeItem(file, m.repoPath))
	}

	m.treeRows = buildChangeTreeRows(combined, m.scopeBranches, m.treeCollapsed, m.rootName)
	for i := range m.treeRows {
		if m.treeRows[i].kind != treeKindFile || m.treeRows[i].change == nil {
			continue
		}
		key := m.treeRows[i].change.ScopeLabel() + "|" + m.treeRows[i].change.Path
		if file, ok := fileMap[key]; ok {
			fileCopy := file
			if _, isWorkingFile := m.workingKeys[key]; !isWorkingFile {
				m.treeRows[i].id = commitFileSelectionID(file)
			}
			m.treeRows[i].commitFile = &fileCopy
		}
	}
	m.restoreSelection(previousSelectionID)
}

func commitFileSelectionID(file model.CommitFile) string {
	return "commitfile|" + file.Scope + "|" + file.CommitHash + "|" + file.Path
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

func minInt(a int, b int) int {
	if a < b {
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
	if len(m.recentCommits) == 0 {
		if m.effectiveSinceMode() {
			return m.styles.Muted.Render("No commits found from the selected anchor timestamp to now.")
		}
		return m.styles.Muted.Render("No commits found in the selected window.")
	}

	b := strings.Builder{}
	if m.effectiveSinceMode() {
		b.WriteString(m.styles.Muted.Render("Commit range snapshot from selected anchor timestamp to now. Select a file on the left to auto-load its commit diff."))
	} else {
		b.WriteString(m.styles.Muted.Render("Recent commit snapshot. Select a file on the left to auto-load its commit diff."))
	}
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
	if m.effectiveSinceMode() {
		anchorWhen, ok := m.currentAnchorTime()
		if !ok {
			m.setDiffText("Unable to resolve selected anchor timestamp")
			return nil
		}
		m.requestID++
		requestID := m.requestID
		scope := file.Scope
		if file.IsRoot {
			scope = "root"
		}
		activeRef := fmt.Sprintf("%s/%s since %s", scope, file.Path, anchorWhen.Local().Format(time.RFC3339))
		m.activeRef = activeRef
		req := git.SinceDiffRequest{
			RepoRoot:      m.repoPath,
			SubmodulePath: file.SubmodulePath,
			Path:          file.Path,
			Since:         anchorWhen,
		}

		m.setDiffText("Loading since-anchor diff...")
		showHunksOnly := m.showHunksOnly
		return func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
			defer cancel()

			result, err := m.diffs.LoadSince(ctx, req)
			if err != nil {
				return diffLoadedMsg{requestID: requestID, err: err}
			}
			if result.IsBinary {
				return diffLoadedMsg{requestID: requestID, lines: []string{"Binary file diff"}, isBinary: true, empty: result.Empty}
			}

			full, fullErr := m.diffs.LoadHeadFile(ctx, req)
			if fullErr != nil {
				fallback := "Path: " + activeRef + "\n\n" + m.renderDiff(result)
				return diffLoadedMsg{requestID: requestID, lines: strings.Split(fallback, "\n"), empty: result.Empty}
			}

			ranges := git.ParseChangedLineRangesFromPatch(result.Patch)
			decor := git.ParseLineDecorationsFromPatch(result.Patch)
			cache := &cachedDiffView{header: []string{"Path: " + activeRef, ""}, path: file.Path, fullContent: full, changed: ranges, decor: decor, hunksOnly: showHunksOnly, contextLines: 5}
			rows := m.buildCachedDiffRows(cache)
			return diffLoadedMsg{requestID: requestID, rows: rows, cache: cache, empty: result.Empty}
		}
	}

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

	m.setDiffText("Loading commit diff...")
	showHunksOnly := m.showHunksOnly
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
		defer cancel()

		result, err := m.diffs.LoadCommit(ctx, req)
		if err != nil {
			return diffLoadedMsg{requestID: requestID, err: err}
		}
		if result.IsBinary {
			return diffLoadedMsg{requestID: requestID, lines: []string{"Binary file diff"}, isBinary: true, empty: result.Empty}
		}

		full, fullErr := m.diffs.LoadCommitFile(ctx, req)
		if fullErr != nil {
			fallback := "Path: " + activeRef + "\n\n" + m.renderDiff(result)
			return diffLoadedMsg{requestID: requestID, lines: strings.Split(fallback, "\n"), empty: result.Empty}
		}

		ranges := git.ParseChangedLineRangesFromPatch(result.Patch)
		decor := git.ParseLineDecorationsFromPatch(result.Patch)
		cache := &cachedDiffView{header: []string{"Path: " + activeRef, ""}, path: file.Path, fullContent: full, changed: ranges, decor: decor, hunksOnly: showHunksOnly, contextLines: 5}
		rows := m.buildCachedDiffRows(cache)
		return diffLoadedMsg{requestID: requestID, rows: rows, cache: cache, empty: result.Empty}
	}
}

func (m *Model) currentAnchorTime() (time.Time, bool) {
	if len(m.rootHistory) == 0 {
		return time.Time{}, false
	}
	if m.anchorIndex < 0 || m.anchorIndex >= len(m.rootHistory) {
		return time.Time{}, false
	}
	return m.rootHistory[m.anchorIndex].When, true
}

func (m *Model) effectiveSinceMode() bool {
	return m.useSinceMode || m.recentWindow <= 0
}

func (m *Model) autoLoadSelectedDiff() tea.Cmd {
	if m.leftTab == leftTabCommits {
		return nil
	}
	selectionID := m.currentSelectionID()
	if selectionID == "" {
		m.activeRef = ""
		m.setDiffText("No current file selection.\n\nUse [ ] to switch to Commits.")
		m.message = "No file selected"
		return nil
	}
	if selectionID == m.lastSelID {
		return nil
	}
	m.lastSelID = selectionID
	if !m.currentSelectionIsFile() {
		if row := m.currentTreeRow(); row != nil {
			m.activeRef = treeLabelOnly(row.text)
			m.setDiffText(m.renderTreeSelectionSummary(*row))
			m.message = "Folder summary"
		}
		return nil
	}

	item := m.selectedItem()
	if item == nil {
		file := m.selectedRecentFile()
		if file == nil {
			return nil
		}
		return m.loadCommitDiffCmd(*file)
	}
	if m.showRecent {
		if _, ok := m.workingKeys[item.ScopeLabel()+"|"+item.Path]; !ok {
			file := m.selectedRecentFile()
			if file != nil {
				return m.loadCommitDiffCmd(*file)
			}
		}
	}
	return m.loadDiffCmd(*item)
}

func (m *Model) currentSelectionID() string {
	if m.selectedTree < 0 || m.selectedTree >= len(m.treeRows) {
		return ""
	}
	return m.treeRows[m.selectedTree].id
}

func (m *Model) currentTreeRow() *treeRow {
	if m.selectedTree < 0 || m.selectedTree >= len(m.treeRows) {
		return nil
	}
	row := m.treeRows[m.selectedTree]
	return &row
}

func (m *Model) currentSelectionIsFile() bool {
	row := m.currentTreeRow()
	if row == nil {
		return false
	}
	return row.kind == treeKindFile
}

func (m *Model) renderTreeSelectionSummary(row treeRow) string {
	if row.kind == treeKindFile {
		return m.styles.Muted.Render("File selected")
	}

	if m.showRecent {
		scope, prefix := parseTreeNodeScopePrefix(row)
		files := filterRecentByScopePrefix(m.recentVisible, scope, prefix)
		return renderRecentSummaryBlock(row.text, files)
	}

	scope, prefix := parseTreeNodeScopePrefix(row)
	items := filterChangesByScopePrefix(m.filteredItems, scope, prefix)
	return renderChangeSummaryBlock(row.text, items)
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

func (m *Model) buildFileDiffRows(path string, fullContent string, changed []git.LineRange, decor map[int]git.LineDecoration, hunksOnly bool, contextLines int) []diffRow {
	highlighted := ui.HighlightForPath(path, fullContent, m.colors)
	highlightedLines := strings.Split(highlighted, "\n")
	rawLines := strings.Split(fullContent, "\n")
	if !hunksOnly || len(changed) == 0 {
		return m.buildNumberedDiffRows(rawLines, highlightedLines, nil, decor)
	}

	keep := make([]bool, len(rawLines))
	for _, h := range changed {
		start := h.Start - contextLines
		end := h.End + contextLines
		if start < 1 {
			start = 1
		}
		if end > len(rawLines) {
			end = len(rawLines)
		}
		for i := start; i <= end; i++ {
			keep[i-1] = true
		}
	}

	return m.buildNumberedDiffRows(rawLines, highlightedLines, keep, decor)
}

func (m *Model) buildCachedDiffRows(cache *cachedDiffView) []diffRow {
	if cache == nil {
		return nil
	}
	rows := buildPlainDiffRows(cache.header)
	rows = append(rows, m.buildFileDiffRows(cache.path, cache.fullContent, cache.changed, cache.decor, cache.hunksOnly, cache.contextLines)...)
	return rows
}

func (m *Model) rebuildCachedDiffRows() bool {
	if m.cachedDiff == nil {
		return false
	}
	yOffset := m.diff.YOffset
	xOffset := m.diffXOffset
	m.diffRows = m.buildCachedDiffRows(m.cachedDiff)
	m.diffRawLines = diffRowsToRawLines(m.diffRows)
	m.diffChangeRows = collectChangeRowIndexes(m.diffRows)
	m.diffXOffset = xOffset
	m.refreshDiffViewport()
	m.diff.SetYOffset(yOffset)
	return true
}

func (m *Model) buildNumberedDiffRows(rawLines []string, highlightedLines []string, keep []bool, decor map[int]git.LineDecoration) []diffRow {
	rows := make([]diffRow, 0, len(rawLines)+len(decor))
	skipped := false
	highlightAddedRows, highlightDeletedRows := classifyDiffRowHighlights(rawLines, decor)
	addedFg := ansiFg(m.colors.Added)
	removedFg := ansiFg(m.colors.Removed)
	for i, rawLine := range rawLines {
		if keep != nil && !keep[i] {
			skipped = true
			continue
		}
		if skipped {
			rows = append(rows, diffRow{kind: diffRowGap})
			skipped = false
		}
		if d, ok := decor[i+1]; ok && len(d.DeletedLines) > 0 {
			rows = appendDeletedDiffRows(rows, d.DeletedLines, removedFg, highlightDeletedRows, m.colors)
		}
		line := rawLine
		if i < len(highlightedLines) {
			line = highlightedLines[i]
		}
		line = expandTabsANSI(line, 4)
		marker := " "
		markerFg := ""
		if d, ok := decor[i+1]; ok {
			switch {
			case d.Added && d.Deleted:
				marker = "+"
				markerFg = addedFg
			case d.Added:
				marker = "+"
				markerFg = addedFg
			case d.Deleted:
				marker = "-"
				markerFg = removedFg
			}
		}
		lineForRow := line
		lineForRow = preserveBackgroundAcrossResets(lineForRow)
		row := diffRow{kind: diffRowCode, marker: marker, lineNo: strconv.Itoa(i + 1), code: lineForRow, markerFg: markerFg}
		if d, ok := decor[i+1]; ok {
			switch {
			case d.Added && d.Deleted:
				if highlightAddedRows {
					row.codeBg = ansiBg(m.colors.RowAddedBg)
					row.gutterBg = ansiBg(darkenHexColor(m.colors.RowAddedBg, 0.72))
				}
				row.isChange = true
			case d.Added:
				if highlightAddedRows {
					row.codeBg = ansiBg(m.colors.RowAddedBg)
					row.gutterBg = ansiBg(darkenHexColor(m.colors.RowAddedBg, 0.72))
				}
				row.isChange = true
			case d.Deleted:
				if highlightDeletedRows {
					row.codeBg = ansiBg(m.colors.RowRemovedBg)
					row.gutterBg = ansiBg(darkenHexColor(m.colors.RowRemovedBg, 0.72))
				}
				row.isChange = true
			}
		}
		rows = append(rows, row)
	}
	if d, ok := decor[len(rawLines)+1]; ok && len(d.DeletedLines) > 0 {
		rows = appendDeletedDiffRows(rows, d.DeletedLines, removedFg, highlightDeletedRows, m.colors)
	}

	if len(rows) == 0 {
		return buildPlainDiffRows([]string{"No lines to display"})
	}

	return rows
}

func appendDeletedDiffRows(rows []diffRow, deletedLines []string, removedFg string, highlightDeletedRows bool, colors theme.Tokens) []diffRow {
	for _, deletedLine := range deletedLines {
		highlightedDeleted := ui.HighlightForPath("diff", deletedLine, colors)
		highlightedDeleted = expandTabsANSI(highlightedDeleted, 4)
		highlightedDeleted = preserveBackgroundAcrossResets(highlightedDeleted)
		rows = append(rows, diffRow{
			kind:     diffRowCode,
			marker:   "-",
			lineNo:   "",
			code:     highlightedDeleted,
			markerFg: removedFg,
			codeBg:   chooseTint(highlightDeletedRows, ansiBg(colors.RowRemovedBg)),
			gutterBg: chooseTint(highlightDeletedRows, ansiBg(darkenHexColor(colors.RowRemovedBg, 0.72))),
			isChange: true,
		})
	}
	return rows
}

func classifyDiffRowHighlights(rawLines []string, decor map[int]git.LineDecoration) (highlightAddedRows bool, highlightDeletedRows bool) {
	hasAdded := false
	hasDeleted := false
	allRawLinesAdded := len(rawLines) > 0
	allRawLinesDeleted := len(rawLines) > 0
	for i := range rawLines {
		d, ok := decor[i+1]
		if ok && d.Added {
			hasAdded = true
		} else {
			allRawLinesAdded = false
		}
		if ok && d.Deleted {
			hasDeleted = true
		} else {
			allRawLinesDeleted = false
		}
	}
	for _, d := range decor {
		if d.Added {
			hasAdded = true
		}
		if d.Deleted || len(d.DeletedLines) > 0 {
			hasDeleted = true
		}
	}
	highlightAddedRows = hasAdded && !allRawLinesAdded
	highlightDeletedRows = hasDeleted && !allRawLinesDeleted
	return highlightAddedRows, highlightDeletedRows
}

func chooseTint(enabled bool, color string) string {
	if !enabled {
		return ""
	}
	return color
}

func buildPlainDiffRows(lines []string) []diffRow {
	rows := make([]diffRow, 0, len(lines))
	for _, line := range lines {
		rows = append(rows, diffRow{kind: diffRowPlain, code: line})
	}
	if len(rows) == 0 {
		rows = append(rows, diffRow{kind: diffRowPlain, code: ""})
	}
	return rows
}

func diffRowsToRawLines(rows []diffRow) []string {
	lines := make([]string, 0, len(rows))
	for _, row := range rows {
		switch row.kind {
		case diffRowPlain:
			lines = append(lines, row.code)
		default:
			lines = append(lines, row.marker+" "+row.lineNo+" "+row.code)
		}
	}
	return lines
}

func (m *Model) setDiffText(text string) {
	m.cachedDiff = nil
	m.diffRawLines = strings.Split(text, "\n")
	m.diffRows = buildPlainDiffRows(m.diffRawLines)
	m.diffXOffset = 0
	m.diffChangeRows = collectChangeRowIndexes(m.diffRows)
	m.refreshDiffViewport()
}

func (m *Model) renderDiffBody(width int, height int, bg lipgloss.Color) string {
	bodyWidth := maxInt(width, 1)
	showScroll := len(m.diffRows) > height
	contentWidth := bodyWidth
	if showScroll {
		contentWidth = maxInt(bodyWidth-1, 1)
	}
	start := m.diff.YOffset
	if start < 0 {
		start = 0
	}
	maxStart := maxInt(len(m.diffRows)-height, 0)
	if start > maxStart {
		start = maxStart
	}
	lineNoDigits := diffLineNumberDigits(m.diffRows)
	codeWidth := diffCodeColumnWidth(contentWidth, lineNoDigits)
	contentLines := make([]string, 0, height)
	for i := 0; i < height; i++ {
		idx := start + i
		if idx >= 0 && idx < len(m.diffRows) {
			contentLines = append(contentLines, m.renderVisibleDiffRow(m.diffRows[idx], contentWidth, lineNoDigits, codeWidth))
		} else {
			contentLines = append(contentLines, strings.Repeat(" ", contentWidth))
		}
	}
	if !showScroll {
		return strings.Join(contentLines, "\n")
	}

	scrollRailStyle := lipgloss.NewStyle().Background(bg).Foreground(lipgloss.Color(m.colors.Muted))
	scrollThumbStyle := lipgloss.NewStyle().Background(bg).Foreground(lipgloss.Color(m.colors.Accent))
	thumbSize := (height * height) / len(m.diffRows)
	if thumbSize < 1 {
		thumbSize = 1
	}
	if thumbSize > height {
		thumbSize = height
	}
	thumbMaxStart := height - thumbSize
	if thumbMaxStart < 0 {
		thumbMaxStart = 0
	}
	thumbStart := 0
	denom := len(m.diffRows) - height
	if denom > 0 {
		thumbStart = (start * thumbMaxStart) / denom
	}
	thumbEnd := thumbStart + thumbSize

	b := strings.Builder{}
	for i := 0; i < height; i++ {
		b.WriteString(contentLines[i])
		if i >= thumbStart && i < thumbEnd {
			b.WriteString(scrollThumbStyle.Render("█"))
		} else {
			b.WriteString(scrollRailStyle.Render("│"))
		}
		if i < height-1 {
			b.WriteByte('\n')
		}
	}
	return b.String()
}

func (m *Model) refreshDiffViewport() {
	if m.diff.Width <= 0 {
		m.diff.SetContent(renderPlainDiffRows(m.diffRows))
		return
	}
	if len(m.diffRows) == 0 {
		m.diff.SetContent("")
		return
	}
	m.diff.SetContent(renderPlainDiffRows(m.diffRows))
}

func renderPlainDiffRows(rows []diffRow) string {
	parts := make([]string, 0, len(rows))
	for _, row := range rows {
		parts = append(parts, row.code)
	}
	return strings.Join(parts, "\n")
}

func diffLineNumberDigits(rows []diffRow) int {
	digits := 1
	for _, row := range rows {
		if row.kind != diffRowCode {
			continue
		}
		if n := len(strings.TrimSpace(row.lineNo)); n > digits {
			digits = n
		}
	}
	return digits
}

func diffCodeColumnWidth(totalWidth int, lineNoDigits int) int {
	const markerWidth = 2
	gutterWidth := lineNoDigits + 2
	codeWidth := totalWidth - markerWidth - gutterWidth
	if codeWidth < 1 {
		codeWidth = 1
	}
	return codeWidth
}

func collectChangeRowIndexes(rows []diffRow) []int {
	out := make([]int, 0)
	for i, row := range rows {
		if row.isChange {
			out = append(out, i)
		}
	}
	return out
}

func (m *Model) renderVisibleDiffRow(row diffRow, totalWidth int, lineNoDigits int, codeWidth int) string {
	defaultBg := ansiBg(m.colors.PanelRightBg)
	if row.kind == diffRowPlain {
		visible := ansi.Cut(row.code, m.diffXOffset, m.diffXOffset+totalWidth)
		return sanitizeRenderedDiffRow(clampRenderedDiffRow(padStyledCodeSegment(defaultBg, visible, totalWidth), totalWidth))
	}
	if row.kind == diffRowGap {
		const markerWidth = 2
		gutterWidth := lineNoDigits + 2
		markerSeg := renderTintedTextSegment("", markerWidth, "", defaultBg)
		lineSeg := renderTintedTextSegment("", gutterWidth, "", defaultBg)
		gapCode := buildGapPattern(codeWidth)
		codeSeg := renderTintedTextSegment(gapCode, codeWidth, ansiFg(m.colors.Muted), defaultBg)
		return sanitizeRenderedDiffRow(clampRenderedDiffRow(markerSeg+lineSeg+codeSeg, totalWidth))
	}

	const markerWidth = 2
	markerBg := row.gutterBg
	if markerBg == "" {
		markerBg = defaultBg
	}
	markerText := row.marker
	if markerText == "" {
		markerText = " "
	}
	markerSeg := renderTintedTextSegment(markerText+" ", markerWidth, row.markerFg, markerBg)

	gutterBg := row.gutterBg
	if gutterBg == "" {
		gutterBg = defaultBg
	}
	lineNoText := fmt.Sprintf("%*s ", lineNoDigits, row.lineNo) + ansiFg(m.colors.LineSep) + "│\x1b[39m"
	lineSeg := renderTintedTextSegment(lineNoText, lineNoDigits+2, ansiFg(m.colors.Muted), gutterBg)

	codeBg := row.codeBg
	if codeBg == "" {
		codeBg = defaultBg
	}
	visibleCode := ansi.Cut(row.code, m.diffXOffset, m.diffXOffset+codeWidth)
	codeSeg := padStyledCodeSegment(codeBg, visibleCode, codeWidth)
	return sanitizeRenderedDiffRow(clampRenderedDiffRow(markerSeg+lineSeg+codeSeg, totalWidth))
}

func renderTintedTextSegment(text string, width int, fg string, bg string) string {
	if width <= 0 {
		return ""
	}
	visible := text
	pad := width - ansi.StringWidth(visible)
	if pad > 0 {
		visible += strings.Repeat(" ", pad)
	}
	prefix := ""
	if bg != "" {
		prefix += bg
	}
	if fg != "" {
		prefix += fg
	}
	if prefix == "" {
		return visible
	}
	return prefix + visible + "\x1b[0m"
}

func padStyledCodeSegment(bg string, visible string, width int) string {
	if width <= 0 {
		return ""
	}
	pad := width - ansi.StringWidth(visible)
	if pad < 0 {
		pad = 0
	}
	spaces := strings.Repeat(" ", pad)
	if bg == "" {
		return visible + spaces
	}
	visible = preserveBackgroundAcrossResets(visible)
	return bg + visible + spaces + "\x1b[0m"
}

func buildGapPattern(width int) string {
	if width <= 0 {
		return ""
	}
	pattern := "~"
	if width == 1 {
		return pattern
	}
	b := strings.Builder{}
	for ansi.StringWidth(b.String()) < width {
		b.WriteString(pattern)
		b.WriteString(" ")
	}
	return ansi.Cut(strings.TrimSpace(b.String()), 0, width)
}

func clampRenderedDiffRow(row string, width int) string {
	if width <= 0 {
		return ""
	}
	clamped := ansi.Cut(row, 0, width)
	pad := width - ansi.StringWidth(clamped)
	if pad > 0 {
		clamped += strings.Repeat(" ", pad)
	}
	return clamped
}

func sanitizeRenderedDiffRow(row string) string {
	row = strings.ReplaceAll(row, "\r", "")
	row = strings.ReplaceAll(row, "\n", "")
	return row
}

func (m *Model) shiftDiffX(delta int) {
	if delta == 0 {
		return
	}
	lineNoDigits := diffLineNumberDigits(m.diffRows)
	codeWidth := diffCodeColumnWidth(m.diff.Width, lineNoDigits)
	maxOffset := 0
	for _, row := range m.diffRows {
		visibleWidth := codeWidth
		if row.kind == diffRowPlain {
			visibleWidth = m.diff.Width
		}
		overflow := ansi.StringWidth(row.code) - visibleWidth
		if overflow > maxOffset {
			maxOffset = overflow
		}
	}
	if maxOffset <= 0 {
		m.diffXOffset = 0
		m.refreshDiffViewport()
		return
	}
	m.diffXOffset += delta
	if m.diffXOffset < 0 {
		m.diffXOffset = 0
	}
	if m.diffXOffset > maxOffset {
		m.diffXOffset = maxOffset
	}
	m.refreshDiffViewport()
}

func (m *Model) jumpToChange(forward bool) bool {
	if len(m.diffChangeRows) == 0 {
		return false
	}
	current := m.diff.YOffset
	if forward {
		for _, row := range m.diffChangeRows {
			if row > current {
				m.diff.SetYOffset(row)
				return true
			}
		}
		m.diff.SetYOffset(m.diffChangeRows[0])
		return true
	}
	for i := len(m.diffChangeRows) - 1; i >= 0; i-- {
		row := m.diffChangeRows[i]
		if row < current {
			m.diff.SetYOffset(row)
			return true
		}
	}
	m.diff.SetYOffset(m.diffChangeRows[len(m.diffChangeRows)-1])
	return true
}

func (m *Model) renderTreePanel() string {
	if m.leftHeight <= 0 {
		return ""
	}
	bg := lipgloss.Color(m.colors.PanelLeftBg)
	normalStyle := lipgloss.NewStyle().Background(bg)
	selectedStyle := lipgloss.NewStyle().Background(bg).Foreground(lipgloss.Color(m.colors.Accent)).Bold(true)
	folderStyle := lipgloss.NewStyle().Background(bg).Foreground(lipgloss.Color(m.colors.Muted))
	fileStyle := lipgloss.NewStyle().Background(bg).Foreground(lipgloss.Color(m.colors.Fg))
	scrollRailStyle := lipgloss.NewStyle().Background(bg).Foreground(lipgloss.Color(m.colors.Muted))
	scrollThumbStyle := lipgloss.NewStyle().Background(bg).Foreground(lipgloss.Color(m.colors.Accent))

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
	showScroll := len(m.treeRows) > m.leftHeight
	contentWidth := renderWidth
	thumbStart := 0
	thumbEnd := 0
	if showScroll {
		contentWidth = maxInt(renderWidth-1, 4)
		thumbSize := (m.leftHeight * m.leftHeight) / len(m.treeRows)
		if thumbSize < 1 {
			thumbSize = 1
		}
		if thumbSize > m.leftHeight {
			thumbSize = m.leftHeight
		}
		maxStart := m.leftHeight - thumbSize
		if maxStart < 0 {
			maxStart = 0
		}
		denom := len(m.treeRows) - m.leftHeight
		if denom > 0 {
			thumbStart = (start * maxStart) / denom
		}
		thumbEnd = thumbStart + thumbSize
	}
	printed := 0
	for i := start; i < end; i++ {
		lineRow := printed
		row := m.treeRows[i]
		prefix := "  "
		if i == m.selectedTree {
			prefix = "❯ "
		}
		rowText := row.text
		if row.kind == treeKindScope {
			scope := strings.TrimPrefix(row.nodeID, "scope|")
			if icon := m.scanIconForScope(scope); icon != "" {
				rowText = renderScopeScanPrefix(row.text, icon)
			}
		}
		status := " "
		if row.kind == treeKindFile {
			status = row.status
		}
		line := truncateText(prefix+status+" "+rowText, contentWidth)
		if row.kind == treeKindFile {
			if i == m.selectedTree {
				line = m.renderSelectedTreeFileLine(prefix, status, rowText, contentWidth)
			} else {
				line = strings.Replace(line, status+" ", statusWithColor(status, m.colors)+" ", 1)
			}
		}
		if i == m.selectedTree && row.kind == treeKindFile {
			b.WriteString(padANSIWidth(line, contentWidth))
		} else if i == m.selectedTree {
			b.WriteString(selectedStyle.Width(contentWidth).Render(line))
		} else if row.kind == treeKindScope || row.kind == treeKindDir {
			b.WriteString(folderStyle.Width(contentWidth).Render(line))
		} else {
			b.WriteString(fileStyle.Width(contentWidth).Render(line))
		}
		if showScroll {
			if lineRow >= thumbStart && lineRow < thumbEnd {
				b.WriteString(scrollThumbStyle.Render("█"))
			} else {
				b.WriteString(scrollRailStyle.Render("│"))
			}
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
		b.WriteString(normalStyle.Width(contentWidth).Render(""))
		if showScroll {
			if printed >= thumbStart && printed < thumbEnd {
				b.WriteString(scrollThumbStyle.Render("█"))
			} else {
				b.WriteString(scrollRailStyle.Render("│"))
			}
		}
		printed++
	}

	return b.String()
}

func (m *Model) renderSelectedTreeFileLine(prefix string, status string, rowText string, width int) string {
	available := maxInt(width-ansi.StringWidth(prefix)-2, 1)
	label := truncateText(rowText, available)
	accent := "\x1b[1;38;2;" + hexToRGBAnsi(m.colors.Accent) + "m"
	clear := "\x1b[22;39m"
	line := accent + prefix + clear + statusWithColor(status, m.colors) + accent + label + clear
	return padANSIWidth(line, width)
}

func padANSIWidth(input string, width int) string {
	if width <= 0 {
		return ""
	}
	pad := width - ansi.StringWidth(input)
	if pad <= 0 {
		return input
	}
	return input + strings.Repeat(" ", pad)
}

func (m *Model) resetSelection() {
	m.lastSelID = ""
	m.leftOffset = 0
	if len(m.treeRows) == 0 {
		m.selectedTree = -1
	} else {
		m.selectedTree = 0
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
	m.selectedTree += delta
	if m.selectedTree < 0 {
		m.selectedTree = 0
	}
	if m.selectedTree >= len(m.treeRows) {
		m.selectedTree = len(m.treeRows) - 1
	}
	m.ensureSelectionVisible()
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

func (m *Model) collapseAllTreeNodes() bool {
	if len(m.treeRows) == 0 {
		return false
	}
	changed := false
	for _, row := range m.treeRows {
		if row.kind == treeKindScope || row.kind == treeKindDir {
			if !m.treeCollapsed[row.nodeID] {
				m.treeCollapsed[row.nodeID] = true
				changed = true
			}
		}
	}
	return changed
}

func (m *Model) expandAllTreeNodes() bool {
	if len(m.treeCollapsed) == 0 {
		return false
	}
	m.treeCollapsed = map[string]bool{}
	return true
}

func buildChangeTreeRows(changes []model.ChangeItem, scopeBranches map[string]string, collapsed map[string]bool, rootName string) []treeRow {
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
		header := scopeNodeLabel(scope, scopeBranches, rootName)
		rows = append(rows, treeRow{
			id:         nodeID,
			nodeID:     nodeID,
			parentID:   "",
			depth:      0,
			kind:       treeKindScope,
			text:       icon + " " + header,
			selectable: true,
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
		child, label := compactFolderChain(n.Folders[name], scope, collapsed)
		nodeID := "dir|" + scope + "|" + child.Path
		expanded := !collapsed[nodeID]
		icon := "▾"
		if !expanded {
			icon = "▸"
		}
		rows = append(rows, treeRow{
			id:         nodeID,
			nodeID:     nodeID,
			parentID:   parentID,
			depth:      depth,
			kind:       treeKindDir,
			text:       indent + icon + " " + label,
			selectable: true,
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
		status := displayStatusChange(f)
		rows = append(rows, treeRow{
			id:         "file|" + f.ScopeLabel() + "|" + string(f.Type) + "|" + f.Path,
			nodeID:     "file|" + f.ScopeLabel() + "|" + f.Path,
			parentID:   parentID,
			depth:      depth,
			kind:       treeKindFile,
			status:     status,
			text:       indent + baseName(f.Path),
			selectable: true,
			change:     &copy,
		})
	}

	return rows
}

func compactFolderChain(n *treeBuildNode, scope string, collapsed map[string]bool) (*treeBuildNode, string) {
	label := n.Name
	cur := n
	for len(cur.Files) == 0 && len(cur.Folders) == 1 && !collapsed["dir|"+scope+"|"+cur.Path] {
		nextName := ""
		var next *treeBuildNode
		for name, child := range cur.Folders {
			nextName = name
			next = child
		}
		if next == nil || collapsed["dir|"+scope+"|"+next.Path] {
			break
		}
		label += "/" + nextName
		cur = next
	}
	return cur, label
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

func buildRecentTreeRows(files []model.CommitFile, scopeBranches map[string]string, collapsed map[string]bool, rootName string) []treeRow {
	changes := make([]model.ChangeItem, 0, len(files))
	fileMap := map[string]model.CommitFile{}
	for _, f := range files {
		changes = append(changes, commitFileToChangeItem(f, ""))
		key := f.Scope + "|" + f.Path
		fileMap[key] = f
	}
	rows := buildChangeTreeRows(changes, scopeBranches, collapsed, rootName)
	for i := range rows {
		if rows[i].change != nil {
			key := rows[i].change.ScopeLabel() + "|" + rows[i].change.Path
			if f, ok := fileMap[key]; ok {
				copy := f
				rows[i].commitFile = &copy
				rows[i].change = nil
				rows[i].id = "recent|" + f.Scope + "|" + f.CommitHash + "|" + f.Path
				rows[i].status = normalizeRecentStatus(f.Status)
				rows[i].text = strings.Repeat("  ", rows[i].depth) + baseName(f.Path) + "  [" + shortHash(f.CommitHash) + "]"
			}
		}
	}
	return rows
}

func commitFileToChangeItem(f model.CommitFile, repoRoot string) model.ChangeItem {
	ctype := model.ChangeTypeUnstaged
	status := strings.ToUpper(strings.TrimSpace(f.Status))
	submodulePath := f.SubmodulePath
	if !f.IsRoot {
		if strings.TrimSpace(submodulePath) == "" {
			submodulePath = f.Scope
		}
	}
	item := model.ChangeItem{
		RepoRoot:      repoRoot,
		Path:          f.Path,
		Type:          ctype,
		SubmodulePath: submodulePath,
	}
	if f.IsRoot {
		item.SubmodulePath = ""
	}
	if strings.HasPrefix(status, "A") {
		item.WorktreeStatus = 'A'
	} else if strings.HasPrefix(status, "D") {
		item.WorktreeStatus = 'D'
	} else {
		item.WorktreeStatus = 'M'
	}
	return item
}

func scopeNodeLabel(scope string, scopeBranches map[string]string, rootName string) string {
	branch := valueOr(scopeBranches[scope], "?")
	if scope == "root" {
		name := strings.TrimSpace(rootName)
		if name == "" || name == "." || name == string(filepath.Separator) {
			name = "root"
		}
		return fmt.Sprintf("%s (%s)", name, branch)
	}
	return fmt.Sprintf("%s (%s)", scope, branch)
}

func normalizeRecentStatus(status string) string {
	s := strings.ToUpper(strings.TrimSpace(status))
	if s == "A" || s == "M" || s == "D" {
		return s
	}
	return "M"
}

func displayStatusChange(c model.ChangeItem) string {
	if c.StagedStatus == 'D' || c.WorktreeStatus == 'D' {
		return "D"
	}
	if c.Type == model.ChangeTypeUntracked || c.StagedStatus == 'A' || c.WorktreeStatus == 'A' {
		return "A"
	}
	return "M"
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

func parseTreeNodeScopePrefix(row treeRow) (scope string, prefix string) {
	if strings.HasPrefix(row.nodeID, "scope|") {
		parts := strings.SplitN(row.nodeID, "|", 2)
		if len(parts) == 2 {
			return parts[1], ""
		}
		return "root", ""
	}
	if strings.HasPrefix(row.nodeID, "dir|") {
		parts := strings.SplitN(row.nodeID, "|", 3)
		if len(parts) == 3 {
			return parts[1], parts[2]
		}
	}
	if row.change != nil {
		return row.change.ScopeLabel(), row.change.Path
	}
	if row.commitFile != nil {
		return row.commitFile.Scope, row.commitFile.Path
	}
	return "root", ""
}

func filterChangesByScopePrefix(items []model.ChangeItem, scope string, prefix string) []model.ChangeItem {
	out := make([]model.ChangeItem, 0)
	prefix = strings.Trim(prefix, "/")
	for _, item := range items {
		if item.ScopeLabel() != scope {
			continue
		}
		if prefix != "" && !strings.HasPrefix(item.Path, prefix+"/") && item.Path != prefix {
			continue
		}
		out = append(out, item)
	}
	return out
}

func filterRecentByScopePrefix(files []model.CommitFile, scope string, prefix string) []model.CommitFile {
	out := make([]model.CommitFile, 0)
	prefix = strings.Trim(prefix, "/")
	for _, file := range files {
		if file.Scope != scope {
			continue
		}
		if prefix != "" && !strings.HasPrefix(file.Path, prefix+"/") && file.Path != prefix {
			continue
		}
		out = append(out, file)
	}
	return out
}

func treeLabelOnly(text string) string {
	trimmed := strings.TrimSpace(text)
	trimmed = strings.TrimPrefix(trimmed, "▾ ")
	trimmed = strings.TrimPrefix(trimmed, "▸ ")
	return strings.TrimSpace(trimmed)
}

func renderChangeSummaryBlock(label string, items []model.ChangeItem) string {
	b := strings.Builder{}
	b.WriteString("Folder: ")
	b.WriteString(treeLabelOnly(label))
	b.WriteString("\n\n")
	if len(items) == 0 {
		b.WriteString("No changed files in this folder")
		return b.String()
	}

	counts := map[model.ChangeType]int{}
	paths := make([]string, 0, len(items))
	for _, item := range items {
		counts[item.Type]++
		paths = append(paths, fmt.Sprintf("%s  %s", displayStatusChange(item), item.Path))
	}
	sort.Strings(paths)
	b.WriteString(fmt.Sprintf("Files: %d  (S:%d M:%d A:%d)\n\n", len(items), counts[model.ChangeTypeStaged], counts[model.ChangeTypeUnstaged], counts[model.ChangeTypeUntracked]))
	maxList := 80
	if len(paths) < maxList {
		maxList = len(paths)
	}
	for i := 0; i < maxList; i++ {
		b.WriteString(paths[i])
		if i < maxList-1 {
			b.WriteByte('\n')
		}
	}
	if len(paths) > maxList {
		b.WriteString(fmt.Sprintf("\n... and %d more", len(paths)-maxList))
	}
	return b.String()
}

func renderRecentSummaryBlock(label string, files []model.CommitFile) string {
	b := strings.Builder{}
	b.WriteString("Folder: ")
	b.WriteString(treeLabelOnly(label))
	b.WriteString("\n\n")
	if len(files) == 0 {
		b.WriteString("No recent commit files in this folder")
		return b.String()
	}
	b.WriteString(fmt.Sprintf("Files: %d\n\n", len(files)))

	list := make([]string, 0, len(files))
	for _, f := range files {
		list = append(list, fmt.Sprintf("%s  %s", shortHash(f.CommitHash), f.Path))
	}
	sort.Strings(list)
	maxList := 80
	if len(list) < maxList {
		maxList = len(list)
	}
	for i := 0; i < maxList; i++ {
		b.WriteString(list[i])
		if i < maxList-1 {
			b.WriteByte('\n')
		}
	}
	if len(list) > maxList {
		b.WriteString(fmt.Sprintf("\n... and %d more", len(list)-maxList))
	}
	return b.String()
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
	line := strings.Repeat("│\n", height)
	return strings.TrimRight(line, "\n")
}

func statusWithColor(status string, colors theme.Tokens) string {
	switch status {
	case "A":
		return ansiFg(colors.Added) + "A\x1b[39m"
	case "D":
		return ansiFg(colors.Removed) + "D\x1b[39m"
	case "M":
		return ansiFg(colors.Modified) + "M\x1b[39m"
	default:
		return status
	}
}

func ansiFg(hex string) string {
	return "\x1b[38;2;" + hexToRGBAnsi(hex) + "m"
}

func ansiBg(hex string) string {
	return "\x1b[48;2;" + hexToRGBAnsi(hex) + "m"
}

func hexToRGBAnsi(hex string) string {
	r, g, b, ok := hexToRGB(hex)
	if !ok {
		return "201;209;217"
	}
	return fmt.Sprintf("%d;%d;%d", r, g, b)
}

func darkenHexColor(hex string, factor float64) string {
	r, g, b, ok := hexToRGB(hex)
	if !ok {
		return hex
	}
	if factor < 0 {
		factor = 0
	}
	if factor > 1 {
		factor = 1
	}
	return fmt.Sprintf("#%02x%02x%02x", int(float64(r)*factor), int(float64(g)*factor), int(float64(b)*factor))
}

func hexToRGB(hex string) (int, int, int, bool) {
	h := strings.TrimPrefix(strings.TrimSpace(hex), "#")
	if len(h) != 6 {
		return 0, 0, 0, false
	}
	return parseHexByte(h[0:2]), parseHexByte(h[2:4]), parseHexByte(h[4:6]), true
}

func parseHexByte(v string) int {
	if len(v) != 2 {
		return 0
	}
	n, err := strconv.ParseInt(v, 16, 32)
	if err != nil {
		return 0
	}
	return int(n)
}

func preserveBackgroundAcrossResets(input string) string {
	if input == "" {
		return input
	}
	input = strings.ReplaceAll(input, "\x1b[0m", "\x1b[39m")
	return strings.ReplaceAll(input, "\x1b[m", "\x1b[39m")
}

func expandTabsANSI(input string, tabWidth int) string {
	if input == "" || tabWidth <= 0 || !strings.Contains(input, "\t") {
		return input
	}
	b := strings.Builder{}
	col := 0
	for i := 0; i < len(input); i++ {
		if input[i] == '\x1b' && i+1 < len(input) && input[i+1] == '[' {
			j := i + 2
			for j < len(input) && input[j] != 'm' {
				j++
			}
			if j < len(input) {
				b.WriteString(input[i : j+1])
				i = j
				continue
			}
		}
		if input[i] == '\t' {
			spaces := tabWidth - (col % tabWidth)
			if spaces <= 0 {
				spaces = tabWidth
			}
			b.WriteString(strings.Repeat(" ", spaces))
			col += spaces
			continue
		}
		b.WriteByte(input[i])
		col += ansi.StringWidth(string(input[i]))
	}
	return b.String()
}

var scanSpinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

func (m *Model) rootScanLabel() string {
	name := strings.TrimSpace(m.rootName)
	if name == "" || name == "." || name == string(filepath.Separator) {
		return "root"
	}
	return name
}

func (m *Model) markScanRootDone() {
	if len(m.scanStates) == 0 {
		m.scanStates = []scanState{{path: "root", label: m.rootScanLabel(), status: "done"}}
		return
	}
	m.scanStates[0].status = "done"
	m.scanStates[0].err = ""
}

func (m *Model) setSubmoduleScanPaths(paths []string) {
	root := scanState{path: "root", label: m.rootScanLabel(), status: "done"}
	if len(m.scanStates) > 0 {
		root = m.scanStates[0]
	}
	states := make([]scanState, 0, len(paths)+1)
	states = append(states, root)
	for _, p := range paths {
		states = append(states, scanState{path: p, label: p, status: "pending"})
	}
	m.scanStates = states
	m.scanActive = true
}

func (m *Model) markScanStatus(path string, status string, errText string) {
	for i := range m.scanStates {
		if m.scanStates[i].path != path {
			continue
		}
		m.scanStates[i].status = status
		m.scanStates[i].err = errText
		return
	}
	m.scanStates = append(m.scanStates, scanState{path: path, label: path, status: status, err: errText})
}

func (m *Model) scanAllDone() bool {
	if len(m.scanStates) == 0 {
		return true
	}
	for i := 1; i < len(m.scanStates); i++ {
		if m.scanStates[i].status == "pending" || m.scanStates[i].status == "loading" {
			return false
		}
	}
	return true
}

func (m *Model) scanProgressCounts() (int, int) {
	if len(m.scanStates) <= 1 {
		return 0, 0
	}
	done := 0
	total := len(m.scanStates) - 1
	for i := 1; i < len(m.scanStates); i++ {
		s := m.scanStates[i].status
		if s == "done" || s == "error" {
			done++
		}
	}
	return done, total
}

func (m *Model) renderScanStatus() string {
	if len(m.scanStates) == 0 {
		return ""
	}
	maxRows := minInt(maxInt(m.leftHeight/3, 2), len(m.scanStates))
	b := strings.Builder{}
	for i := 0; i < maxRows; i++ {
		state := m.scanStates[i]
		icon := "•"
		switch state.status {
		case "loading":
			icon = scanSpinnerFrames[m.scanSpinner%len(scanSpinnerFrames)]
		case "pending":
			icon = "·"
		case "done":
			icon = "✓"
		case "error":
			icon = "!"
		}
		label := truncateText(state.label, maxInt(m.leftWidth-6, 8))
		line := icon + " " + label
		if state.status == "error" {
			line = line + " (err)"
		}
		b.WriteString(line)
		if i < maxRows-1 {
			b.WriteByte('\n')
		}
	}
	if len(m.scanStates) > maxRows {
		b.WriteString("\n...")
	}
	return b.String()
}

func (m *Model) scanIconForScope(scope string) string {
	for _, state := range m.scanStates {
		if state.path != scope {
			continue
		}
		switch state.status {
		case "loading":
			return scanSpinnerFrames[m.scanSpinner%len(scanSpinnerFrames)]
		case "pending":
			return "·"
		case "done":
			return "✓"
		case "error":
			return "!"
		default:
			return ""
		}
	}
	return ""
}

func renderScopeScanPrefix(text string, icon string) string {
	if icon == "" {
		return text
	}
	if strings.HasPrefix(text, "▾ ") || strings.HasPrefix(text, "▸ ") {
		return text[:len("▾ ")] + icon + " " + text[len("▾ "):]
	}
	return icon + " " + text
}

func (m *Model) renderRecentTabs(width int, bg lipgloss.Color) string {
	filesLabel := "Files"
	commitsLabel := "Commits"
	active := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(m.colors.Accent)).Background(bg)
	inactive := lipgloss.NewStyle().Foreground(lipgloss.Color(m.colors.Muted)).Background(bg)
	separator := lipgloss.NewStyle().Foreground(lipgloss.Color(m.colors.Muted)).Background(bg).Render(" | ")
	if m.leftTab == leftTabFiles {
		filesLabel = active.Render("[" + filesLabel + "]")
		commitsLabel = inactive.Render("[" + commitsLabel + "]")
	} else {
		filesLabel = inactive.Render("[" + filesLabel + "]")
		commitsLabel = active.Render("[" + commitsLabel + "]")
	}
	return lipgloss.NewStyle().Background(bg).Width(width).Render(filesLabel + separator + commitsLabel)
}

func (m *Model) nextLeftTab() {
	if m.leftTab == leftTabFiles {
		m.leftTab = leftTabCommits
		return
	}
	m.leftTab = leftTabFiles
}

func (m *Model) prevLeftTab() {
	m.nextLeftTab()
}

func (m *Model) previewCmdForActiveTab() tea.Cmd {
	if m.leftTab == leftTabCommits {
		return m.loadSelectedCommitSummaryCmd()
	}
	return m.autoLoadSelectedDiff()
}

func (m *Model) ensureCommitCursorInBounds() {
	if len(m.rootHistory) == 0 {
		m.commitCursor = -1
		m.commitOffset = 0
		return
	}
	if m.commitCursor < 0 {
		m.commitCursor = 0
	}
	if m.commitCursor >= len(m.rootHistory) {
		m.commitCursor = len(m.rootHistory) - 1
	}
	if m.commitCursor < m.commitOffset {
		m.commitOffset = m.commitCursor
	}
	if m.leftHeight > 0 && m.commitCursor >= m.commitOffset+m.leftHeight {
		m.commitOffset = m.commitCursor - m.leftHeight + 1
	}
	if m.commitOffset < 0 {
		m.commitOffset = 0
	}
}

func (m *Model) moveCommitSelection(delta int) {
	if len(m.rootHistory) == 0 || delta == 0 {
		return
	}
	if m.commitCursor < 0 {
		m.commitCursor = 0
	}
	m.commitCursor += delta
	if m.commitCursor < 0 {
		m.commitCursor = 0
	}
	if m.commitCursor >= len(m.rootHistory) {
		m.commitCursor = len(m.rootHistory) - 1
	}
	m.ensureCommitCursorInBounds()
}

func (m *Model) pageCommitSelection(direction int) {
	step := maxInt(m.leftHeight-1, 1)
	if direction < 0 {
		step = -step
	}
	m.moveCommitSelection(step)
}

func (m *Model) selectedRootCommit() *model.RepoCommit {
	if m.commitCursor < 0 || m.commitCursor >= len(m.rootHistory) {
		return nil
	}
	copy := m.rootHistory[m.commitCursor]
	return &copy
}

func (m *Model) renderCommitPanel() string {
	if len(m.rootHistory) == 0 {
		return m.styles.Muted.Render("No root commits loaded")
	}
	m.ensureCommitCursorInBounds()
	start := m.commitOffset
	if start < 0 {
		start = 0
	}
	end := start + m.leftHeight
	if end > len(m.rootHistory) {
		end = len(m.rootHistory)
	}
	bg := lipgloss.Color(m.colors.PanelLeftBg)
	normalStyle := lipgloss.NewStyle().Background(bg).Foreground(lipgloss.Color(m.colors.Fg))
	selectedStyle := lipgloss.NewStyle().Background(bg).Foreground(lipgloss.Color(m.colors.Accent)).Bold(true)
	scrollRailStyle := lipgloss.NewStyle().Background(bg).Foreground(lipgloss.Color(m.colors.Muted))
	scrollThumbStyle := lipgloss.NewStyle().Background(bg).Foreground(lipgloss.Color(m.colors.Accent))
	b := strings.Builder{}
	renderWidth := maxInt(m.leftWidth, 8)
	showScroll := len(m.rootHistory) > m.leftHeight
	contentWidth := renderWidth
	thumbStart := 0
	thumbEnd := 0
	if showScroll {
		contentWidth = maxInt(renderWidth-1, 4)
		thumbSize := (m.leftHeight * m.leftHeight) / len(m.rootHistory)
		if thumbSize < 1 {
			thumbSize = 1
		}
		if thumbSize > m.leftHeight {
			thumbSize = m.leftHeight
		}
		maxStart := m.leftHeight - thumbSize
		if maxStart < 0 {
			maxStart = 0
		}
		denom := len(m.rootHistory) - m.leftHeight
		if denom > 0 {
			thumbStart = (start * maxStart) / denom
		}
		thumbEnd = thumbStart + thumbSize
	}
	printed := 0
	for i := start; i < end; i++ {
		lineRow := printed
		commit := m.rootHistory[i]
		prefix := "  "
		if i == m.commitCursor {
			prefix = "❯ "
		}
		line := truncateText(prefix+shortHash(commit.Hash)+"  "+commit.Subject, contentWidth)
		if i == m.commitCursor {
			b.WriteString(selectedStyle.Width(contentWidth).Render(line))
		} else {
			b.WriteString(normalStyle.Width(contentWidth).Render(line))
		}
		if showScroll {
			if lineRow >= thumbStart && lineRow < thumbEnd {
				b.WriteString(scrollThumbStyle.Render("█"))
			} else {
				b.WriteString(scrollRailStyle.Render("│"))
			}
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
		b.WriteString(normalStyle.Width(contentWidth).Render(""))
		if showScroll {
			if printed >= thumbStart && printed < thumbEnd {
				b.WriteString(scrollThumbStyle.Render("█"))
			} else {
				b.WriteString(scrollRailStyle.Render("│"))
			}
		}
		printed++
	}
	return b.String()
}

func (m *Model) loadSelectedCommitSummaryCmd() tea.Cmd {
	commit := m.selectedRootCommit()
	if commit == nil {
		return nil
	}
	requestID := m.requestID
	hash := commit.Hash
	m.setDiffText("Loading commit details...")
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		summary, err := git.LoadRootCommitSummary(ctx, m.runner, m.repoPath, hash)
		return commitSummaryLoadedMsg{requestID: requestID, hash: hash, summary: summary, err: err}
	}
}

func renderCommitSummary(summary model.CommitSummary) string {
	b := strings.Builder{}
	b.WriteString("commit ")
	b.WriteString(summary.Hash)
	b.WriteString("\n")
	b.WriteString("Author: ")
	b.WriteString(summary.Author)
	if summary.Email != "" {
		b.WriteString(" <")
		b.WriteString(summary.Email)
		b.WriteString(">")
	}
	b.WriteString("\nDate:   ")
	b.WriteString(summary.When.Local().Format(time.RFC3339))
	b.WriteString("\n\n")
	b.WriteString(summary.Subject)
	b.WriteString("\n")
	if summary.Body != "" {
		b.WriteString("\n")
		b.WriteString(summary.Body)
		b.WriteString("\n")
	}
	if len(summary.Parents) > 0 {
		b.WriteString("\nParents: ")
		b.WriteString(strings.Join(summary.Parents, " "))
		b.WriteString("\n")
	}
	if summary.ShortStat != "" {
		b.WriteString("\nStats: ")
		b.WriteString(summary.ShortStat)
		b.WriteString("\n")
	}
	b.WriteString("\nFiles:\n")
	for i, file := range summary.Files {
		b.WriteString("  ")
		b.WriteString(file.Status)
		b.WriteString("  ")
		b.WriteString(file.Path)
		if i < len(summary.Files)-1 {
			b.WriteByte('\n')
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

func resolveAnchorIndex(history []model.RepoCommit, currentHash string) int {
	if len(history) == 0 {
		return 0
	}
	if strings.TrimSpace(currentHash) == "" {
		return 0
	}
	for i, commit := range history {
		if commit.Hash == currentHash || strings.HasPrefix(commit.Hash, currentHash) {
			return i
		}
	}
	return 0
}
