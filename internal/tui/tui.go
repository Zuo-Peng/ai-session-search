package tui

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/Zuo-Peng/ai-session-search/internal/index"
	"github.com/Zuo-Peng/ai-session-search/internal/search"
)

const debounceDelay = 200 * time.Millisecond

type tuiMode int

const (
	modeSearch tuiMode = iota
	modeList
)

// message types

type searchResultMsg struct {
	query   string
	results []search.Result
	err     error
}

type debounceTickMsg struct {
	query string
}

// model

type model struct {
	db          *index.DB
	searchOpts  search.Options
	mode        tuiMode
	query       string
	results     []search.Result
	cursor      int
	listOffset  int
	filterInput textinput.Model
	preview     viewport.Model
	previewKey  string // "sessionKey:chunkID" to avoid duplicate renders
	width       int
	height      int
	ready       bool
	quitting    bool
	openResult *search.Result
}

func initialModel(db *index.DB, query string, opts search.Options) model {
	ti := textinput.New()
	ti.Placeholder = "Search..."
	ti.Focus()
	ti.SetValue(query)
	ti.Prompt = "> "
	ti.PromptStyle = styleInputPrompt
	ti.TextStyle = styleInput
	ti.CharLimit = 256

	return model{
		db:          db,
		searchOpts:  opts,
		query:       query,
		filterInput: ti,
		preview:     viewport.New(0, 0),
	}
}

// Run starts the TUI and blocks until it exits.
// If the user selects a result, it copies the session ID to clipboard.
func Run(db *index.DB, query string, opts search.Options) error {
	m := initialModel(db, query, opts)
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	finalModel, err := p.Run()
	if err != nil {
		return fmt.Errorf("tui: %w", err)
	}

	fm := finalModel.(model)
	if fm.openResult != nil {
		return copySessionID(db, fm.openResult.SessionKey, fm.openResult.Source)
	}
	return nil
}

// RunList starts the TUI in list mode, showing all sessions sorted by update time.
func RunList(db *index.DB, opts search.Options) error {
	ti := textinput.New()
	ti.Placeholder = "Filter..."
	ti.Focus()
	ti.Prompt = "> "
	ti.PromptStyle = styleInputPrompt
	ti.TextStyle = styleInput
	ti.CharLimit = 256

	m := model{
		db:          db,
		searchOpts:  opts,
		mode:        modeList,
		filterInput: ti,
		preview:     viewport.New(0, 0),
	}
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	finalModel, err := p.Run()
	if err != nil {
		return fmt.Errorf("tui: %w", err)
	}

	fm := finalModel.(model)
	if fm.openResult != nil {
		return copySessionID(db, fm.openResult.SessionKey, fm.openResult.Source)
	}
	return nil
}

// copySessionID extracts the resume session ID from the DB and copies it to clipboard.
func copySessionID(db *index.DB, sessionKey, source string) error {
	session, err := db.GetSessionByKey(sessionKey)
	if err != nil {
		return fmt.Errorf("get session: %w", err)
	}
	if session == nil {
		return fmt.Errorf("session not found: %s", sessionKey)
	}

	// Extract session ID from file name (without .jsonl extension)
	sessionID := strings.TrimSuffix(filepath.Base(session.FilePath), ".jsonl")

	var resumeCmd string
	switch source {
	case "claude":
		resumeCmd = fmt.Sprintf("claude --resume %s", sessionID)
	case "codex":
		// Codex expects UUID only, extract from filename like
		// rollout-2026-01-26T17-30-22-019bf9a3-d433-7fc1-8214-b82613804964
		codexID := extractUUID(sessionID)
		resumeCmd = fmt.Sprintf("codex resume %s", codexID)
	default:
		resumeCmd = sessionID
	}

	// Prepend cd if the session has a working directory
	var fullCmd string
	if session.RepoCwd != "" {
		fullCmd = fmt.Sprintf("cd %s && %s", session.RepoCwd, resumeCmd)
	} else {
		fullCmd = resumeCmd
	}

	if err := clipboard.WriteAll(fullCmd); err != nil {
		fmt.Printf("%s\n", fullCmd)
		return nil
	}

	fmt.Printf("Copied to clipboard: %s\n", fullCmd)
	return nil
}

// uuidRe matches a standard UUID (8-4-4-4-12 hex pattern).
var uuidRe = regexp.MustCompile(`[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`)

// extractUUID extracts a UUID from a string, returning the original if none found.
func extractUUID(s string) string {
	if m := uuidRe.FindString(s); m != "" {
		return m
	}
	return s
}

// Init triggers the initial search/list load.
func (m model) Init() tea.Cmd {
	cmds := []tea.Cmd{textinput.Blink}
	if m.mode == modeList {
		cmds = append(cmds, m.doListAll(""))
	} else if m.query != "" {
		cmds = append(cmds, m.doSearch(m.query))
	}
	return tea.Batch(cmds...)
}

