package cmd

import (
	"fmt"
	"os"

	"github.com/christopherdobbyn/jiratui/internal/config"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "jiratui",
	Short: "A terminal UI for Jira Cloud",
	Long:  "jiratui is a fast, lightweight terminal user interface for browsing and interacting with Jira Cloud.",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Skip first-run check for auth commands (they handle it themselves)
		if cmd.Parent() != nil && cmd.Parent().Use == "auth" {
			return nil
		}
		// Also skip for the auth command group itself
		if cmd.Use == "auth" {
			return nil
		}

		cfgPath := config.DefaultPath()
		if !config.Exists(cfgPath) {
			fmt.Println("Welcome to jiratui!")
			fmt.Println()
			fmt.Println("No configuration found. Let's set up your first Jira Cloud profile.")
			fmt.Println()
			return runAuthAdd(cmd, nil)
		}
		return nil
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
