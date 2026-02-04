package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var version = "dev"

func main() {
	rootCmd := &cobra.Command{
		Use:     "ais",
		Short:   "AI Session Searcher - search Claude Code and Codex conversation logs",
		Version: version,
	}

	rootCmd.AddCommand(indexCmd())
	rootCmd.AddCommand(searchCmd())
	rootCmd.AddCommand(previewCmd())
	rootCmd.AddCommand(openCmd())
	rootCmd.AddCommand(doctorCmd())

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
