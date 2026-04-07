package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var downloadAttachCmd = &cobra.Command{
	Use:   "download-attachment <project-key> <issue-key> <filename> [dest-dir]",
	Short: "Download a ticket attachment to a local path",
	Args:  cobra.RangeArgs(3, 4),
	RunE:  runDownloadAttachment,
}

func init() {
	rootCmd.AddCommand(downloadAttachCmd)
}

func runDownloadAttachment(cmd *cobra.Command, args []string) error {
	issueKey := args[1]
	filename := args[2]
	destDir := "."
	if len(args) > 3 {
		destDir = args[3]
	}

	client, err := newClientFromConfig()
	if err != nil {
		return err
	}

	issue, err := client.GetIssue(issueKey)
	if err != nil {
		return fmt.Errorf("fetching issue: %w", err)
	}

	var attachURL string
	for _, a := range issue.Attachments {
		if a.Filename == filename {
			attachURL = a.URL
			break
		}
	}
	if attachURL == "" {
		return fmt.Errorf("attachment %q not found on %s", filename, issueKey)
	}

	fmt.Printf("Downloading %s from %s...\n", filename, issueKey)
	data, err := client.DownloadURL(attachURL)
	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}

	destPath := filepath.Join(destDir, filename)
	if err := os.WriteFile(destPath, data, 0o644); err != nil {
		return fmt.Errorf("writing file: %w", err)
	}
	fmt.Printf("Saved to %s\n", destPath)
	return nil
}
