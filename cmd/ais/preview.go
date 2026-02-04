package main

import (
	"fmt"

	"github.com/Zuo-Peng/ai-session-search/internal/config"
	"github.com/Zuo-Peng/ai-session-search/internal/index"
	"github.com/Zuo-Peng/ai-session-search/internal/render"
	"github.com/spf13/cobra"
)

func previewCmd() *cobra.Command {
	var hitChunkID int
	var context int
	var query string

	cmd := &cobra.Command{
		Use:   "preview <sessionKey>",
		Short: "Preview a conversation with context around a hit",
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

			out, _, err := render.RenderConversation(db, args[0], render.Options{
				HitChunkID: hitChunkID,
				Context:    context,
				Query:      query,
			})
			if err != nil {
				return err
			}

			fmt.Print(out)
			return nil
		},
	}

	cmd.Flags().IntVar(&hitChunkID, "hit", -1, "Chunk ID to highlight")
	cmd.Flags().IntVar(&context, "context", 10, "Messages before/after hit to show")
	cmd.Flags().StringVar(&query, "query", "", "Search query for keyword highlighting")

	return cmd
}
