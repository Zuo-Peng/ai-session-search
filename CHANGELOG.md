# Changelog

## [0.2.0] - 2026-02-06

### Added

- `ais list` command: browse all sessions sorted by update time (newest first)
  - Interactive TUI with real-time filtering
  - Empty input shows all sessions; typing triggers full-text search across conversation content
  - Supports `--source`, `--since`, `--limit` flags
  - List items show repo path as second line instead of repeating summary
- `search.ListAll()` function for querying sessions without FTS

## [0.1.0] - 2026-01-27

### Added

- Initial release
- Full-text search across Claude Code and Codex JSONL logs
- Incremental indexing with SQLite FTS5
- Interactive TUI with session list + conversation preview
- Enter-to-copy resume command (`claude --resume` / `codex resume`)
- Pipe-friendly TSV output
- Filters by source, role, date range
- `ais index`, `ais search`, `ais preview`, `ais open`, `ais doctor` commands
