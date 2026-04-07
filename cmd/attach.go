package cmd

import (
	"fmt"

	"github.com/christopherdobbyn/jiratui/internal/config"
	"github.com/christopherdobbyn/jiratui/internal/jira"
	"github.com/spf13/cobra"
)

var attachCmd = &cobra.Command{
	Use:   "attach <project-key> <issue-key> <file-path>",
	Short: "Upload a file as an attachment to a Jira ticket",
	Args:  cobra.ExactArgs(3),
	RunE:  runAttach,
}

func init() {
	rootCmd.AddCommand(attachCmd)
}

func runAttach(cmd *cobra.Command, args []string) error {
	issueKey := args[1]
	filePath := args[2]

	client, err := newClientFromConfig()
	if err != nil {
		return err
	}

	fmt.Printf("Uploading %s to %s...\n", filePath, issueKey)
	if err := client.UploadAttachment(issueKey, filePath); err != nil {
		return fmt.Errorf("upload failed: %w", err)
	}
	fmt.Println("Done.")
	return nil
}

// newClientFromConfig loads config and creates a Jira client.
func newClientFromConfig() (*jira.Client, error) {
	cfgPath := config.DefaultPath()
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return nil, fmt.Errorf("loading config: %w", err)
	}
	profile, err := cfg.ActiveProfileConfig()
	if err != nil {
		return nil, err
	}
	return jira.NewClient(profile.URL, profile.Email, profile.APIToken), nil
}