// Update handles messages.
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true
		m.preview = newViewport(m.previewWidth(), m.panelHeight())
		// Re-render preview if we have a selection
		if len(m.results) > 0 && m.cursor < len(m.results) {
			cmds = append(cmds, loadPreviewCmd(m.db, m.results[m.cursor], m.query, m.previewWidth()))
		}
		return m, tea.Batch(cmds...)

	case tea.KeyMsg:
		switch {
		case key.Matches(msg, keys.Quit):
			m.quitting = true
			return m, tea.Quit

		case key.Matches(msg, keys.Enter):
			if len(m.results) > 0 && m.cursor < len(m.results) {
				r := m.results[m.cursor]
				m.openResult = &r
				m.quitting = true
				return m, tea.Quit
			}

		case key.Matches(msg, keys.Up):
			if m.cursor > 0 {
				m.cursor--
				m.adjustListScroll(m.panelHeight())
				cmds = append(cmds, m.loadCurrentPreview())
			}
			return m, tea.Batch(cmds...)

		case key.Matches(msg, keys.Down):
			if m.cursor < len(m.results)-1 {
				m.cursor++
				m.adjustListScroll(m.panelHeight())
				cmds = append(cmds, m.loadCurrentPreview())
			}
			return m, tea.Batch(cmds...)

		case key.Matches(msg, keys.PreviewUp):
			m.preview.LineUp(m.panelHeight() / 2)
			return m, nil

		case key.Matches(msg, keys.PreviewDn):
			m.preview.LineDown(m.panelHeight() / 2)
			return m, nil

		case key.Matches(msg, keys.PageUp):
			m.preview.LineUp(m.panelHeight())
			return m, nil

		case key.Matches(msg, keys.PageDown):
			m.preview.LineDown(m.panelHeight())
			return m, nil
		}

		// Pass remaining keys to text input
		var tiCmd tea.Cmd
		m.filterInput, tiCmd = m.filterInput.Update(msg)
		cmds = append(cmds, tiCmd)

		// Check if query changed
		newQuery := m.filterInput.Value()
		if newQuery != m.query {
			m.query = newQuery
			cmds = append(cmds, m.scheduleDebouncedSearch(newQuery))
		}
		return m, tea.Batch(cmds...)

	case tea.MouseMsg:
		if !m.ready || len(m.results) == 0 {
			return m, nil
		}

		region, itemIdx := m.hitTest(msg.X, msg.Y)

		switch {
		case region == regionList && msg.Button == tea.MouseButtonWheelUp:
			if m.listOffset > 0 {
				m.listOffset--
			}
			return m, nil

		case region == regionList && msg.Button == tea.MouseButtonWheelDown:
			pH := m.panelHeight()
			visibleItems := pH / linesPerItem
			maxOffset := len(m.results) - visibleItems
			if maxOffset < 0 {
				maxOffset = 0
			}
			if m.listOffset < maxOffset {
				m.listOffset++
			}
			return m, nil

		case region == regionList && msg.Button == tea.MouseButtonLeft && msg.Action == tea.MouseActionPress:
			if itemIdx >= 0 && itemIdx < len(m.results) && m.cursor != itemIdx {
				m.cursor = itemIdx
				m.adjustListScroll(m.panelHeight())
				cmds = append(cmds, m.loadCurrentPreview())
			}
			return m, tea.Batch(cmds...)

		case region == regionPreview && (msg.Button == tea.MouseButtonWheelUp || msg.Button == tea.MouseButtonWheelDown):
			var vpCmd tea.Cmd
			m.preview, vpCmd = m.preview.Update(msg)
			if vpCmd != nil {
				cmds = append(cmds, vpCmd)
			}
			return m, tea.Batch(cmds...)
		}

		return m, nil

	case debounceTickMsg:
		// Only fire search if query hasn't changed since debounce was scheduled
		if msg.query == m.query {
			if m.mode == modeList {
				cmds = append(cmds, m.doListAll(msg.query))
			} else {
				cmds = append(cmds, m.doSearch(msg.query))
			}
		}
		return m, tea.Batch(cmds...)

	case searchResultMsg:
		// Only apply if this result matches current query
		if msg.query != m.query {
			return m, nil
		}
		if msg.err != nil {
			// On error, clear results
			m.results = nil
			m.cursor = 0
			m.listOffset = 0
			m.preview.SetContent("Error: " + msg.err.Error())
			m.previewKey = ""
			return m, nil
		}
		m.results = msg.results
		m.cursor = 0
		m.listOffset = 0
		if len(m.results) > 0 {
			cmds = append(cmds, m.loadCurrentPreview())
		} else {
			m.preview.SetContent("")
			m.previewKey = ""
		}
		return m, tea.Batch(cmds...)

	case previewRenderedMsg:
		key := previewCacheKey(msg.sessionKey, msg.chunkID)
		if key == m.previewKey {
			// Already showing this preview, skip
			return m, nil
		}
		// Check if this preview is still the one we want
		if len(m.results) > 0 && m.cursor < len(m.results) {
			r := m.results[m.cursor]
			wantKey := previewCacheKey(r.SessionKey, r.ChunkID)
			if key != wantKey {
				return m, nil // stale preview
			}
		}
		if msg.err != nil {
			m.preview.SetContent("Preview error: " + msg.err.Error())
		} else {
			m.preview.SetContent(msg.content)
			if msg.hitLine > 0 {
				m.preview.SetYOffset(msg.hitLine)
			} else {
				m.preview.GotoTop()
			}
		}
		m.previewKey = key
		return m, nil
	}

	return m, tea.Batch(cmds...)
}

