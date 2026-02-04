package index

import (
	"fmt"

	"github.com/Zuo-Peng/ai-session-search/internal/parse"
	"github.com/Zuo-Peng/ai-session-search/internal/scan"
)

type Stats struct {
	Scanned  int
	Updated  int
	Skipped  int
	Pruned   int
	Errors   int
}

func (s Stats) String() string {
	return fmt.Sprintf("scanned=%d updated=%d skipped=%d pruned=%d errors=%d",
		s.Scanned, s.Updated, s.Skipped, s.Pruned, s.Errors)
}

func IndexAll(db *DB, claudeRoot, codexRoot string) (Stats, error) {
	var stats Stats

	files, err := scan.ScanRoots(claudeRoot, codexRoot)
	if err != nil {
		return stats, fmt.Errorf("scan: %w", err)
	}
	stats.Scanned = len(files)

	// track which files we see, for pruning
	seenKeys := make(map[string]struct{})

	for _, fi := range files {
		result, err := parseFile(fi, claudeRoot, codexRoot)
		if err != nil {
			stats.Errors++
			fmt.Printf("  WARN: parse %s: %v\n", fi.Path, err)
			continue
		}
		if result == nil || len(result.Chunks) == 0 {
			continue
		}

		seenKeys[result.Meta.SessionKey] = struct{}{}

		needs, err := needsUpdate(db, result.Meta.SessionKey, fi.Mtime, fi.Size)
		if err != nil {
			stats.Errors++
			continue
		}
		if !needs {
			stats.Skipped++
			continue
		}

		if err := indexSession(db, result); err != nil {
			stats.Errors++
			fmt.Printf("  WARN: index %s: %v\n", fi.Path, err)
			continue
		}
		stats.Updated++
	}

	// prune sessions whose files no longer exist
	pruned, err := pruneSessions(db, seenKeys)
	if err != nil {
		return stats, fmt.Errorf("prune: %w", err)
	}
	stats.Pruned = pruned

	return stats, nil
}

func parseFile(fi scan.FileInfo, claudeRoot, codexRoot string) (*parse.ParseResult, error) {
	switch fi.Source {
	case "claude":
		return parse.ParseClaude(fi.Path, claudeRoot)
	case "codex":
		return parse.ParseCodex(fi.Path, codexRoot)
	default:
		return nil, fmt.Errorf("unknown source: %s", fi.Source)
	}
}

func needsUpdate(db *DB, sessionKey string, mtime, size int64) (bool, error) {
	info, err := db.GetSessionInfo(sessionKey)
	if err != nil {
		return false, err
	}
	if info == nil {
		return true, nil // new session
	}
	return info.Mtime != mtime || info.Size != size, nil
}

func indexSession(db *DB, result *parse.ParseResult) error {
	// delete old data first
	if err := db.DeleteSession(result.Meta.SessionKey); err != nil {
		return err
	}

	tx, err := db.Raw().Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// insert session
	_, err = tx.Exec(
		`INSERT INTO sessions (session_key, source, file_path, repo_cwd, created_at, updated_at, summary, mtime, size)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		result.Meta.SessionKey,
		result.Meta.Source,
		result.Meta.FilePath,
		result.Meta.RepoCwd,
		result.Meta.CreatedAt.Format("2006-01-02T15:04:05Z"),
		result.Meta.UpdatedAt.Format("2006-01-02T15:04:05Z"),
		result.Meta.Summary,
		result.Meta.Mtime.Unix(),
		result.Meta.Size,
	)
	if err != nil {
		return err
	}

	// insert chunks
	stmt, err := tx.Prepare(
		`INSERT INTO chunks (session_key, chunk_id, ts, role, kind, text, line_number)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
	)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, c := range result.Chunks {
		kind := c.Kind
		if kind == "" {
			kind = "text"
		}
		_, err := stmt.Exec(
			c.SessionKey,
			c.ChunkID,
			c.Timestamp.Format("2006-01-02T15:04:05Z"),
			c.Role,
			kind,
			c.Text,
			c.LineNumber,
		)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func pruneSessions(db *DB, seenKeys map[string]struct{}) (int, error) {
	allKeys, err := db.AllSessionKeys()
	if err != nil {
		return 0, err
	}

	pruned := 0
	for key := range allKeys {
		if _, ok := seenKeys[key]; !ok {
			if err := db.DeleteSession(key); err != nil {
				return pruned, err
			}
			pruned++
		}
	}
	return pruned, nil
}
