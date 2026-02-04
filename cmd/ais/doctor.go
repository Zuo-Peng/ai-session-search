package main

import (
	"fmt"
	"os"

	"github.com/Zuo-Peng/ai-session-search/internal/config"
	"github.com/Zuo-Peng/ai-session-search/internal/index"
	"github.com/Zuo-Peng/ai-session-search/internal/scan"
	"github.com/spf13/cobra"
)

func doctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Self-check: verify roots, DB, FTS5, and show stats",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("config: %w", err)
			}

			// check roots
			fmt.Println("=== Roots ===")
			checkDir("Claude", cfg.ClaudeRoot)
			checkDir("Codex", cfg.CodexRoot)

			// scan file counts
			fmt.Println("\n=== File Scan ===")
			files, err := scan.ScanRoots(cfg.ClaudeRoot, cfg.CodexRoot)
			if err != nil {
				fmt.Printf("  scan error: %v\n", err)
			} else {
				claudeCount, codexCount := 0, 0
				for _, f := range files {
					if f.Source == "claude" {
						claudeCount++
					} else {
						codexCount++
					}
				}
				fmt.Printf("  Claude JSONL files: %d\n", claudeCount)
				fmt.Printf("  Codex  JSONL files: %d\n", codexCount)
			}

			// check DB
			fmt.Println("\n=== Database ===")
			fmt.Printf("  Path: %s\n", cfg.DBPath)
			if _, err := os.Stat(cfg.DBPath); os.IsNotExist(err) {
				fmt.Println("  Status: NOT FOUND (run 'ais index' first)")
				return nil
			}

			db, err := index.OpenDB(cfg.DBPath)
			if err != nil {
				return fmt.Errorf("open db: %w", err)
			}
			defer db.Close()

			sessionCount, err := db.SessionCount()
			if err != nil {
				return fmt.Errorf("count sessions: %w", err)
			}

			chunkCount, err := db.ChunkCount()
			if err != nil {
				return fmt.Errorf("count chunks: %w", err)
			}

			fmt.Printf("  Sessions: %d\n", sessionCount)
			fmt.Printf("  Chunks:   %d\n", chunkCount)

			// check FTS5
			fmt.Println("\n=== FTS5 ===")
			var ftsCount int
			err = db.Raw().QueryRow("SELECT COUNT(*) FROM chunks_fts").Scan(&ftsCount)
			if err != nil {
				fmt.Printf("  FTS5 error: %v\n", err)
			} else {
				fmt.Printf("  FTS5 entries: %d\n", ftsCount)
				if ftsCount == chunkCount {
					fmt.Println("  Status: OK (synced)")
				} else {
					fmt.Printf("  Status: MISMATCH (chunks=%d, fts=%d)\n", chunkCount, ftsCount)
				}
			}

			// check DB file size
			if info, err := os.Stat(cfg.DBPath); err == nil {
				sizeMB := float64(info.Size()) / 1024 / 1024
				fmt.Printf("\n=== DB Size: %.1f MB ===\n", sizeMB)
			}

			return nil
		},
	}
}

func checkDir(name, path string) {
	if info, err := os.Stat(path); err != nil {
		fmt.Printf("  %s: %s (NOT FOUND)\n", name, path)
	} else if !info.IsDir() {
		fmt.Printf("  %s: %s (NOT A DIRECTORY)\n", name, path)
	} else {
		fmt.Printf("  %s: %s (OK)\n", name, path)
	}
}
