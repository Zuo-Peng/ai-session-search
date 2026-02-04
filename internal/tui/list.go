package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
	"github.com/Zuo-Peng/ai-session-search/internal/search"
)

// linesPerItem is the number of terminal lines each result occupies.
const linesPerItem = 2

// renderList renders the left panel: search results list with scrolling.
func (m model) renderList(width, height int) string {
	if len(m.results) == 0 {
		empty := lipgloss.NewStyle().
			Foreground(colorDim).
			Width(width).
			Height(height).
			Align(lipgloss.Center, lipgloss.Center).
			Render("No results")
		return empty
	}

	var lines []string
	for i, r := range m.results {
		if i < m.listOffset {
			continue
		}
		if len(lines)+linesPerItem > height {
			break
		}
		rows := formatResultLine(r, width, i == m.cursor)
		lines = append(lines, rows...)
	}

	// Pad remaining lines
	for len(lines) < height {
		lines = append(lines, strings.Repeat(" ", width))
	}

	return strings.Join(lines, "\n")
}

// formatResultLine formats a single search result as two lines:
//
//	line 1: [>] source  date  summary
//	line 2:    snippet (dimmed)
func formatResultLine(r search.Result, width int, selected bool) []string {
	// Format source with color
	var src string
	switch r.Source {
	case "claude":
		src = styleSourceClaude.Render("claude")
	case "codex":
		src = styleSourceCodex.Render("codex")
	default:
		src = r.Source
	}

	// Extract short date from UpdatedAt (e.g. "2026-01-27" -> "01-27")
	date := r.UpdatedAt
	if len(date) >= 10 {
		date = date[5:10] // MM-DD
	}

	// Truncate summary to fit width: leave room for prefix "  src MM-DD "
	summary := strings.ReplaceAll(r.Summary, "\n", " ")
	summaryMax := width - 2 - 7 - 6 - 2 // prefix + source + date + padding
	if summaryMax < 0 {
		summaryMax = 0
	}
	if runewidth.StringWidth(summary) > summaryMax {
		summary = runewidth.Truncate(summary, summaryMax, "")
	}

	// Line 1: source date summary
	line1 := fmt.Sprintf("%s %s %s", src, date, summary)
	if selected {
		line1 = styleListSelected.Render("> ") + line1
	} else {
		line1 = "  " + line1
	}

	// Line 2: snippet (dimmed, indented)
	snippet := strings.ReplaceAll(r.Snippet, "\n", " ")
	snippet = strings.ReplaceAll(snippet, "\t", " ")
	snippet = strings.ReplaceAll(snippet, ">>>", "")
	snippet = strings.ReplaceAll(snippet, "<<<", "")
	snippetMax := width - 4 // indent
	if snippetMax < 0 {
		snippetMax = 0
	}
	if runewidth.StringWidth(snippet) > snippetMax {
		snippet = runewidth.Truncate(snippet, snippetMax, "")
	}
	line2 := "    " + lipgloss.NewStyle().Foreground(colorDim).Render(snippet)

	return []string{line1, line2}
}

// adjustListScroll keeps the cursor visible within the list viewport.
func (m *model) adjustListScroll(listHeight int) {
	visibleItems := listHeight / linesPerItem
	if visibleItems < 1 {
		visibleItems = 1
	}
	if m.cursor < m.listOffset {
		m.listOffset = m.cursor
	}
	if m.cursor >= m.listOffset+visibleItems {
		m.listOffset = m.cursor - visibleItems + 1
	}
}
