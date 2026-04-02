package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"syscall"

	"github.com/christopherdobbyn/jiratui/internal/config"
	"github.com/christopherdobbyn/jiratui/internal/jira"
	"github.com/spf13/cobra"
	"golang.org/x/term"
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

func promptSecret(label string) (string, error) {
	fmt.Printf("%s: ", label)

	fd := int(syscall.Stdin)
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		// Fallback to ReadPassword if raw mode fails (e.g. piped input)
		bytes, err := term.ReadPassword(fd)
		fmt.Println()
		if err != nil {
			return "", fmt.Errorf("reading secret input: %w", err)
		}
		return strings.TrimSpace(string(bytes)), nil
	}
	defer term.Restore(fd, oldState)

	var result []byte
	buf := make([]byte, 1)
	for {
		_, err := os.Stdin.Read(buf)
		if err != nil {
			break
		}
		ch := buf[0]
		switch {
		case ch == '\r' || ch == '\n':
			// Enter pressed — done
			fmt.Print("\r\n")
			term.Restore(fd, oldState)
			return strings.TrimSpace(string(result)), nil
		case ch == 3:
			// Ctrl+C
			fmt.Print("\r\n")
			term.Restore(fd, oldState)
			return "", fmt.Errorf("cancelled")
		case ch == 127 || ch == 8:
			// Backspace
			if len(result) > 0 {
				result = result[:len(result)-1]
				fmt.Print("\b \b") // erase last asterisk
			}
		default:
			result = append(result, ch)
			fmt.Print("*")
		}
	}

	fmt.Print("\r\n")
	return strings.TrimSpace(string(result)), nil
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
	fmt.Println()
	fmt.Println("You'll need a Jira API token. Create one here:")
	fmt.Println("  https://id.atlassian.com/manage-profile/security/api-tokens")
	fmt.Println()
	fmt.Println("Note: Jira Cloud personal API tokens have the same access as your")
	fmt.Println("account — there are no granular scopes. For scoped access, you'd")
	fmt.Println("need an OAuth 2.0 app (not supported by jiratui yet).")
	fmt.Println()

	name := prompt(reader, "👤 Profile name (e.g. work, personal)")

	// Check if profile already exists
	if _, exists := cfg.Profiles[name]; exists {
		fmt.Printf("\nProfile %q already exists.\n", name)
		fmt.Println("  [r] Replace it with a new profile")
		fmt.Println("  [n] Choose a different name")
		fmt.Println("  [q] Cancel")
		choice := strings.ToLower(prompt(reader, "Choice"))
		switch choice {
		case "r":
			delete(cfg.Profiles, name)
		case "n":
			name = prompt(reader, "👤 New profile name")
			if _, exists := cfg.Profiles[name]; exists {
				return fmt.Errorf("profile %q also already exists", name)
			}
		default:
			fmt.Println("Cancelled.")
			return nil
		}
		fmt.Println()
	}

	defaultURL := fmt.Sprintf("https://%s.atlassian.net", name)
	url := prompt(reader, fmt.Sprintf("🌐 Jira URL [%s]", defaultURL))
	if url == "" {
		url = defaultURL
	}
	email := prompt(reader, "📧 Email")

	token, err := promptSecret("🔒 API token")
	if err != nil {
		return err
	}

	if token == "" {
		return fmt.Errorf("API token cannot be empty")
	}

	// Normalize URL: strip trailing slash
	url = strings.TrimRight(url, "/")

	// Verify credentials before saving
	fmt.Printf("\nVerifying credentials against %s...\n", url)
	client := jira.NewClient(url, email, token)
	displayName, err := client.VerifyCredentials()
	if err != nil {
		return fmt.Errorf("credential verification failed: %w", err)
	}
	fmt.Printf("Authenticated as: %s\n", displayName)

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
