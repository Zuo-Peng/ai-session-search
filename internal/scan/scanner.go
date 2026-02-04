package scan

import (
	"os"
	"path/filepath"
	"strings"
)

type FileInfo struct {
	Path   string
	Source string // "claude" or "codex"
	Mtime  int64
	Size   int64
}

func ScanRoots(claudeRoot, codexRoot string) ([]FileInfo, error) {
	var files []FileInfo

	if claudeRoot != "" {
		cf, err := scanClaude(claudeRoot)
		if err != nil && !os.IsNotExist(err) {
			return nil, err
		}
		files = append(files, cf...)
	}

	if codexRoot != "" {
		cf, err := scanCodex(codexRoot)
		if err != nil && !os.IsNotExist(err) {
			return nil, err
		}
		files = append(files, cf...)
	}

	return files, nil
}

func scanClaude(root string) ([]FileInfo, error) {
	var files []FileInfo
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip unreadable dirs
		}
		if info.IsDir() {
			base := filepath.Base(path)
			if base == "subagents" {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".jsonl" {
			return nil
		}
		if strings.Contains(filepath.Base(path), "sessions-index") {
			return nil
		}
		files = append(files, FileInfo{
			Path:   path,
			Source: "claude",
			Mtime:  info.ModTime().Unix(),
			Size:   info.Size(),
		})
		return nil
	})
	return files, err
}

func scanCodex(root string) ([]FileInfo, error) {
	var files []FileInfo
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			return nil
		}
		if filepath.Ext(path) != ".jsonl" {
			return nil
		}
		files = append(files, FileInfo{
			Path:   path,
			Source: "codex",
			Mtime:  info.ModTime().Unix(),
			Size:   info.Size(),
		})
		return nil
	})
	return files, err
}
