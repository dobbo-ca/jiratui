package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/christopherdobbyn/jiratui/internal/config"
	"github.com/spf13/cobra"
)

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Manage Jira authentication profiles",
}

var authAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a new authentication profile",
	RunE:  runAuthAdd,
}

var authListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all authentication profiles",
	RunE:  runAuthList,
}

var authSwitchCmd = &cobra.Command{
	Use:   "switch [profile-name]",
	Short: "Switch the active profile",
	Args:  cobra.ExactArgs(1),
	RunE:  runAuthSwitch,
}

func init() {
	rootCmd.AddCommand(authCmd)
	authCmd.AddCommand(authAddCmd)
	authCmd.AddCommand(authListCmd)
	authCmd.AddCommand(authSwitchCmd)
}

func prompt(reader *bufio.Reader, label string) string {
	fmt.Printf("%s: ", label)
	text, _ := reader.ReadString('\n')
	return strings.TrimSpace(text)
}

func runAuthAdd(cmd *cobra.Command, args []string) error {
	cfgPath := config.DefaultPath()

	var cfg *config.Config
	if config.Exists(cfgPath) {
		var err error
		cfg, err = config.Load(cfgPath)
		if err != nil {
			return fmt.Errorf("loading existing config: %w", err)
		}
	} else {
		cfg = &config.Config{
			Profiles: make(map[string]config.Profile),
		}
	}

	reader := bufio.NewReader(os.Stdin)

	fmt.Println("Add a new Jira Cloud profile")
	fmt.Println("---")
	fmt.Println("You'll need an API token from: https://id.atlassian.com/manage-profile/security/api-tokens")
	fmt.Println()

	name := prompt(reader, "Profile name (e.g. work, personal)")
	url := prompt(reader, "Jira URL (e.g. https://company.atlassian.net)")
	email := prompt(reader, "Email")
	token := prompt(reader, "API token")

	// Normalize URL: strip trailing slash
	url = strings.TrimRight(url, "/")

	profile := config.Profile{
		URL:      url,
		Email:    email,
		APIToken: token,
	}

	if err := cfg.AddProfile(name, profile); err != nil {
		return err
	}

	// If this is the first profile, set it as active
	if len(cfg.Profiles) == 1 {
		cfg.ActiveProfile = name
	}

	if err := config.Save(cfg, cfgPath); err != nil {
		return err
	}

	fmt.Printf("\nProfile %q saved to %s\n", name, cfgPath)
	if cfg.ActiveProfile == name {
		fmt.Printf("Set as active profile.\n")
	}
	return nil
}

func runAuthList(cmd *cobra.Command, args []string) error {
	cfgPath := config.DefaultPath()

	if !config.Exists(cfgPath) {
		fmt.Println("No config file found. Run `jiratui auth add` to create one.")
		return nil
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		return err
	}

	if len(cfg.Profiles) == 0 {
		fmt.Println("No profiles configured. Run `jiratui auth add` to add one.")
		return nil
	}

	fmt.Println("Profiles:")
	for name, p := range cfg.Profiles {
		active := " "
		if name == cfg.ActiveProfile {
			active = "*"
		}
		fmt.Printf("  %s %s — %s (%s)\n", active, name, p.URL, p.Email)
	}
	return nil
}

func runAuthSwitch(cmd *cobra.Command, args []string) error {
	cfgPath := config.DefaultPath()

	cfg, err := config.Load(cfgPath)
	if err != nil {
		return err
	}

	name := args[0]
	if _, ok := cfg.Profiles[name]; !ok {
		return fmt.Errorf("profile %q not found", name)
	}

	cfg.ActiveProfile = name

	if err := config.Save(cfg, cfgPath); err != nil {
		return err
	}

	fmt.Printf("Switched active profile to %q\n", name)
	return nil
}
