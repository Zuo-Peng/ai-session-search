package render

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/mattn/go-runewidth"
	"github.com/Zuo-Peng/ai-session-search/internal/index"
)

const (
	colorReset    = "\033[0m"
	colorUser     = "\033[1;34m" // bold blue
	colorAssist   = "\033[1;32m" // bold green
	colorThink    = "\033[2;35m" // dim magenta for thinking
	colorDim      = "\033[2m"
	colorHit      = "\033[43m"   // yellow background
	colorBoldRed  = "\033[1;31m" // bold red for keyword highlights
)

type Options struct {
	HitChunkID int
	Context    int    // messages before/after hit to show
	Width      int    // wrap width (0 = no wrap)
	ShowSystem bool   // not used yet, reserved
	Query      string // search query for keyword highlighting
}

// fts5Operators are FTS5 operators that should not be highlighted as keywords.
var fts5Operators = map[string]bool{
	"AND": true, "OR": true, "NOT": true, "NEAR": true,
	"and": true, "or": true, "not": true, "near": true,
}

// highlightKeywords wraps case-insensitive matches of query terms in bold red ANSI codes.
func highlightKeywords(text, query string) string {
	if query == "" {
		return text
	}
	terms := strings.Fields(query)
	var filtered []string
	for _, t := range terms {
		if !fts5Operators[t] {
			filtered = append(filtered, t)
		}
	}
	if len(filtered) == 0 {
		return text
	}
	for _, term := range filtered {
		lower := strings.ToLower(term)
		i := 0
		for i < len(text) {
			idx := strings.Index(strings.ToLower(text[i:]), lower)
			if idx < 0 {
				break
			}
			pos := i + idx
			orig := text[pos : pos+len(term)]
			replacement := colorBoldRed + orig + colorReset
			text = text[:pos] + replacement + text[pos+len(term):]
			i = pos + len(replacement)
		}
	}
	return text
}

// indentLines prepends each line of text with the given prefix.
func indentLines(text, prefix string) string {
	lines := strings.Split(text, "\n")
	for i, l := range lines {
		lines[i] = prefix + l
	}
	return strings.Join(lines, "\n")
}

// wrapLine breaks a single line into multiple lines that fit within maxWidth
// visible columns, correctly skipping ANSI escape sequences when measuring width.
func wrapLine(line string, maxWidth int) []string {
	if maxWidth <= 0 {
		return []string{line}
	}

	var result []string
	var cur strings.Builder
	visW := 0

	i := 0
	for i < len(line) {
		// check for ANSI escape sequence: ESC[ ... m
		if i+1 < len(line) && line[i] == '\033' && line[i+1] == '[' {
			j := i + 2
			for j < len(line) && line[j] != 'm' {
				j++
			}
			if j < len(line) {
				j++ // include 'm'
			}
			cur.WriteString(line[i:j])
			i = j
			continue
		}

		r, size := utf8.DecodeRuneInString(line[i:])
		rw := runewidth.RuneWidth(r)

		if visW+rw > maxWidth {
			result = append(result, cur.String())
			cur.Reset()
			visW = 0
		}

		cur.WriteRune(r)
		visW += rw
		i += size
	}

	if cur.Len() > 0 {
		result = append(result, cur.String())
	}

	if len(result) == 0 {
		return []string{""}
	}
	return result
}

// RenderConversation renders a conversation and returns the content,
// the 0-based line number of the hit chunk header (-1 if no hit), and any error.
func RenderConversation(db *index.DB, sessionKey string, opts Options) (string, int, error) {
	if opts.Context == 0 {
		opts.Context = 10
	}
	if opts.Context < 0 {
		opts.Context = 1000000 // no limit
	}

	session, err := db.GetSessionByKey(sessionKey)
	if err != nil {
		return "", -1, fmt.Errorf("get session: %w", err)
	}
	if session == nil {
		return "", -1, fmt.Errorf("session not found: %s", sessionKey)
	}

	chunks, hitIdx, startPos, totalCount, err := db.GetChunksWindow(sessionKey, opts.HitChunkID, opts.Context)
	if err != nil {
		return "", -1, fmt.Errorf("get chunks: %w", err)
	}

	if totalCount == 0 {
		return "(empty session)", -1, nil
	}

	skipAfter := totalCount - startPos - len(chunks)

	var b strings.Builder
	hitLine := -1
	lineCount := 0
	separator := colorDim + "--------------------------------------------------" + colorReset
	wrapW := opts.Width

	// helper to track line count; wraps long lines if Width is set
	writeLine := func(s string) {
		wrapped := wrapLine(s, wrapW)
		for _, wl := range wrapped {
			b.WriteString(wl)
			b.WriteString("\n")
			lineCount++
		}
	}

	// header
	writeLine(fmt.Sprintf("%s--- %s [%s] %s ---%s", colorDim, sessionKey, session.Source, session.RepoCwd, colorReset))

	if startPos > 0 {
		writeLine(fmt.Sprintf("%s... (%d messages before) ...%s", colorDim, startPos, colorReset))
	}

	for i, c := range chunks {
		isHit := (i == hitIdx)

		// separator between messages
		if i > 0 {
			writeLine(separator)
		}

		if isHit {
			hitLine = lineCount
		}

		var roleColor string
		var roleLabel string
		isThinking := c.Kind == "thinking"
		switch c.Role {
		case "user":
			roleColor = colorUser
			roleLabel = "USER"
		case "assistant":
			if isThinking {
				roleColor = colorThink
				roleLabel = "THINK"
			} else {
				roleColor = colorAssist
				roleLabel = "ASST"
			}
		default:
			roleColor = colorDim
			roleLabel = strings.ToUpper(c.Role)
		}

		if isHit {
			writeLine(fmt.Sprintf("%s>> %s > %s <<%s", colorHit, roleLabel, c.Ts, colorReset))
		} else {
			writeLine(fmt.Sprintf("%s%s >%s %s%s%s", roleColor, roleLabel, colorReset, colorDim, c.Ts, colorReset))
		}

		text := c.Text
		if isThinking {
			text = colorDim + text + colorReset
		}
		text = highlightKeywords(text, opts.Query)
		text = indentLines(text, "  ")

		textLines := strings.Split(text, "\n")
		for _, tl := range textLines {
			writeLine(tl)
		}
		writeLine("") // blank line after message
	}

	if skipAfter > 0 {
		writeLine(fmt.Sprintf("%s... (%d messages after) ...%s", colorDim, skipAfter, colorReset))
	}

	return b.String(), hitLine, nil
}
