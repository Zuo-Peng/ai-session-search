package parse

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const maxLineSize = 10 * 1024 * 1024 // 10MB
const maxTextSize = 8 * 1024          // 8KB for FTS index

type claudeRecord struct {
	Type      string          `json:"type"`
	IsMeta    bool            `json:"isMeta"`
	Timestamp string          `json:"timestamp"`
	Cwd       string          `json:"cwd"`
	Message   json.RawMessage `json:"message"`
	Summary   string          `json:"summary"` // for type="summary" records
}

type claudeMessage struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
}

type claudeContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

func ParseClaude(filePath, claudeRoot string) (*ParseResult, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return nil, err
	}

	// derive session key from path
	rel, err := filepath.Rel(claudeRoot, filePath)
	if err != nil {
		rel = filePath
	}
	sessionKey := "claude:" + strings.TrimSuffix(rel, ".jsonl")

	result := &ParseResult{
		Meta: SessionMeta{
			SessionKey: sessionKey,
			Source:     "claude",
			FilePath:   filePath,
			Mtime:      info.ModTime(),
			Size:       info.Size(),
		},
	}

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), maxLineSize)

	chunkID := 0
	lineNum := 0
	var firstTS, lastTS time.Time
	var summaryFromRecord string

	for scanner.Scan() {
		lineNum++
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var rec claudeRecord
		if err := json.Unmarshal(line, &rec); err != nil {
			continue
		}

		// capture summary record
		if rec.Type == "summary" && rec.Summary != "" {
			summaryFromRecord = rec.Summary
			continue
		}

		// capture cwd from first record that has it
		if rec.Cwd != "" && result.Meta.RepoCwd == "" {
			result.Meta.RepoCwd = rec.Cwd
		}

		if rec.IsMeta {
			continue
		}

		if rec.Type != "user" && rec.Type != "assistant" {
			continue
		}

		var msg claudeMessage
		if err := json.Unmarshal(rec.Message, &msg); err != nil {
			continue
		}

		role := rec.Type
		content := extractClaudeContent(msg.Content)
		if content.Text == "" && content.Thinking == "" {
			continue
		}

		ts := parseTimestamp(rec.Timestamp)
		if firstTS.IsZero() {
			firstTS = ts
		}
		lastTS = ts

		// Emit thinking chunk before text chunk (if both exist)
		if content.Thinking != "" {
			think := content.Thinking
			if len(think) > maxTextSize {
				think = think[:maxTextSize]
			}
			result.Chunks = append(result.Chunks, Chunk{
				SessionKey: sessionKey,
				ChunkID:    chunkID,
				Timestamp:  ts,
				Role:       role,
				Kind:       "thinking",
				Text:       think,
				LineNumber: lineNum,
			})
			chunkID++
		}

		if content.Text != "" {
			text := content.Text
			if len(text) > maxTextSize {
				text = text[:maxTextSize]
			}
			result.Chunks = append(result.Chunks, Chunk{
				SessionKey: sessionKey,
				ChunkID:    chunkID,
				Timestamp:  ts,
				Role:       role,
				Kind:       "text",
				Text:       text,
				LineNumber: lineNum,
			})
			chunkID++
		}
	}

	result.Meta.CreatedAt = firstTS
	result.Meta.UpdatedAt = lastTS

	// prefer summary from record, fallback to first user message
	if summaryFromRecord != "" {
		result.Meta.Summary = summaryFromRecord
	} else if len(result.Chunks) > 0 {
		s := result.Chunks[0].Text
		if len(s) > 200 {
			s = s[:200]
		}
		result.Meta.Summary = strings.ReplaceAll(s, "\n", " ")
	}

	return result, scanner.Err()
}

type extractedContent struct {
	Text     string
	Thinking string
}

func extractClaudeContent(raw json.RawMessage) extractedContent {
	// try string first
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return extractedContent{Text: strings.TrimSpace(s)}
	}

	// try array of content blocks
	var blocks []claudeContentBlock
	if err := json.Unmarshal(raw, &blocks); err == nil {
		var textParts []string
		var thinkParts []string
		for _, b := range blocks {
			if b.Text == "" {
				continue
			}
			switch b.Type {
			case "thinking":
				thinkParts = append(thinkParts, b.Text)
			case "text":
				textParts = append(textParts, b.Text)
			}
		}
		return extractedContent{
			Text:     strings.TrimSpace(strings.Join(textParts, "\n")),
			Thinking: strings.TrimSpace(strings.Join(thinkParts, "\n")),
		}
	}

	return extractedContent{}
}

func parseTimestamp(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	// try RFC3339
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t
	}
	// try RFC3339Nano
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return t
	}
	// try ISO8601 without timezone
	if t, err := time.Parse("2006-01-02T15:04:05", s); err == nil {
		return t
	}
	return time.Time{}
}
