package parse

import "time"

type SessionMeta struct {
	SessionKey string
	Source     string // "claude" or "codex"
	FilePath   string
	RepoCwd    string
	CreatedAt  time.Time
	UpdatedAt  time.Time
	Summary    string
	Mtime      time.Time
	Size       int64
}

type Chunk struct {
	SessionKey string
	ChunkID    int
	Timestamp  time.Time
	Role       string // "user" or "assistant"
	Kind       string // "text" or "thinking"
	Text       string
	LineNumber int // line number in original file
}

type ParseResult struct {
	Meta   SessionMeta
	Chunks []Chunk
}
