package search

import (
	"database/sql"
	"fmt"
	"strings"
	"unicode"

	"github.com/Zuo-Peng/ai-session-search/internal/index"
)

type Result struct {
	SessionKey string
	ChunkID    int
	UpdatedAt  string
	Source     string
	RepoCwd    string
	Summary    string
	Snippet    string
	Role       string
	Rank       float64
}

type Options struct {
	Query  string
	Source string // "" = all, "claude", "codex"
	Role   string // "" = all, "user", "assistant"
	Since  string // "" = no filter, e.g. "2024-01-01"
	Limit  int
}

// containsCJK returns true if the string contains any CJK Unified Ideograph.
func containsCJK(s string) bool {
	for _, r := range s {
		if unicode.Is(unicode.Han, r) {
			return true
		}
	}
	return false
}

// makeSnippet extracts a snippet around the first occurrence of query in text.
func makeSnippet(text, query string, contextChars int) string {
	lower := strings.ToLower(text)
	qLower := strings.ToLower(query)
	idx := strings.Index(lower, qLower)
	if idx < 0 {
		// no match, return head
		if len([]rune(text)) > contextChars*2 {
			return string([]rune(text)[:contextChars*2]) + "..."
		}
		return text
	}
	runes := []rune(text)
	qRunes := []rune(query)
	// find rune position of idx
	runePos := len([]rune(text[:idx]))
	start := runePos - contextChars
	if start < 0 {
		start = 0
	}
	end := runePos + len(qRunes) + contextChars
	if end > len(runes) {
		end = len(runes)
	}
	prefix := ""
	suffix := ""
	if start > 0 {
		prefix = "..."
	}
	if end < len(runes) {
		suffix = "..."
	}
	// wrap the matched part with markers
	snippet := string(runes[start:runePos]) +
		">>>" + string(runes[runePos:runePos+len(qRunes)]) + "<<<" +
		string(runes[runePos+len(qRunes):end])
	return prefix + snippet + suffix
}

func Search(db *index.DB, opts Options) ([]Result, error) {
	if opts.Limit <= 0 {
		opts.Limit = 100
	}

	// Fetch more results before dedup so we still have enough after
	origLimit := opts.Limit
	opts.Limit = origLimit * 3

	var results []Result
	var err error
	if containsCJK(opts.Query) {
		results, err = searchLike(db, opts)
	} else {
		results, err = searchFTS(db, opts)
	}
	if err != nil {
		return nil, err
	}

	// Deduplicate: keep only the best-ranked result per session
	seen := make(map[string]bool)
	var deduped []Result
	for _, r := range results {
		if seen[r.SessionKey] {
			continue
		}
		seen[r.SessionKey] = true
		deduped = append(deduped, r)
		if len(deduped) >= origLimit {
			break
		}
	}
	return deduped, nil
}

func searchFTS(db *index.DB, opts Options) ([]Result, error) {
	var conditions []string
	var args []interface{}

	// FTS match
	conditions = append(conditions, "chunks_fts MATCH ?")
	args = append(args, opts.Query)

	// source filter
	if opts.Source != "" {
		conditions = append(conditions, "s.source = ?")
		args = append(args, opts.Source)
	}

	// role filter
	if opts.Role != "" {
		conditions = append(conditions, "c.role = ?")
		args = append(args, opts.Role)
	}

	// since filter
	if opts.Since != "" {
		conditions = append(conditions, "s.updated_at >= ?")
		args = append(args, opts.Since)
	}

	where := strings.Join(conditions, " AND ")

	query := fmt.Sprintf(`
		SELECT
			c.session_key,
			c.chunk_id,
			s.updated_at,
			s.source,
			s.repo_cwd,
			s.summary,
			snippet(chunks_fts, 0, '>>>','<<<', '...', 40) as snip,
			c.role,
			bm25(chunks_fts, 1.0) as rank
		FROM chunks_fts
		JOIN chunks c ON chunks_fts.rowid = c.rowid
		JOIN sessions s ON c.session_key = s.session_key
		WHERE %s
		ORDER BY rank
		LIMIT ?
	`, where)

	args = append(args, opts.Limit)

	rows, err := db.Raw().Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("search query: %w", err)
	}
	defer rows.Close()

	return scanResults(rows)
}

func searchLike(db *index.DB, opts Options) ([]Result, error) {
	var conditions []string
	var args []interface{}

	// LIKE match for CJK substring search
	conditions = append(conditions, "c.text LIKE ?")
	args = append(args, "%"+opts.Query+"%")

	// source filter
	if opts.Source != "" {
		conditions = append(conditions, "s.source = ?")
		args = append(args, opts.Source)
	}

	// role filter
	if opts.Role != "" {
		conditions = append(conditions, "c.role = ?")
		args = append(args, opts.Role)
	}

	// since filter
	if opts.Since != "" {
		conditions = append(conditions, "s.updated_at >= ?")
		args = append(args, opts.Since)
	}

	where := strings.Join(conditions, " AND ")

	query := fmt.Sprintf(`
		SELECT
			c.session_key,
			c.chunk_id,
			s.updated_at,
			s.source,
			s.repo_cwd,
			s.summary,
			c.text,
			c.role
		FROM chunks c
		JOIN sessions s ON c.session_key = s.session_key
		WHERE %s
		ORDER BY s.updated_at DESC
		LIMIT ?
	`, where)

	args = append(args, opts.Limit)

	rows, err := db.Raw().Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("search query: %w", err)
	}
	defer rows.Close()

	var results []Result
	for rows.Next() {
		var r Result
		var fullText string
		if err := rows.Scan(
			&r.SessionKey, &r.ChunkID, &r.UpdatedAt,
			&r.Source, &r.RepoCwd, &r.Summary,
			&fullText, &r.Role,
		); err != nil {
			return nil, err
		}
		r.Snippet = makeSnippet(fullText, opts.Query, 30)
		r.Rank = 0
		results = append(results, r)
	}
	return results, rows.Err()
}

func scanResults(rows *sql.Rows) ([]Result, error) {
	var results []Result
	for rows.Next() {
		var r Result
		if err := rows.Scan(
			&r.SessionKey, &r.ChunkID, &r.UpdatedAt,
			&r.Source, &r.RepoCwd, &r.Summary,
			&r.Snippet, &r.Role, &r.Rank,
		); err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	return results, rows.Err()
}
