package cmd

import (
	"fmt"
	"os"

	"github.com/christopherdobbyn/jiratui/internal/config"
	"github.com/christopherdobbyn/jiratui/internal/jira"
	"github.com/christopherdobbyn/jiratui/internal/tui"
	"github.com/spf13/cobra"
)

var (
	appVersion = "dev"
	appCommit  = "none"
	appDate    = "unknown"
)

// SetVersionInfo sets the version info from ldflags.
func SetVersionInfo(version, commit, date string) {
	appVersion = version
	appCommit = commit
	appDate = date
}

var rootCmd = &cobra.Command{
	Use:     "jt",
	Short:   "A terminal UI for Jira Cloud",
	Long:    "jt is a fast, lightweight terminal user interface for browsing and interacting with Jira Cloud.",
	Version: "dev",
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
	RunE: func(cmd *cobra.Command, args []string) error {
		cfgPath := config.DefaultPath()
		cfg, err := config.Load(cfgPath)
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}

		profile, err := cfg.ActiveProfileConfig()
		if err != nil {
			return err
		}

		client := jira.NewClient(profile.URL, profile.Email, profile.APIToken)
		var profileNames []string
		for name := range cfg.Profiles {
			profileNames = append(profileNames, name)
		}
		return tui.Run(client, cfg.ActiveProfile, profile.Project, cfgPath, profileNames)
	},
}

func Execute() {
	rootCmd.Version = fmt.Sprintf("%s (commit: %s, built: %s)", appVersion, appCommit, appDate)
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
