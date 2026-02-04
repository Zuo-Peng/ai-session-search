package tui

import (
	"github.com/charmbracelet/bubbles/viewport"
	"github.com/Zuo-Peng/ai-session-search/internal/index"
	"github.com/Zuo-Peng/ai-session-search/internal/render"
	"github.com/Zuo-Peng/ai-session-search/internal/search"
	tea "github.com/charmbracelet/bubbletea"
)

// previewRenderedMsg is sent when an async preview render completes.
type previewRenderedMsg struct {
	sessionKey string
	chunkID    int
	content    string
	hitLine    int
	err        error
}

// loadPreviewCmd returns a tea.Cmd that renders the conversation preview async.
func loadPreviewCmd(db *index.DB, r search.Result, query string, width int) tea.Cmd {
	return func() tea.Msg {
		content, hitLine, err := render.RenderConversation(db, r.SessionKey, render.Options{
			HitChunkID: r.ChunkID,
			Context:    -1,
			Width:      width,
			Query:      query,
		})
		return previewRenderedMsg{
			sessionKey: r.SessionKey,
			chunkID:    r.ChunkID,
			content:    content,
			hitLine:    hitLine,
			err:        err,
		}
	}
}

// newViewport creates a new viewport model with the given dimensions.
func newViewport(width, height int) viewport.Model {
	vp := viewport.New(width, height)
	vp.Style = stylePanelBorder
	return vp
}
