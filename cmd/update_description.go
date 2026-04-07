package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var updateDescCmd = &cobra.Command{
	Use:   "update-description <project-key> <issue-key> <file-path>",
	Short: "Update a ticket's description from a markdown file",
	Args:  cobra.ExactArgs(3),
	RunE:  runUpdateDescription,
}

func init() {
	rootCmd.AddCommand(updateDescCmd)
}

func runUpdateDescription(cmd *cobra.Command, args []string) error {
	issueKey := args[1]
	filePath := args[2]

	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("reading file: %w", err)
	}

	client, err := newClientFromConfig()
	if err != nil {
		return err
	}

	fmt.Printf("Updating description of %s...\n", issueKey)
	if err := client.UpdateDescription(issueKey, string(data)); err != nil {
		return fmt.Errorf("update failed: %w", err)
	}
	fmt.Println("Done.")
	return nil
}
