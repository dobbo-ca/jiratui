package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/christopherdobbyn/jiratui/internal/config"
	"github.com/christopherdobbyn/jiratui/internal/jira"
	"github.com/spf13/cobra"
)

var issuesCmd = &cobra.Command{
	Use:   "issues",
	Short: "List issues assigned to you",
	RunE:  runIssues,
}

func init() {
	rootCmd.AddCommand(issuesCmd)
}

func runIssues(cmd *cobra.Command, args []string) error {
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

	fmt.Printf("Fetching issues from %s...\n\n", profile.URL)

	result, err := client.SearchMyIssues(50, "", "")
	if err != nil {
		return err
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "KEY\tPRIORITY\tSTATUS\tASSIGNEE\tSUMMARY\n")
	fmt.Fprintf(w, "---\t--------\t------\t--------\t-------\n")

	for _, issue := range result.Issues {
		assignee := "-"
		if issue.Assignee != nil {
			assignee = issue.Assignee.DisplayName
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			issue.Key,
			issue.Priority.Name,
			issue.Status.Name,
			assignee,
			issue.Summary,
		)
	}

	w.Flush()

	more := ""
	if !result.IsLast {
		more = " (more available)"
	}
	fmt.Printf("\nShowing %d issues%s\n", len(result.Issues), more)

	return nil
}