// View renders the full TUI.
func (m model) View() string {
	if m.quitting || !m.ready {
		return ""
	}

	// Layout dimensions
	listW := m.listWidth()
	previewW := m.previewWidth()
	panelH := m.panelHeight()

	// Input row
	inputRow := m.filterInput.View()

	// List panel
	listContent := m.renderList(listW, panelH)
	listPanel := stylePanelBorder.
		Width(listW).
		Height(panelH).
		Render(listContent)

	// Preview panel
	m.preview.Width = previewW
	m.preview.Height = panelH
	previewPanel := styleActiveBorder.
		Width(previewW).
		Height(panelH).
		Render(m.preview.View())

	// Join panels side by side
	panels := lipgloss.JoinHorizontal(lipgloss.Top, listPanel, previewPanel)

	// Status bar
	status := m.statusBar()

	return lipgloss.JoinVertical(lipgloss.Left, inputRow, panels, status)
}

// helper methods

func (m model) listWidth() int {
	if m.width <= 0 {
		return 40
	}
	// 40% for list, minus border padding
	w := m.width*40/100 - 4
	if w < 20 {
		w = 20
	}
	return w
}

func (m model) previewWidth() int {
	if m.width <= 0 {
		return 60
	}
	// 60% for preview, minus border padding
	w := m.width*60/100 - 4
	if w < 20 {
		w = 20
	}
	return w
}

func (m model) panelHeight() int {
	if m.height <= 0 {
		return 20
	}
	// Subtract input row (1) + status bar (1) + borders (4)
	h := m.height - 6
	if h < 5 {
		h = 5
	}
	return h
}

type mouseRegion int

const (
	regionNone mouseRegion = iota
	regionList
	regionPreview
)

// hitTest maps terminal coordinates to a panel region and list item index.
func (m model) hitTest(x, y int) (mouseRegion, int) {
	pH := m.panelHeight()
	contentYStart := 2 // input row (1) + top border (1)
	contentYEnd := contentYStart + pH - 1

	if y < contentYStart || y > contentYEnd {
		return regionNone, -1
	}
	relY := y - contentYStart

	lw := m.listWidth()
	listBoxRight := lw + 1 // col 0=border, 1..lw=content, lw+1=border

	if x >= 1 && x <= lw {
		itemIndex := m.listOffset + (relY / linesPerItem)
		return regionList, itemIndex
	}

	if x > listBoxRight+1 {
		return regionPreview, -1
	}

	return regionNone, -1
}

func (m model) statusBar() string {
	count := len(m.results)
	var parts []string
	parts = append(parts, fmt.Sprintf("%d results", count))
	parts = append(parts, "click/up/dn navigate")
	parts = append(parts, "scroll/C-u/C-d preview")
	parts = append(parts, "Enter copy resume cmd")
	parts = append(parts, "Esc quit")
	return styleStatusBar.Render(strings.Join(parts, " | "))
}

func (m model) doSearch(query string) tea.Cmd {
	db := m.db
	opts := m.searchOpts
	opts.Query = query
	return func() tea.Msg {
		if query == "" {
			return searchResultMsg{query: query}
		}
		results, err := search.Search(db, opts)
		return searchResultMsg{query: query, results: results, err: err}
	}
}

func (m model) doListAll(filter string) tea.Cmd {
	db := m.db
	opts := m.searchOpts
	opts.Query = filter
	return func() tea.Msg {
		if filter == "" {
			results, err := search.ListAll(db, opts)
			return searchResultMsg{query: filter, results: results, err: err}
		}
		// When there's input, do full-text search across all conversation content
		results, err := search.Search(db, opts)
		return searchResultMsg{query: filter, results: results, err: err}
	}
}

func (m model) scheduleDebouncedSearch(query string) tea.Cmd {
	return tea.Tick(debounceDelay, func(time.Time) tea.Msg {
		return debounceTickMsg{query: query}
	})
}

func (m model) loadCurrentPreview() tea.Cmd {
	if len(m.results) == 0 || m.cursor >= len(m.results) {
		return nil
	}
	r := m.results[m.cursor]
	key := previewCacheKey(r.SessionKey, r.ChunkID)
	if key == m.previewKey {
		return nil // already showing this preview
	}
	return loadPreviewCmd(m.db, r, m.query, m.previewWidth())
}

func previewCacheKey(sessionKey string, chunkID int) string {
	return fmt.Sprintf("%s:%d", sessionKey, chunkID)
}
