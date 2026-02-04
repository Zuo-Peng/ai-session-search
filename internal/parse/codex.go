package parse

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Top-level record in Codex JSONL
type codexRecord struct {
	Timestamp string          `json:"timestamp"`
	Type      string          `json:"type"`
	Payload   json.RawMessage `json:"payload"`
}

// session_meta payload
type codexSessionMeta struct {
	Cwd string `json:"cwd"`
	Git *struct {
		Branch        string `json:"branch"`
		RepositoryURL string `json:"repository_url"`
	} `json:"git"`
}

// event_msg payload (flat, not nested)
type codexEventPayload struct {
	Type    string `json:"type"`
	Message string `json:"message"` // for user_message
	Text    string `json:"text"`    // for agent_reasoning
}

// response_item payload
type codexResponsePayload struct {
	Type    string `json:"type"`
	Role    string `json:"role"`
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
}

func ParseCodex(filePath, codexRoot string) (*ParseResult, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return nil, err
	}

	rel, err := filepath.Rel(codexRoot, filePath)
	if err != nil {
		rel = filePath
	}
	sessionKey := "codex:" + strings.TrimSuffix(rel, ".jsonl")

	result := &ParseResult{
		Meta: SessionMeta{
			SessionKey: sessionKey,
			Source:     "codex",
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

	for scanner.Scan() {
		lineNum++
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var rec codexRecord
		if err := json.Unmarshal(line, &rec); err != nil {
			continue
		}

		ts := parseTimestamp(rec.Timestamp)

		switch rec.Type {
		case "session_meta":
			var meta codexSessionMeta
			if err := json.Unmarshal(rec.Payload, &meta); err == nil {
				result.Meta.RepoCwd = meta.Cwd
			}

		case "event_msg":
			var evt codexEventPayload
			if err := json.Unmarshal(rec.Payload, &evt); err != nil {
				continue
			}

			var role, kind, text string
			switch evt.Type {
			case "user_message":
				role = "user"
				kind = "text"
				text = strings.TrimSpace(evt.Message)
			case "agent_reasoning":
				role = "assistant"
				kind = "thinking"
				text = strings.TrimSpace(evt.Text)
			default:
				continue
			}

			if text == "" {
				continue
			}
			if firstTS.IsZero() {
				firstTS = ts
			}
			lastTS = ts
			if len(text) > maxTextSize {
				text = text[:maxTextSize]
			}
			result.Chunks = append(result.Chunks, Chunk{
				SessionKey: sessionKey,
				ChunkID:    chunkID,
				Timestamp:  ts,
				Role:       role,
				Kind:       kind,
				Text:       text,
				LineNumber: lineNum,
			})
			chunkID++

		case "response_item":
			var item codexResponsePayload
			if err := json.Unmarshal(rec.Payload, &item); err != nil {
				continue
			}

			// only extract actual message items (user input or assistant output)
			if item.Type != "message" {
				continue
			}

			role := item.Role
			if role == "" {
				role = "assistant"
			}

			var parts []string
			for _, c := range item.Content {
				if (c.Type == "input_text" || c.Type == "output_text" || c.Type == "text") && c.Text != "" {
					parts = append(parts, c.Text)
				}
			}
			text := strings.TrimSpace(strings.Join(parts, "\n"))
			if text == "" {
				continue
			}

			if firstTS.IsZero() {
				firstTS = ts
			}
			lastTS = ts
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

	if len(result.Chunks) > 0 {
		s := result.Chunks[0].Text
		if len(s) > 200 {
			s = s[:200]
		}
		result.Meta.Summary = strings.ReplaceAll(s, "\n", " ")
	}

	return result, scanner.Err()
}
