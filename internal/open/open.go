package open

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/Zuo-Peng/ai-session-search/internal/index"
)

func OpenSession(db *index.DB, sessionKey string, hitChunkID int) error {
	session, err := db.GetSessionByKey(sessionKey)
	if err != nil {
		return fmt.Errorf("get session: %w", err)
	}
	if session == nil {
		return fmt.Errorf("session not found: %s", sessionKey)
	}

	filePath := session.FilePath
	if _, err := os.Stat(filePath); err != nil {
		return fmt.Errorf("file not found: %s", filePath)
	}

	// find line number for the hit chunk
	lineNum := 1
	if hitChunkID >= 0 {
		chunks, err := db.GetChunks(sessionKey)
		if err == nil {
			for _, c := range chunks {
				if c.ChunkID == hitChunkID {
					lineNum = c.LineNumber
					break
				}
			}
		}
	}

	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "less"
	}

	return openInEditor(editor, filePath, lineNum)
}

func openInEditor(editor, filePath string, lineNum int) error {
	var cmd *exec.Cmd

	switch {
	case strings.Contains(editor, "vim") || strings.Contains(editor, "nvim"):
		cmd = exec.Command(editor, fmt.Sprintf("+%d", lineNum), filePath)
	case strings.Contains(editor, "code"):
		cmd = exec.Command(editor, "--goto", filePath+":"+strconv.Itoa(lineNum))
	case strings.Contains(editor, "less"):
		cmd = exec.Command(editor, "+"+strconv.Itoa(lineNum), filePath)
	default:
		cmd = exec.Command(editor, filePath)
	}

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
