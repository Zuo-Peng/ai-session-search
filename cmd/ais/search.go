package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/Zuo-Peng/ai-session-search/internal/config"
	"github.com/Zuo-Peng/ai-session-search/internal/index"
	"github.com/Zuo-Peng/ai-session-search/internal/search"
	"github.com/Zuo-Peng/ai-session-search/internal/tui"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

const (
	sColorReset   = "\033[0m"
	sColorBoldRed = "\033[1;31m"
	sColorBlue    = "\033[1;34m"
	sColorGreen   = "\033[1;32m"
	sColorDim     = "\033[2m"
)

func colorizeSource(source string) string {
	switch source {
	case "claude":
		return sColorBlue + source + sColorReset
	case "codex":
		return sColorGreen + source + sColorReset
	default:
		return source
	}
}

func colorizeSnippet(snippet string) string {
	snippet = strings.ReplaceAll(snippet, ">>>", sColorBoldRed)
	snippet = strings.ReplaceAll(snippet, "<<<", sColorReset)
	return snippet
}

func searchCmd() *cobra.Command {
	var source, role, since string
	var limit int

	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Full-text search across indexed conversations",
		Long: `Search indexed conversations using FTS5. Output is TSV for fzf integration:
  sessionKey, chunkId, updatedAt, source, repo, summary, snippet

Recommended shell function (add to .zshrc):
  aisf() {
    ~/aisession/ais search "$*" | fzf \
      --ansi \
      --delimiter='\t' --with-nth=3.. \
      --preview '~/aisession/ais preview {1} --hit {2} --context 5 --query {q}' \
      --preview-window=right:60%:wrap \
      --preview-debounce=150 \
      --bind 'enter:execute(~/aisession/ais open {1} --hit {2})'
  }`,
		Args: cobra.ExactArgs(1),
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

			// Auto-update index before searching
			index.IndexAll(db, cfg.ClaudeRoot, cfg.CodexRoot)

			opts := search.Options{
				Source: source,
				Role:   role,
				Since:  since,
				Limit:  limit,
			}

			// Interactive TUI when stdout is a terminal; TSV output for pipes
			if term.IsTerminal(int(os.Stdout.Fd())) {
				return tui.Run(db, args[0], opts)
			}

			opts.Query = args[0]
			results, err := search.Search(db, opts)
			if err != nil {
				return err
			}

			if len(results) == 0 {
				fmt.Fprintln(os.Stderr, "No results found.")
				return nil
			}

			for _, r := range results {
				snippet := strings.ReplaceAll(r.Snippet, "\t", " ")
				snippet = strings.ReplaceAll(snippet, "\n", " ")
				snippet = colorizeSnippet(snippet)
				summary := strings.ReplaceAll(r.Summary, "\t", " ")
				summary = strings.ReplaceAll(summary, "\n", " ")
				repo := r.RepoCwd
				if repo == "" {
					repo = "-"
				}
				// first two fields (sessionKey, chunkID) stay plain for fzf {1} {2}
				fmt.Printf("%s\t%d\t%s%s%s\t%s\t%s\t%s\t%s\n",
					r.SessionKey,
					r.ChunkID,
					sColorDim, r.UpdatedAt, sColorReset,
					colorizeSource(r.Source),
					repo,
					summary,
					snippet,
				)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&source, "source", "", "Filter by source (claude/codex)")
	cmd.Flags().StringVar(&role, "role", "", "Filter by role (user/assistant)")
	cmd.Flags().StringVar(&since, "since", "", "Filter sessions updated since date (YYYY-MM-DD)")
	cmd.Flags().IntVar(&limit, "limit", 100, "Max results")

	return cmd
}
