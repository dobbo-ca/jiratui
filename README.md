# jt

A fast, lightweight terminal UI for Jira Cloud.

## Install

```bash
brew tap dobbo-ca/taps
brew update
brew install jt
```

## Setup

On first run, `jt` will prompt you to configure a Jira Cloud profile:

```bash
jt
```

You'll need:
- Your Jira Cloud URL (e.g. `https://yourorg.atlassian.net`)
- Your email address
- A Jira API token ([create one here](https://id.atlassian.com/manage-profile/security/api-tokens))

To manage profiles later:

```bash
jt auth add          # Add a new profile
jt auth list         # List all profiles
jt auth switch <name> # Switch active profile
```

## Usage

Launch the TUI:

```bash
jt
```

### Keyboard Shortcuts

| Key | Action |
|-----|--------|
| `j/k` or `Up/Down` | Navigate issues |
| `1-4` | Switch detail tabs (Details, Comments, Associations, Attachments) |
| `F1` | Open/close Claude Code session for current ticket |
| `F2` | Full-screen Claude Code (tmux attach, `Ctrl-B d` to return) |
| `f` | Toggle filter bar |
| `/` | Search |
| `p` | Switch project |
| `o` | Open issue in browser |
| `r` | Refresh |
| `?` | Help |
| `q` | Quit |

## CLI Commands

These commands can be used for automation or are available to Claude Code sessions launched from within `jt`.

### List issues

```bash
jt issues
```

### Attach a file to a ticket

```bash
jt attach <project-key> <issue-key> <file-path>
```

### Update a ticket's description

```bash
jt update-description <project-key> <issue-key> <file-path>
```

Reads the file as markdown and updates the ticket description (converts to Atlassian Document Format automatically).

### Download an attachment

```bash
jt download-attachment <project-key> <issue-key> <filename> [dest-dir]
```

Downloads the named attachment from a ticket. If `dest-dir` is omitted, saves to the current directory.
