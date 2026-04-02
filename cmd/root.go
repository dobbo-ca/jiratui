package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "jiratui",
	Short: "A terminal UI for Jira Cloud",
	Long:  "jiratui is a fast, lightweight terminal user interface for browsing and interacting with Jira Cloud.",
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
