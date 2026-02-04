# ais - AI Session Searcher

A local CLI tool for searching and previewing Claude Code and Codex conversation logs. Built with Go, SQLite FTS5, and [Bubble Tea](https://github.com/charmbracelet/bubbletea).

## Features

- **Full-text search** across Claude Code (`~/.claude/projects/`) and Codex (`~/.codex/sessions/`) JSONL logs
- **Incremental indexing** using SQLite FTS5 (only re-indexes changed files)
- **Interactive TUI** with session list + conversation preview (powered by Bubble Tea)
- **fzf integration** via TSV pipe output for custom workflows
- **Conversation preview** with role-based formatting (user/assistant/tool/system)
- **Filters**: by source (`claude`/`codex`), role, date range

## Install

### From source

```bash
git clone https://github.com/Zuo-Peng/ai-session-search.git
cd ai-session-search
make install   # installs to ~/.local/bin/ais
```

### Build only

```bash
make build     # produces ./ais binary
```

## Usage

### Build the index

```bash
ais index
```

Scans `~/.claude/projects/` and `~/.codex/sessions/` for JSONL conversation files, parses them into chunks, and stores them in a local SQLite database with FTS5.

Subsequent runs are incremental -- only changed files are re-indexed.

### Search

```bash
# Interactive TUI (when stdout is a terminal)
ais search "keyword"

# TSV output for piping (when stdout is not a terminal)
ais search "keyword" | less

# With filters
ais search "keyword" --source claude --role user --since 2026-01-01 --limit 50
```

When running in a terminal, `ais search` launches an interactive TUI with a session list on the left and a conversation preview on the right.

When piped, it outputs TSV:

```
sessionKey  chunkId  updatedAt  source  repo  summary  snippet
```

### Preview a session

```bash
ais preview <sessionKey> --hit <chunkId> --context 5
```

Renders a conversation in a terminal-friendly format with role labels and timestamps. The `--hit` flag highlights the matched chunk and shows surrounding context.

### Open a session

```bash
ais open <sessionKey> --hit <chunkId>
```

Opens the source JSONL file at the matched location.

### Health check

```bash
ais doctor
```

Checks data directories, database status, and index statistics.

## fzf integration

Add this shell function to your `.zshrc` or `.bashrc`:

```bash
aisf() {
  ais search "$*" | fzf \
    --ansi \
    --delimiter='\t' --with-nth=3.. \
    --preview 'ais preview {1} --hit {2} --context 5 --query {q}' \
    --preview-window=right:60%:wrap \
    --preview-debounce=150 \
    --bind 'enter:execute(ais open {1} --hit {2})'
}
```

Then: `aisf some keyword`

## Configuration

Optional config file at `~/.config/ais/config.toml`:

```toml
claude_root = "~/.claude/projects"
codex_root  = "~/.codex/sessions"
db_path     = "~/.config/ais/ais.db"
```

All paths support `~` expansion.

## Project structure

```
cmd/ais/           # CLI entry point (cobra commands)
internal/config/   # TOML config loading
internal/scan/     # File scanning with glob filtering
internal/parse/    # Claude/Codex JSONL parsers + unified model
internal/index/    # SQLite schema + incremental indexer
internal/search/   # FTS5 query + ranking + snippet extraction
internal/render/   # Terminal-friendly conversation rendering
internal/open/     # Open source file at matched location
internal/tui/      # Bubble Tea interactive UI
```

## License

MIT
