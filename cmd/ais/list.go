package main

import (
	"github.com/Zuo-Peng/ai-session-search/internal/config"
	"github.com/Zuo-Peng/ai-session-search/internal/index"
	"github.com/Zuo-Peng/ai-session-search/internal/search"
	"github.com/Zuo-Peng/ai-session-search/internal/tui"
	"github.com/spf13/cobra"
)

func listCmd() *cobra.Command {
	var source, since string
	var limit int

	cmd := &cobra.Command{
		Use:   "list",
		Short: "Browse all sessions sorted by update time",
		Long:  `Opens a TUI panel showing all indexed sessions sorted by update time (newest first). Type to filter by summary or repo path.`,
		Args:  cobra.NoArgs,
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

			index.IndexAll(db, cfg.ClaudeRoot, cfg.CodexRoot)

			opts := search.Options{
				Source: source,
				Since:  since,
				Limit:  limit,
			}

			return tui.RunList(db, opts)
		},
	}

	cmd.Flags().StringVar(&source, "source", "", "Filter by source (claude/codex)")
	cmd.Flags().StringVar(&since, "since", "", "Filter sessions updated since date (YYYY-MM-DD)")
	cmd.Flags().IntVar(&limit, "limit", 0, "Max results (0 = no limit)")

	return cmd
}
