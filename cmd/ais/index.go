package main

import (
	"fmt"
	"os"

	"github.com/Zuo-Peng/ai-session-search/internal/config"
	"github.com/Zuo-Peng/ai-session-search/internal/index"
	"github.com/spf13/cobra"
)

func indexCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "index",
		Short: "Scan and index Claude Code and Codex conversation logs",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			db, err := index.OpenDB(cfg.DBPath)
			if err != nil {
				return fmt.Errorf("open db: %w", err)
			}
			defer db.Close()

			fmt.Fprintf(os.Stderr, "Scanning roots...\n")
			fmt.Fprintf(os.Stderr, "  Claude: %s\n", cfg.ClaudeRoot)
			fmt.Fprintf(os.Stderr, "  Codex:  %s\n", cfg.CodexRoot)

			stats, err := index.IndexAll(db, cfg.ClaudeRoot, cfg.CodexRoot)
			if err != nil {
				return fmt.Errorf("index: %w", err)
			}

			fmt.Fprintf(os.Stderr, "Done. %s\n", stats)
			return nil
		},
	}
}
