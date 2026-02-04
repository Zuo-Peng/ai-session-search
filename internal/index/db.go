package index

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

const schema = `
PRAGMA journal_mode = WAL;
PRAGMA synchronous = NORMAL;
PRAGMA cache_size = -64000;
PRAGMA busy_timeout = 5000;

CREATE TABLE IF NOT EXISTS sessions (
    session_key TEXT PRIMARY KEY,
    source      TEXT NOT NULL,
    file_path   TEXT NOT NULL,
    repo_cwd    TEXT NOT NULL DEFAULT '',
    created_at  TEXT NOT NULL DEFAULT '',
    updated_at  TEXT NOT NULL DEFAULT '',
    summary     TEXT NOT NULL DEFAULT '',
    mtime       INTEGER NOT NULL DEFAULT 0,
    size        INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS chunks (
    session_key TEXT NOT NULL,
    chunk_id    INTEGER NOT NULL,
    ts          TEXT NOT NULL DEFAULT '',
    role        TEXT NOT NULL,
    kind        TEXT NOT NULL DEFAULT 'text',
    text        TEXT NOT NULL,
    line_number INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (session_key, chunk_id)
);

CREATE VIRTUAL TABLE IF NOT EXISTS chunks_fts USING fts5(
    text,
    content=chunks,
    content_rowid=rowid,
    tokenize='unicode61'
);

-- triggers to keep FTS in sync
CREATE TRIGGER IF NOT EXISTS chunks_ai AFTER INSERT ON chunks BEGIN
    INSERT INTO chunks_fts(rowid, text) VALUES (new.rowid, new.text);
END;

CREATE TRIGGER IF NOT EXISTS chunks_ad AFTER DELETE ON chunks BEGIN
    INSERT INTO chunks_fts(chunks_fts, rowid, text) VALUES('delete', old.rowid, old.text);
END;

CREATE TRIGGER IF NOT EXISTS chunks_au AFTER UPDATE ON chunks BEGIN
    INSERT INTO chunks_fts(chunks_fts, rowid, text) VALUES('delete', old.rowid, old.text);
    INSERT INTO chunks_fts(rowid, text) VALUES (new.rowid, new.text);
END;
`

type DB struct {
	db *sql.DB
}

func OpenDB(dbPath string) (*DB, error) {
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create db dir: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("init schema: %w", err)
	}

	// migrate: add kind column if missing (for existing databases)
	db.Exec("ALTER TABLE chunks ADD COLUMN kind TEXT NOT NULL DEFAULT 'text'")

	// schema version tracking for forced re-index
	db.Exec("CREATE TABLE IF NOT EXISTS meta (key TEXT PRIMARY KEY, value TEXT)")
	d := &DB{db: db}
	d.migrateSchemaVersion()

	return d, nil
}

// schemaVersion should be bumped whenever chunk parsing logic changes
// to force a full re-index.
const schemaVersion = "3"

func (d *DB) migrateSchemaVersion() {
	var ver string
	err := d.db.QueryRow("SELECT value FROM meta WHERE key = 'schema_version'").Scan(&ver)
	if err != nil || ver != schemaVersion {
		// force re-index by resetting all session mtime/size to 0
		d.db.Exec("UPDATE sessions SET mtime = 0, size = 0")
		d.db.Exec("INSERT OR REPLACE INTO meta (key, value) VALUES ('schema_version', ?)", schemaVersion)
	}
}

func (d *DB) Close() error {
	return d.db.Close()
}

func (d *DB) Raw() *sql.DB {
	return d.db
}

type SessionInfo struct {
	Mtime int64
	Size  int64
}

func (d *DB) GetSessionInfo(sessionKey string) (*SessionInfo, error) {
	var info SessionInfo
	err := d.db.QueryRow(
		"SELECT mtime, size FROM sessions WHERE session_key = ?",
		sessionKey,
	).Scan(&info.Mtime, &info.Size)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &info, nil
}

func (d *DB) AllSessionKeys() (map[string]struct{}, error) {
	rows, err := d.db.Query("SELECT session_key FROM sessions")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	keys := make(map[string]struct{})
	for rows.Next() {
		var k string
		if err := rows.Scan(&k); err != nil {
			return nil, err
		}
		keys[k] = struct{}{}
	}
	return keys, rows.Err()
}

