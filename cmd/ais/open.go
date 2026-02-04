package main

import (
	"github.com/Zuo-Peng/ai-session-search/internal/config"
	"github.com/Zuo-Peng/ai-session-search/internal/index"
	"github.com/Zuo-Peng/ai-session-search/internal/open"
	"github.com/spf13/cobra"
)

func openCmd() *cobra.Command {
	var hitChunkID int

	cmd := &cobra.Command{
		Use:   "open <sessionKey>",
		Short: "Open the original JSONL file in $EDITOR at the hit line",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}

			db, err := index.OpenDB(cfg.DBPath)
			if err != nil {
				return err
			}
			defer db.Close()

			return open.OpenSession(db, args[0], hitChunkID)
		},
	}

	cmd.Flags().IntVar(&hitChunkID, "hit", -1, "Chunk ID to jump to")

	return cmd
}