func (d *DB) DeleteSession(sessionKey string) error {
	tx, err := d.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec("DELETE FROM chunks WHERE session_key = ?", sessionKey); err != nil {
		return err
	}
	if _, err := tx.Exec("DELETE FROM sessions WHERE session_key = ?", sessionKey); err != nil {
		return err
	}
	return tx.Commit()
}

func (d *DB) SessionCount() (int, error) {
	var n int
	err := d.db.QueryRow("SELECT COUNT(*) FROM sessions").Scan(&n)
	return n, err
}

func (d *DB) ChunkCount() (int, error) {
	var n int
	err := d.db.QueryRow("SELECT COUNT(*) FROM chunks").Scan(&n)
	return n, err
}

func (d *DB) GetSessionByKey(sessionKey string) (*SessionRow, error) {
	var s SessionRow
	err := d.db.QueryRow(
		"SELECT session_key, source, file_path, repo_cwd, created_at, updated_at, summary FROM sessions WHERE session_key = ?",
		sessionKey,
	).Scan(&s.SessionKey, &s.Source, &s.FilePath, &s.RepoCwd, &s.CreatedAt, &s.UpdatedAt, &s.Summary)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &s, nil
}

type SessionRow struct {
	SessionKey string
	Source     string
	FilePath   string
	RepoCwd    string
	CreatedAt  string
	UpdatedAt  string
	Summary    string
}

type ChunkRow struct {
	SessionKey string
	ChunkID    int
	Ts         string
	Role       string
	Kind       string
	Text       string
	LineNumber int
}

func (d *DB) GetChunks(sessionKey string) ([]ChunkRow, error) {
	rows, err := d.db.Query(
		"SELECT session_key, chunk_id, ts, role, kind, text, line_number FROM chunks WHERE session_key = ? ORDER BY chunk_id",
		sessionKey,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var chunks []ChunkRow
	for rows.Next() {
		var c ChunkRow
		if err := rows.Scan(&c.SessionKey, &c.ChunkID, &c.Ts, &c.Role, &c.Kind, &c.Text, &c.LineNumber); err != nil {
			return nil, err
		}
		chunks = append(chunks, c)
	}
	return chunks, rows.Err()
}

// GetChunksWindow returns a window of chunks around a hit chunk.
// It only loads the necessary rows from the database instead of all chunks.
// startPos is the number of chunks before the returned window.
// totalCount is the total number of chunks in the session.
func (d *DB) GetChunksWindow(sessionKey string, hitChunkID, context int) (chunks []ChunkRow, hitIdx int, startPos int, totalCount int, err error) {
	// get total count
	err = d.db.QueryRow(
		"SELECT COUNT(*) FROM chunks WHERE session_key = ?", sessionKey,
	).Scan(&totalCount)
	if err != nil {
		return nil, -1, 0, 0, err
	}

	// find the row_number (0-based position) of the hit chunk
	hitPos := -1
	if hitChunkID >= 0 {
		err = d.db.QueryRow(`
			SELECT pos FROM (
				SELECT chunk_id, ROW_NUMBER() OVER (ORDER BY chunk_id) - 1 AS pos
				FROM chunks WHERE session_key = ?
			) WHERE chunk_id = ?`,
			sessionKey, hitChunkID,
		).Scan(&hitPos)
		if err == sql.ErrNoRows {
			hitPos = -1
			err = nil
		} else if err != nil {
			return nil, -1, 0, 0, err
		}
	}

	// compute window bounds
	startPos = 0
	limit := totalCount
	if hitPos >= 0 {
		startPos = hitPos - context
		if startPos < 0 {
			startPos = 0
		}
		endPos := hitPos + context + 1
		if endPos > totalCount {
			endPos = totalCount
		}
		limit = endPos - startPos
	}

	rows, err := d.db.Query(
		"SELECT session_key, chunk_id, ts, role, kind, text, line_number FROM chunks WHERE session_key = ? ORDER BY chunk_id LIMIT ? OFFSET ?",
		sessionKey, limit, startPos,
	)
	if err != nil {
		return nil, -1, 0, 0, err
	}
	defer rows.Close()

	var result []ChunkRow
	localHitIdx := -1
	for rows.Next() {
		var c ChunkRow
		if err := rows.Scan(&c.SessionKey, &c.ChunkID, &c.Ts, &c.Role, &c.Kind, &c.Text, &c.LineNumber); err != nil {
			return nil, -1, 0, 0, err
		}
		if c.ChunkID == hitChunkID {
			localHitIdx = len(result)
		}
		result = append(result, c)
	}
	return result, localHitIdx, startPos, totalCount, rows.Err()
}
