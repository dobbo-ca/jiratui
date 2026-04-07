package tui

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/christopherdobbyn/jiratui/internal/config"
	"github.com/christopherdobbyn/jiratui/internal/jira"
	"github.com/christopherdobbyn/jiratui/internal/models"
	"github.com/muesli/ansi"
)

// truncateAnsi truncates a string with ANSI escape codes to a given visual width.
func truncateAnsi(s string, maxWidth int) string {
	if lipgloss.Width(s) <= maxWidth {
		return s
	}
	var (
		result strings.Builder
		vis    int
		i      int
	)
	for i < len(s) {
		// Skip ANSI escape sequences
		if s[i] == '\x1b' {
			j := i
			for j < len(s) && s[j] != 'm' {
				j++
			}
			if j < len(s) {
				j++ // include the 'm'
			}
			result.WriteString(s[i:j])
			i = j
			continue
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		w := ansi.PrintableRuneWidth(string(r))
		if vis+w > maxWidth {
			break
		}
		result.WriteRune(r)
		vis += w
		i += size
	}
	// Append reset to close any open ANSI sequences
	result.WriteString("\x1b[0m")
	return result.String()
}

// skipAnsi returns the portion of an ANSI-encoded string starting at visual
// column skipWidth. It prefixes the result with a reset and re-applies the
// last SGR sequence encountered so colours carry over correctly.
func skipAnsi(s string, skipWidth int) string {
	var (
		vis     int
		i       int
		lastSGR string // most recent ANSI SGR sequence
	)
	for i < len(s) && vis < skipWidth {
		if s[i] == '\x1b' {
			j := i
			for j < len(s) && s[j] != 'm' {
				j++
			}
			if j < len(s) {
				j++
			}
			lastSGR = s[i:j]
			i = j
			continue
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		vis += ansi.PrintableRuneWidth(string(r))
		i += size
	}
	rest := s[i:]
	if rest == "" {
		return ""
	}
	// Reset any colours leaking from the overlay, then restore the
	// original line's style so the right portion renders correctly.
	prefix := "\x1b[0m"
	if lastSGR != "" {
		prefix += lastSGR
	}
	return prefix + rest
}

type state int

const (
	stateLoading state = iota
	stateList
)

// minDetailWidth is the minimum width for the detail pane.
// Ensures the 4-column layout (row 2) and calendar overlay (min 24 cols) render properly.
const minDetailWidth = 80

// defaultListWidth calculates the default list pane width (~35% of terminal).
func defaultListWidth(totalWidth int) int {
	w := totalWidth * 35 / 100
	if w < 30 {
		w = 30
	}
	if w > 80 {
		w = 80
	}
	return w
}

// usableWidth returns the terminal width minus a right margin to avoid
// overlapping the terminal scrollbar.
func (a App) usableWidth() int {
	return a.width - 1
}

func (a App) detailPaneWidth() int {
	return a.usableWidth() - a.listWidth - 1
}

// issuesMsg carries fetched issues into the model.
type issuesMsg struct {
	issues []models.Issue
}

// errMsg carries errors into the model.
type errMsg struct {
	err error
}

// issueDetailMsg carries a fetched issue detail into the model.
type issueDetailMsg struct {
	issue  models.Issue
	forKey string // which issue key this detail is for
}

// assignableUsersMsg carries fetched assignable users for a project.
type assignableUsersMsg struct {
	users      []models.User
	projectKey string
}

// boardAssigneesMsg carries unique assignees from all project issues.
type boardAssigneesMsg struct {
	users      []models.User
	projectKey string
}

func fetchBoardAssignees(client *jira.Client, projectKey string) tea.Cmd {
	return func() tea.Msg {
		jql := fmt.Sprintf(`project = "%s" AND assignee IS NOT EMPTY ORDER BY updated DESC`, projectKey)
		result, err := client.SearchIssues(jql, 100, "")
		if err != nil {
			return nil
		}
		seen := make(map[string]bool)
		var users []models.User
		for _, issue := range result.Issues {
			if issue.Assignee != nil && !seen[issue.Assignee.AccountID] {
				seen[issue.Assignee.AccountID] = true
				users = append(users, *issue.Assignee)
			}
		}
		return boardAssigneesMsg{users: users, projectKey: projectKey}
	}
}

// transitionsMsg carries fetched transitions for an issue.
type transitionsMsg struct {
	transitions []models.Transition
	forKey      string
}

func fetchIssueDetail(client *jira.Client, key string) tea.Cmd {
	return func() tea.Msg {
		issue, err := client.GetIssue(key)
		if err != nil {
			return errMsg{err: err}
		}
		return issueDetailMsg{issue: *issue, forKey: key}
	}
}

func searchAssignableUsers(client *jira.Client, query, projectKey string) tea.Cmd {
	return func() tea.Msg {
		users, err := client.SearchUsers(query, projectKey)
		if err != nil {
			return nil
		}
		return assignableUsersMsg{users: users, projectKey: projectKey}
	}
}

func fetchTransitions(client *jira.Client, issueKey string) tea.Cmd {
	return func() tea.Msg {
		transitions, err := client.GetTransitions(issueKey)
		if err != nil {
			return nil
		}
		return transitionsMsg{transitions: transitions, forKey: issueKey}
	}
}

// prioritiesMsg carries fetched priorities.
type prioritiesMsg struct {
	priorities []models.Priority
}

func fetchPriorities(client *jira.Client) tea.Cmd {
	return func() tea.Msg {
		priorities, err := client.GetPriorities()
		if err != nil {
			return nil
		}
		return prioritiesMsg{priorities: priorities}
	}
}

// parentSearchResultsMsg carries search results for the parent dropdown.
type parentSearchResultsMsg struct {
	items  []DropdownItem
	forKey string
}

func searchParentIssues(client *jira.Client, query, forKey string) tea.Cmd {
	return func() tea.Msg {
		escaped := strings.ReplaceAll(query, `"`, `\"`)
		jql := fmt.Sprintf(`summary ~ "%s" AND issuetype = Epic ORDER BY updated DESC`, escaped)
		result, err := client.SearchIssues(jql, 10, "")
		if err != nil {
			return nil
		}
		items := make([]DropdownItem, len(result.Issues))
		for i, issue := range result.Issues {
			items[i] = DropdownItem{
				ID:    issue.Key,
				Label: issue.Key + " - " + issue.Summary,
			}
		}
		return parentSearchResultsMsg{items: items, forKey: forKey}
	}
}

type labelsUpdatedMsg struct {
	forKey string
	labels []string
}

func updateLabels(client *jira.Client, issueKey string, labels []string) tea.Cmd {
	return func() tea.Msg {
		err := client.UpdateLabels(issueKey, labels)
		if err != nil {
			logDebug("UpdateLabels(%s) error: %v", issueKey, err)
			return nil
		}
		return labelsUpdatedMsg{forKey: issueKey, labels: labels}
	}
}

type fieldUpdatedMsg struct {
	forKey string
	field  string
}

func updateField(client *jira.Client, issueKey, field, value string) tea.Cmd {
	return func() tea.Msg {
		var err error
		switch field {
		case "summary":
			err = client.UpdateSummary(issueKey, value)
		case "assignee":
			err = client.UpdateAssignee(issueKey, value)
		case "priority":
			err = client.UpdatePriority(issueKey, value)
		case "duedate":
			err = client.UpdateDueDate(issueKey, value)
		case "parent":
			err = client.UpdateParent(issueKey, value)
		}
		if err != nil {
			logDebug("updateField(%s, %s, %s) error: %v", issueKey, field, value, err)
			return nil
		}
		logDebug("updateField(%s, %s, %s) success", issueKey, field, value)
		return fieldUpdatedMsg{forKey: issueKey, field: field}
	}
}

func updateDescription(client *jira.Client, issueKey, markdownBody string) tea.Cmd {
	return func() tea.Msg {
		err := client.UpdateDescription(issueKey, markdownBody)
		if err != nil {
			logDebug("UpdateDescription(%s) error: %v", issueKey, err)
			return nil
		}
		logDebug("UpdateDescription(%s) success", issueKey)
		return fieldUpdatedMsg{forKey: issueKey, field: "description"}
	}
}

func transitionIssue(client *jira.Client, issueKey, transitionID string) tea.Cmd {
	return func() tea.Msg {
		err := client.TransitionIssue(issueKey, transitionID)
		if err != nil {
			logDebug("transitionIssue(%s, %s) error: %v", issueKey, transitionID, err)
			return nil
		}
		logDebug("transitionIssue(%s, %s) success", issueKey, transitionID)
		return fieldUpdatedMsg{forKey: issueKey, field: "status"}
	}
}

// Comment action messages
type commentAddedMsg struct{ forKey string }
type commentEditedMsg struct{ forKey string }
type commentDeletedMsg struct{ forKey string }

type commentsRefreshedMsg struct {
	forKey   string
	comments []models.Comment
}

func refreshComments(client *jira.Client, issueKey string) tea.Cmd {
	return func() tea.Msg {
		issue, err := client.GetIssue(issueKey)
		if err != nil {
			logDebug("refreshComments(%s) error: %v", issueKey, err)
			return nil
		}
		return commentsRefreshedMsg{forKey: issueKey, comments: issue.Comments}
	}
}

func addComment(client *jira.Client, issueKey, body string) tea.Cmd {
	return func() tea.Msg {
		err := client.AddComment(issueKey, body)
		if err != nil {
			logDebug("AddComment(%s) error: %v", issueKey, err)
			return nil
		}
		logDebug("AddComment(%s) success", issueKey)
		return commentAddedMsg{forKey: issueKey}
	}
}

func editComment(client *jira.Client, issueKey, commentID, body string) tea.Cmd {
	return func() tea.Msg {
		err := client.UpdateComment(issueKey, commentID, body)
		if err != nil {
			logDebug("UpdateComment(%s, %s) error: %v", issueKey, commentID, err)
			return nil
		}
		logDebug("UpdateComment(%s, %s) success", issueKey, commentID)
		return commentEditedMsg{forKey: issueKey}
	}
}

func deleteComment(client *jira.Client, issueKey, commentID string) tea.Cmd {
	return func() tea.Msg {
		err := client.DeleteComment(issueKey, commentID)
		if err != nil {
			logDebug("DeleteComment(%s, %s) error: %v", issueKey, commentID, err)
			return nil
		}
		logDebug("DeleteComment(%s, %s) success", issueKey, commentID)
		return commentDeletedMsg{forKey: issueKey}
	}
}

// Link action messages
type linkTypesMsg struct {
	types []models.LinkType
}
type linkCreatedMsg struct{ forKey string }
type linkDeletedMsg struct{ forKey string }
type linksRefreshedMsg struct {
	forKey string
	links  []models.IssueLink
}

func fetchLinkTypes(client *jira.Client) tea.Cmd {
	return func() tea.Msg {
		types, err := client.GetLinkTypes()
		if err != nil {
			logDebug("GetLinkTypes error: %v", err)
			return nil
		}
		return linkTypesMsg{types: types}
	}
}

func createLink(client *jira.Client, issueKey, targetKey, linkTypeName, direction string) tea.Cmd {
	return func() tea.Msg {
		err := client.CreateLink(issueKey, targetKey, linkTypeName, direction)
		if err != nil {
			logDebug("CreateLink(%s, %s, %s) error: %v", issueKey, targetKey, linkTypeName, err)
			return nil
		}
		return linkCreatedMsg{forKey: issueKey}
	}
}

func deleteLink(client *jira.Client, issueKey, linkID string) tea.Cmd {
	return func() tea.Msg {
		err := client.DeleteLink(linkID)
		if err != nil {
			logDebug("DeleteLink(%s) error: %v", linkID, err)
			return nil
		}
		return linkDeletedMsg{forKey: issueKey}
	}
}

func refreshLinks(client *jira.Client, issueKey string) tea.Cmd {
	return func() tea.Msg {
		issue, err := client.GetIssue(issueKey)
		if err != nil {
			logDebug("refreshLinks(%s) error: %v", issueKey, err)
			return nil
		}
		return linksRefreshedMsg{forKey: issueKey, links: issue.Links}
	}
}

// statusesAndTypesMsg carries fetched statuses and issue types for a project.
type statusesAndTypesMsg struct {
	statuses   []models.Status
	issueTypes []models.IssueType
}

func fetchStatusesAndTypes(client *jira.Client, projectKey string) tea.Cmd {
	return func() tea.Msg {
		if projectKey == "" {
			return nil
		}
		statuses, issueTypes, err := client.GetProjectStatusesAndTypes(projectKey)
		if err != nil {
			logDebug("GetProjectStatusesAndTypes(%s) error: %v", projectKey, err)
			return nil
		}
		return statusesAndTypesMsg{statuses: statuses, issueTypes: issueTypes}
	}
}

func fetchIssuesWithJQL(client *jira.Client, jql string) tea.Cmd {
	return func() tea.Msg {
		result, err := client.SearchIssues(jql, 50, "")
		if err != nil {
			return errMsg{err: err}
		}
		return issuesMsg{issues: result.Issues}
	}
}

func isFilterBarKey(key string) bool {
	switch key {
	case "t", "s", "p", "a", "l", "d", "D", "o", "O", "x", "S":
		return true
	}
	return false
}

type borderFadeTickMsg struct{}

type attachmentOpenedMsg struct{}

func openAttachment(client *jira.Client, att models.Attachment) tea.Cmd {
	return func() tea.Msg {
		data, err := client.DownloadURL(att.URL)
		if err != nil {
			logDebug("DownloadAttachment(%s) error: %v", att.Filename, err)
			return nil
		}
		tmpFile, err := os.CreateTemp("", "jiratui-*-"+att.Filename)
		if err != nil {
			logDebug("CreateTemp error: %v", err)
			return nil
		}
		if _, err := tmpFile.Write(data); err != nil {
			tmpFile.Close()
			logDebug("Write temp file error: %v", err)
			return nil
		}
		tmpFile.Close()

		// Open with system viewer
		cmd := exec.Command("open", tmpFile.Name())
		if err := cmd.Start(); err != nil {
			logDebug("open command error: %v", err)
		}
		return attachmentOpenedMsg{}
	}
}

type attachmentUploadedMsg struct{ forKey string }
type attachmentUploadCancelledMsg struct{}

// openFileDialog opens a native file picker and returns the selected path.
// Uses osascript on macOS, zenity on Linux, powershell on Windows.
func openFileDialog() (string, error) {
	switch runtime.GOOS {
	case "darwin":
		out, err := exec.Command("osascript", "-e",
			`POSIX path of (choose file with prompt "Select file to upload")`).Output()
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(string(out)), nil
	case "windows":
		out, err := exec.Command("powershell", "-Command",
			`Add-Type -AssemblyName System.Windows.Forms; `+
				`$f = New-Object System.Windows.Forms.OpenFileDialog; `+
				`if ($f.ShowDialog() -eq 'OK') { $f.FileName } else { exit 1 }`).Output()
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(string(out)), nil
	default: // linux
		out, err := exec.Command("zenity", "--file-selection",
			"--title=Select file to upload").Output()
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(string(out)), nil
	}
}

func uploadAttachment(client *jira.Client, issueKey string) tea.Cmd {
	return func() tea.Msg {
		filePath, err := openFileDialog()
		if err != nil {
			logDebug("File dialog: %v", err)
			return attachmentUploadCancelledMsg{}
		}
		if filePath == "" {
			return attachmentUploadCancelledMsg{}
		}

		err = client.UploadAttachment(issueKey, filePath)
		if err != nil {
			logDebug("UploadAttachment(%s, %s) error: %v", issueKey, filePath, err)
			return attachmentUploadCancelledMsg{}
		}
		logDebug("UploadAttachment(%s, %s) success", issueKey, filePath)
		return attachmentUploadedMsg{forKey: issueKey}
	}
}

type attachmentDeletedMsg struct{ forKey string }

func deleteAttachment(client *jira.Client, issueKey, attachmentID string) tea.Cmd {
	return func() tea.Msg {
		err := client.DeleteAttachment(attachmentID)
		if err != nil {
			logDebug("DeleteAttachment(%s) error: %v", attachmentID, err)
			return nil
		}
		return attachmentDeletedMsg{forKey: issueKey}
	}
}

type attachmentsRefreshedMsg struct {
	forKey      string
	attachments []models.Attachment
}

func refreshAttachments(client *jira.Client, issueKey string) tea.Cmd {
	return func() tea.Msg {
		issue, err := client.GetIssue(issueKey)
		if err != nil {
			logDebug("refreshAttachments(%s) error: %v", issueKey, err)
			return nil
		}
		return attachmentsRefreshedMsg{forKey: issueKey, attachments: issue.Attachments}
	}
}

func logDebug(format string, args ...interface{}) {
	f, _ := os.OpenFile("/tmp/jiratui_debug.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if f != nil {
		fmt.Fprintf(f, format+"\n", args...)
		f.Close()
	}
}

// SortField represents a sortable column.
type SortField int

const (
	SortUpdated SortField = iota
	SortKey
	SortSummary
)

// SortState tracks the current sort configuration.
type SortState struct {
	Field SortField
	Asc   bool
}

func (s SortState) orderByClause() string {
	var field string
	switch s.Field {
	case SortKey:
		field = "key"
	case SortSummary:
		field = "summary"
	default:
		field = "updated"
	}
	dir := "DESC"
	if s.Asc {
		dir = "ASC"
	}
	return field + " " + dir
}

// App is the root Bubble Tea model.
type App struct {
	state         state
	list          List
	detail        *Detail
	detailLoading bool
	detailKey     string // issue key currently shown/loading in detail
	listWidth     int    // width of the list pane (draggable)
	dragging      bool    // true while dragging the border
	borderHover   bool    // true when mouse is over the draggable border
	borderFade    float64 // 0.0 = hidden, 1.0 = fully visible (for fade animation)
	borderFading  bool    // true when a fade animation tick is active
	showHelp      bool   // true when help overlay is visible
	sort          SortState
	spinner       spinner.Model
	client        *jira.Client
	profileName   string
	configPath    string              // path to config file for saving state
	projectKey    string              // current project filter (empty = all)
	projectName   string              // display name of current project
	projects      []models.Project    // available projects
	projectDrop   Dropdown            // project selector dropdown
	profileDrop   Dropdown            // workspace/profile selector dropdown
	profileNames  []string            // available profile names
	filterBar     FilterBar
	myAccountID   string              // current user's Jira account ID
	err           error
	width         int
	height        int
}

// projectsMsg carries fetched projects.
type projectsMsg struct {
	projects []models.Project
}

func fetchProjects(client *jira.Client) tea.Cmd {
	return func() tea.Msg {
		projects, err := client.GetProjects()
		if err != nil {
			return nil
		}
		return projectsMsg{projects: projects}
	}
}

// NewApp creates the root TUI model.
func NewApp(client *jira.Client, profileName, initialProject, configPath string, profileNames []string) App {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(colorAccent)

	projectName := "All Projects"
	if initialProject != "" {
		projectName = initialProject
	}
	projectDrop := NewDropdown("Project", nil, projectName, initialProject, 30)
	projectDrop.SetPinnedItems([]DropdownItem{{ID: "", Label: "All Projects"}})
	projectDrop.SetValueColor(colorInfo)

	// Profile/workspace dropdown
	profileItems := make([]DropdownItem, len(profileNames))
	for i, name := range profileNames {
		profileItems[i] = DropdownItem{ID: name, Label: name}
	}
	profileDrop := NewSimpleDropdown("Workspace", profileItems, profileName, profileName, 25)
	profileDrop.SetValueColor(colorSuccess)

	// Fetch current user's account ID (best effort)
	myAccountID, _, _ := client.GetMyself()

	filterBar := NewFilterBar(0) // width set on first WindowSizeMsg
	if myAccountID != "" {
		filterBar.SetDefaultAssignee(myAccountID, "")
	}

	// Restore saved filters for the current project
	if configPath != "" {
		if cfg, err := config.Load(configPath); err == nil {
			if profile, ok := cfg.Profiles[profileName]; ok && profile.Filters != nil {
				if sf, ok := profile.Filters[initialProject]; ok {
					filterBar.RestoreFilters(sf)
				}
			}
		}
	}

	return App{
		state:        stateLoading,
		spinner:      s,
		client:       client,
		profileName:  profileName,
		configPath:   configPath,
		projectKey:   initialProject,
		projectName:  projectName,
		projectDrop:  projectDrop,
		profileDrop:  profileDrop,
		profileNames: profileNames,
		filterBar:    filterBar,
		myAccountID:  myAccountID,
	}
}

// profileSwitchedMsg signals that the active profile was changed and the app should restart.
type profileSwitchedMsg struct{}

func switchProfile(configPath, newProfile string) tea.Cmd {
	return func() tea.Msg {
		if configPath == "" {
			return nil
		}
		cfg, err := config.Load(configPath)
		if err != nil {
			return nil
		}
		cfg.ActiveProfile = newProfile
		_ = config.Save(cfg, configPath)
		return profileSwitchedMsg{}
	}
}

func saveProjectToConfig(configPath, profileName, projectKey string) tea.Cmd {
	return func() tea.Msg {
		if configPath == "" {
			return nil
		}
		cfg, err := config.Load(configPath)
		if err != nil {
			return nil
		}
		profile, ok := cfg.Profiles[profileName]
		if !ok {
			return nil
		}
		profile.Project = projectKey
		cfg.Profiles[profileName] = profile
		_ = config.Save(cfg, configPath)
		return nil
	}
}

func saveFiltersToConfig(configPath, profileName, projectKey string, filters config.SavedFilters) tea.Cmd {
	return func() tea.Msg {
		if configPath == "" {
			return nil
		}
		cfg, err := config.Load(configPath)
		if err != nil {
			return nil
		}
		profile, ok := cfg.Profiles[profileName]
		if !ok {
			return nil
		}
		if profile.Filters == nil {
			profile.Filters = make(map[string]config.SavedFilters)
		}
		profile.Filters[projectKey] = filters
		cfg.Profiles[profileName] = profile
		_ = config.Save(cfg, configPath)
		return nil
	}
}

func sortProjectsByKey(projects []models.Project) {
	for i := 1; i < len(projects); i++ {
		for j := i; j > 0 && projects[j].Key < projects[j-1].Key; j-- {
			projects[j], projects[j-1] = projects[j-1], projects[j]
		}
	}
}

func fetchIssues(client *jira.Client, sort SortState, projectKey string) tea.Cmd {
	return func() tea.Msg {
		result, err := client.SearchMyIssues(50, "", sort.orderByClause(), projectKey)
		if err != nil {
			return errMsg{err: err}
		}
		return issuesMsg{issues: result.Issues}
	}
}

// Init starts the spinner and fires the initial data fetch.
func (a App) Init() tea.Cmd {
	var issueFetch tea.Cmd
	if a.filterBar.HasActiveFilters() {
		jql := a.filterBar.BuildJQL(a.projectKey, a.sort.orderByClause())
		issueFetch = fetchIssuesWithJQL(a.client, jql)
	} else {
		issueFetch = fetchIssues(a.client, a.sort, a.projectKey)
	}
	return tea.Batch(
		a.spinner.Tick,
		issueFetch,
		fetchProjects(a.client),
		fetchStatusesAndTypes(a.client, a.projectKey),
	)
}

// Update handles all messages.
func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		// Initialize or clamp list width
		if a.listWidth == 0 {
			a.listWidth = defaultListWidth(msg.Width)
		}
		if a.listWidth > a.usableWidth()-minDetailWidth {
			a.listWidth = a.usableWidth() - minDetailWidth
		}
		if a.listWidth < 20 {
			a.listWidth = 20
		}
		a.filterBar.SetWidth(a.usableWidth())
		if a.state == stateList {
			a.list.width = msg.Width
			a.list.height = msg.Height
			a.list.clampCursor()
			if a.detail != nil {
				detailWidth := a.detailPaneWidth()
				adjusted := tea.WindowSizeMsg{Width: detailWidth, Height: msg.Height - 1 - a.filterBar.Height()}
				d := *a.detail
				d, _ = d.Update(adjusted)
				a.detail = &d
			}
		}
		return a, nil

	case tea.KeyMsg:
		// Global quit — always works
		if msg.String() == "ctrl+c" {
			return a, tea.Quit
		}

		// When project dropdown is open, forward keys to it
		if a.projectDrop.IsOpen() {
			var cmd tea.Cmd
			a.projectDrop, cmd = a.projectDrop.Update(msg)
			if !a.projectDrop.IsOpen() {
				sel := a.projectDrop.SelectedItem()
				newKey := ""
				newName := "All Projects"
				if sel != nil {
					newKey = sel.ID
					newName = sel.Label
				}
				if newKey != a.projectKey {
					a.projectKey = newKey
					a.projectName = newName
					a.state = stateLoading
					a.detail = nil
					a.detailKey = ""
					a.filterBar.ClearAll()
					return a, tea.Batch(
						a.spinner.Tick,
						fetchIssues(a.client, a.sort, a.projectKey),
						saveProjectToConfig(a.configPath, a.profileName, a.projectKey),
						fetchStatusesAndTypes(a.client, a.projectKey),
					)
				}
			}
			return a, cmd
		}

		// When profile dropdown is open, forward keys to it
		if a.profileDrop.IsOpen() {
			var cmd tea.Cmd
			a.profileDrop, cmd = a.profileDrop.Update(msg)
			if !a.profileDrop.IsOpen() {
				sel := a.profileDrop.SelectedItem()
				if sel != nil && sel.ID != a.profileName {
					// Switch active profile and save
					return a, switchProfile(a.configPath, sel.ID)
				}
			}
			return a, cmd
		}

		// When detail has a focused input, forward all keys to it
		if a.detail != nil && a.detail.Editing() {
			d := *a.detail
			var cmd tea.Cmd
			d, cmd = d.Update(msg)
			a.detail = &d
			return a, cmd
		}

		// Filter bar: toggle with f, forward / to expand+search
		if a.state == stateList {
			if msg.String() == "f" && !a.filterBar.ActiveDropdown() {
				a.filterBar.Toggle()
				return a, nil
			}
			if msg.String() == "/" {
				if !a.filterBar.IsExpanded() {
					a.filterBar.Expand()
				}
				var cmd tea.Cmd
				a.filterBar, cmd = a.filterBar.Update(msg)
				return a, cmd
			}
		}

		// When filter bar has an active dropdown/picker/search, forward keys to it
		if a.filterBar.IsExpanded() && (a.filterBar.ActiveDropdown() || isFilterBarKey(msg.String())) {
			var cmd tea.Cmd
			a.filterBar, cmd = a.filterBar.Update(msg)
			return a, cmd
		}

		// Help overlay toggle
		if msg.String() == "?" {
			if a.showHelp {
				a.showHelp = false
			} else {
				a.showHelp = true
			}
			return a, nil
		}
		if a.showHelp {
			// Any key dismisses help
			a.showHelp = false
			return a, nil
		}
		if a.state == stateList {
			if msg.String() == "q" && !a.filterBar.ActiveDropdown() {
				return a, tea.Quit
			}
			if msg.String() == "p" && !a.filterBar.ActiveDropdown() {
				return a, a.projectDrop.OpenDropdown()
			}
			if msg.String() == "r" && !a.filterBar.ActiveDropdown() {
				a.state = stateLoading
				a.err = nil
				a.detail = nil
				a.detailKey = ""
				jql := a.filterBar.BuildJQL(a.projectKey, a.sort.orderByClause())
				return a, tea.Batch(a.spinner.Tick, fetchIssuesWithJQL(a.client, jql))
			}
			if a.detail != nil && !a.filterBar.ActiveDropdown() {
				if msg.String() >= "1" && msg.String() <= "4" {
					d := *a.detail
					d, _ = d.Update(msg)
					a.detail = &d
					return a, nil
				}
			}
		}

	case borderFadeTickMsg:
		step := 0.2
		if a.borderHover || a.dragging {
			a.borderFade += step
			if a.borderFade >= 1.0 {
				a.borderFade = 1.0
				a.borderFading = false
				return a, nil
			}
		} else {
			a.borderFade -= step
			if a.borderFade <= 0.0 {
				a.borderFade = 0.0
				a.borderFading = false
				return a, nil
			}
		}
		return a, tea.Tick(30*time.Millisecond, func(t time.Time) tea.Msg {
			return borderFadeTickMsg{}
		})

	case filterChangedMsg:
		jql := a.filterBar.BuildJQL(a.projectKey, a.sort.orderByClause())
		a.state = stateLoading
		a.detail = nil
		a.detailKey = ""
		return a, tea.Batch(a.spinner.Tick, fetchIssuesWithJQL(a.client, jql))

	case filterSaveMsg:
		return a, saveFiltersToConfig(a.configPath, a.profileName, a.projectKey, a.filterBar.GetSavedFilters())

	case filterSearchTick:
		var cmd tea.Cmd
		a.filterBar, cmd = a.filterBar.Update(msg)
		return a, cmd

	case statusesAndTypesMsg:
		statusItems := make([]DropdownItem, len(msg.statuses))
		for i, s := range msg.statuses {
			statusItems[i] = DropdownItem{ID: s.ID, Label: s.Name}
		}
		a.filterBar.SetStatusItems(statusItems)

		typeItems := make([]DropdownItem, len(msg.issueTypes))
		for i, it := range msg.issueTypes {
			typeItems[i] = DropdownItem{ID: it.ID, Label: it.Name}
		}
		a.filterBar.SetTypeItems(typeItems)
		return a, nil

	case issuesMsg:
		a.list = NewList(msg.issues, a.width, a.height)
		// Restore sort indicators
		switch a.sort.Field {
		case SortKey:
			a.list.sortCol = "key"
		case SortSummary:
			a.list.sortCol = "summary"
		}
		a.list.sortAsc = a.sort.Asc
		a.state = stateList
		// Extract unique labels for filter bar
		labelSet := make(map[string]bool)
		for _, issue := range msg.issues {
			for _, l := range issue.Labels {
				labelSet[l] = true
			}
		}
		if len(labelSet) > 0 {
			labelItems := make([]DropdownItem, 0, len(labelSet))
			for l := range labelSet {
				labelItems = append(labelItems, DropdownItem{ID: l, Label: l})
			}
			a.filterBar.SetLabelItems(labelItems)
		}
		// Auto-fetch first issue detail
		if len(msg.issues) > 0 {
			a.detailLoading = true
			a.detailKey = msg.issues[0].Key
			return a, tea.Batch(a.spinner.Tick, fetchIssueDetail(a.client, msg.issues[0].Key))
		}
		return a, nil

	case sortClickMsg:
		// Toggle sort: same column flips direction, different column sorts asc
		var newField SortField
		switch msg.column {
		case "key":
			newField = SortKey
		case "summary":
			newField = SortSummary
		default:
			newField = SortUpdated
		}
		if a.sort.Field == newField {
			a.sort.Asc = !a.sort.Asc
		} else {
			a.sort.Field = newField
			a.sort.Asc = true
		}
		// Update list sort state for header indicators
		a.list.sortCol = msg.column
		a.list.sortAsc = a.sort.Asc
		// Re-fetch with new sort
		a.state = stateLoading
		a.detail = nil
		a.detailKey = ""
		jql := a.filterBar.BuildJQL(a.projectKey, a.sort.orderByClause())
		return a, tea.Batch(a.spinner.Tick, fetchIssuesWithJQL(a.client, jql))

	case sortChangedMsg:
		// Sync sort state from filterBar's sort controls
		field := a.filterBar.SortField()
		switch field {
		case "key":
			a.sort.Field = SortKey
		case "summary":
			a.sort.Field = SortSummary
		default:
			a.sort.Field = SortUpdated
		}
		a.sort.Asc = a.filterBar.SortAsc()
		a.list.sortCol = field
		a.list.sortAsc = a.sort.Asc
		a.state = stateLoading
		a.detail = nil
		a.detailKey = ""
		jql := a.filterBar.BuildJQL(a.projectKey, a.filterBar.OrderByClause())
		return a, tea.Batch(a.spinner.Tick, fetchIssuesWithJQL(a.client, jql))

	case cursorChangedMsg:
		// Only fetch if it's a different issue
		if msg.issueKey != a.detailKey {
			a.detailLoading = true
			a.detailKey = msg.issueKey
			return a, tea.Batch(a.spinner.Tick, fetchIssueDetail(a.client, msg.issueKey))
		}
		return a, nil

	case issueDetailMsg:
		// Only accept if this is still the issue we're waiting for
		if msg.forKey == a.detailKey {
			contentHeight := a.height - 1 - a.filterBar.Height()
			detailWidth := a.detailPaneWidth()
			d := NewDetail(msg.issue, detailWidth, contentHeight)
			d.myAccountID = a.myAccountID
			issueKey := msg.issue.Key
			client := a.client
			d.SetParentSearchFunc(func(query string) tea.Cmd {
				return searchParentIssues(client, query, issueKey)
			})
			d.OnLabelsChanged = func(key string, labels []string) tea.Cmd {
				return updateLabels(client, key, labels)
			}
			d.OnSummaryChanged = func(key, summary string) tea.Cmd {
				return updateField(client, key, "summary", summary)
			}
			d.OnDescriptionChanged = func(key, description string) tea.Cmd {
				return updateDescription(client, key, description)
			}
			d.OnAssigneeChanged = func(key, accountID string) tea.Cmd {
				return updateField(client, key, "assignee", accountID)
			}
			d.OnStatusChanged = func(key, transitionID string) tea.Cmd {
				return transitionIssue(client, key, transitionID)
			}
			d.OnPriorityChanged = func(key, priorityID string) tea.Cmd {
				return updateField(client, key, "priority", priorityID)
			}
			d.OnDueDateChanged = func(key, dueDate string) tea.Cmd {
				return updateField(client, key, "duedate", dueDate)
			}
			d.OnParentChanged = func(key, parentKey string) tea.Cmd {
				return updateField(client, key, "parent", parentKey)
			}
			d.OnCommentAdd = func(key, body string) tea.Cmd {
				return addComment(client, key, body)
			}
			d.OnCommentEdit = func(key, commentID, body string) tea.Cmd {
				return editComment(client, key, commentID, body)
			}
			d.OnCommentDelete = func(key, commentID string) tea.Cmd {
				return deleteComment(client, key, commentID)
			}
			d.OnLinkCreate = func(key, targetKey, typeName, direction string) tea.Cmd {
				return createLink(client, key, targetKey, typeName, direction)
			}
			d.OnLinkDelete = func(linkID string) tea.Cmd {
				return deleteLink(client, issueKey, linkID)
			}
			d.OnNavigate = func(targetKey string) tea.Cmd {
				return func() tea.Msg {
					return cursorChangedMsg{issueKey: targetKey}
				}
			}
			d.OnAttachmentOpen = func(att models.Attachment) tea.Cmd {
				return openAttachment(client, att)
			}
			d.OnAttachmentUpload = func(key, _ string) tea.Cmd {
				return uploadAttachment(client, key)
			}
			d.OnAttachmentDelete = func(key, attachmentID string) tea.Cmd {
				return deleteAttachment(client, key, attachmentID)
			}
			// Set up async search for when user types in assignee search
			projectKey := msg.issue.ProjectKey
			d.assigneeDrop.OnSearch = func(query string) tea.Cmd {
				return searchAssignableUsers(client, query, projectKey)
			}
			d.assigneeDrop.minSearchLen = 0
			a.detail = &d
			a.detailLoading = false
			// Fetch dropdown data in parallel
			return a, tea.Batch(
				fetchBoardAssignees(a.client, msg.issue.ProjectKey),
				fetchTransitions(a.client, msg.issue.Key),
				fetchPriorities(a.client),
				fetchLinkTypes(a.client),
			)
		}
		return a, nil

	case boardAssigneesMsg:
		// Default assignee list: users with issues on the board
		if a.detail != nil {
			d := *a.detail
			d.SetBoardAssignees(msg.users, a.myAccountID)
			a.detail = &d
		}
		// Populate filter bar assignees
		assigneeItems := make([]DropdownItem, len(msg.users))
		for i, u := range msg.users {
			label := u.DisplayName
			if u.AccountID == a.myAccountID {
				label += " (me)"
			}
			assigneeItems[i] = DropdownItem{ID: u.AccountID, Label: label}
		}
		a.filterBar.SetAssigneeItems(assigneeItems)
		return a, nil

	case assignableUsersMsg:
		// Search results for assignee dropdown
		if a.detail != nil {
			d := *a.detail
			items := make([]DropdownItem, len(msg.users))
			for i, u := range msg.users {
				label := u.DisplayName
				if u.AccountID == a.myAccountID {
					label += " (me)"
				}
				items[i] = DropdownItem{ID: u.AccountID, Label: label}
			}
			d.assigneeDrop.SetItems(items)
			a.detail = &d
		}
		return a, nil

	case transitionsMsg:
		if a.detail != nil && msg.forKey == a.detailKey {
			d := *a.detail
			d.SetStatusOptions(msg.transitions)
			a.detail = &d
		}
		return a, nil

	case prioritiesMsg:
		// Populate priority filter dropdown
		prioItems := make([]DropdownItem, len(msg.priorities))
		for i, p := range msg.priorities {
			prioItems[i] = DropdownItem{ID: p.ID, Label: p.Name}
		}
		a.filterBar.SetPriorityItems(prioItems)

		if a.detail != nil {
			d := *a.detail
			d.SetPriorityOptions(msg.priorities)
			a.detail = &d
		}
		return a, nil

	case parentSearchResultsMsg:
		if a.detail != nil && msg.forKey == a.detailKey {
			d := *a.detail
			d.parentDrop.SetItems(msg.items)
			a.detail = &d
		}
		return a, nil

	case dropdownSearchTick:
		if a.detail != nil {
			d := *a.detail
			var cmd tea.Cmd
			switch msg.label {
			case "Parent":
				cmd = d.parentDrop.HandleSearchTick(msg)
			case "Assignee":
				cmd = d.assigneeDrop.HandleSearchTick(msg)
			}
			a.detail = &d
			return a, cmd
		}
		return a, nil

	case labelsUpdatedMsg:
		if a.detail != nil && msg.forKey == a.detailKey {
			d := *a.detail
			d.issue.Labels = msg.labels
			a.detail = &d
		}
		return a, nil

	case fieldUpdatedMsg:
		// Field update confirmed — re-fetch transitions if status changed
		if msg.field == "status" && a.detail != nil && msg.forKey == a.detailKey {
			return a, fetchTransitions(a.client, msg.forKey)
		}
		return a, nil

	case commentAddedMsg:
		if msg.forKey == a.detailKey {
			return a, refreshComments(a.client, msg.forKey)
		}
		return a, nil

	case commentEditedMsg:
		if msg.forKey == a.detailKey {
			return a, refreshComments(a.client, msg.forKey)
		}
		return a, nil

	case commentDeletedMsg:
		if msg.forKey == a.detailKey {
			return a, refreshComments(a.client, msg.forKey)
		}
		return a, nil

	case commentsRefreshedMsg:
		if a.detail != nil && msg.forKey == a.detailKey {
			d := *a.detail
			d.issue.Comments = msg.comments
			a.detail = &d
		}
		return a, nil

	case linkTypesMsg:
		if a.detail != nil {
			d := *a.detail
			d.SetLinkTypes(msg.types)
			a.detail = &d
		}
		return a, nil

	case attachmentOpenedMsg:
		if a.detail != nil {
			d := *a.detail
			d.downloadingAttach = ""
			a.detail = &d
		}
		return a, nil

	case attachmentUploadedMsg:
		if a.detail != nil {
			d := *a.detail
			d.uploadingAttach = false
			a.detail = &d
		}
		if msg.forKey == a.detailKey {
			return a, refreshAttachments(a.client, msg.forKey)
		}
		return a, nil

	case attachmentsRefreshedMsg:
		if a.detail != nil && msg.forKey == a.detailKey {
			d := *a.detail
			d.issue.Attachments = msg.attachments
			a.detail = &d
		}
		return a, nil

	case attachmentUploadCancelledMsg:
		if a.detail != nil {
			d := *a.detail
			d.uploadingAttach = false
			a.detail = &d
		}
		return a, nil

	case attachmentDeletedMsg:
		if msg.forKey == a.detailKey {
			return a, refreshAttachments(a.client, msg.forKey)
		}
		return a, nil

	case linkCreatedMsg:
		if msg.forKey == a.detailKey {
			return a, refreshLinks(a.client, msg.forKey)
		}
		return a, nil

	case linkDeletedMsg:
		if msg.forKey == a.detailKey {
			return a, refreshLinks(a.client, msg.forKey)
		}
		return a, nil

	case linksRefreshedMsg:
		if a.detail != nil && msg.forKey == a.detailKey {
			d := *a.detail
			d.issue.Links = msg.links
			a.detail = &d
		}
		return a, nil

	case projectsMsg:
		a.projects = msg.projects
		// Projects already sorted by name from API; re-sort by key
		sorted := make([]models.Project, len(msg.projects))
		copy(sorted, msg.projects)
		sortProjectsByKey(sorted)
		items := make([]DropdownItem, len(sorted))
		for i, p := range sorted {
			items[i] = DropdownItem{ID: p.Key, Label: p.Key + " — " + p.Name}
		}
		a.projectDrop.SetItems(items)
		return a, nil

	case profileSwitchedMsg:
		// Profile was changed — quit so user relaunches with new credentials
		return a, tea.Quit

	case errMsg:
		a.err = msg.err
		a.state = stateList
		a.detailLoading = false
		return a, nil

	case spinner.TickMsg:
		if a.state == stateLoading || a.detailLoading {
			var cmd tea.Cmd
			a.spinner, cmd = a.spinner.Update(msg)
			return a, cmd
		}
		return a, nil
	}

	// Forward blink messages to project dropdown when open
	if a.projectDrop.IsOpen() {
		var cmd tea.Cmd
		a.projectDrop, cmd = a.projectDrop.Update(msg)
		if cmd != nil {
			return a, cmd
		}
	}

	// Blur detail editing on any click outside the detail pane
	if a.state == stateList && a.detail != nil && a.detail.Editing() {
		if mouseMsg, ok := msg.(tea.MouseMsg); ok {
			if mouseMsg.Action == tea.MouseActionPress && mouseMsg.Button == tea.MouseButtonLeft {
				listW := a.listWidth
				fbH := a.filterBar.Height()
				inDetailPane := mouseMsg.X > listW && mouseMsg.Y >= fbH
				helpBarY := a.height - 1
				onHelpBar := mouseMsg.Y == helpBarY
				if !inDetailPane || onHelpBar {
					d := *a.detail
					cmd := d.blurAllAndSave()
					a.detail = &d
					return a, cmd
				}
			}
		}
	}

	// Forward cursor blink and other messages to detail when editing
	if a.state == stateList && a.detail != nil && a.detail.Editing() {
		// For mouse events, don't forward here — they'll be handled
		// by the regular mouse routing below with proper coordinate adjustment.
		// Only forward non-mouse messages (blink, etc.)
		if _, isMouse := msg.(tea.MouseMsg); !isMouse {
			d := *a.detail
			var cmd tea.Cmd
			d, cmd = d.Update(msg)
			a.detail = &d
			if cmd != nil {
				return a, cmd
			}
		}
	}

	// Forward remaining messages to list
	if a.state == stateList {
		// Route mouse events by pane in split mode
		if mouseMsg, ok := msg.(tea.MouseMsg); ok {
			// Handle clicks on help bar (last line) for project/profile
			if mouseMsg.Action == tea.MouseActionPress && mouseMsg.Button == tea.MouseButtonLeft {
				helpBarY := a.height - 1
				if mouseMsg.Y == helpBarY {
					projectX, profileX := a.helpBarClickZones()
					if mouseMsg.X >= profileX {
						return a, a.profileDrop.OpenDropdown()
					}
					if mouseMsg.X >= projectX {
						return a, a.projectDrop.OpenDropdown()
					}
				}
			}

			// When profile dropdown is open, intercept all mouse events
			if a.profileDrop.IsOpen() {
				if mouseMsg.Action == tea.MouseActionPress && mouseMsg.Button == tea.MouseButtonLeft {
					overlay := a.profileDrop.RenderStandaloneOverlay()
					if overlay != nil {
						overlayH := len(overlay)
						contentH := a.height - 1
						startLine := contentH - overlayH
						if startLine < 0 {
							startLine = 0
						}
						overlayW := lipgloss.Width(overlay[0])
						xOff := a.usableWidth() - overlayW

						if mouseMsg.Y >= startLine && mouseMsg.Y < startLine+overlayH &&
							mouseMsg.X >= xOff && mouseMsg.X < xOff+overlayW {
							overlayLine := mouseMsg.Y - startLine - 3
							if overlayLine >= 0 && a.profileDrop.HandleClick(overlayLine) {
								sel := a.profileDrop.SelectedItem()
								if sel != nil && sel.ID != a.profileName {
									return a, switchProfile(a.configPath, sel.ID)
								}
								return a, nil
							}
						} else {
							a.profileDrop.Close()
						}
					} else {
						a.profileDrop.Close()
					}
				}
				return a, nil
			}

			// When project dropdown is open, intercept all mouse events
			if a.projectDrop.IsOpen() {
				if mouseMsg.Action == tea.MouseActionPress && mouseMsg.Button == tea.MouseButtonLeft {
					// Calculate overlay position to check if click is on an item
					overlay := a.projectDrop.RenderStandaloneOverlay()
					if overlay != nil {
						overlayH := len(overlay)
						contentH := a.height - 1
						startLine := contentH - overlayH
						if startLine < 0 {
							startLine = 0
						}
						overlayW := lipgloss.Width(overlay[0])
						xOff := a.usableWidth() - overlayW

						// Check if click is within the overlay bounds
						if mouseMsg.Y >= startLine && mouseMsg.Y < startLine+overlayH &&
							mouseMsg.X >= xOff && mouseMsg.X < xOff+overlayW {
							overlayLine := mouseMsg.Y - startLine - 3 // subtract field header (top/mid/connector)
							if overlayLine >= 0 && a.projectDrop.HandleClick(overlayLine) {
								// Selection made — check if project changed
								sel := a.projectDrop.SelectedItem()
								newKey := ""
								newName := "All Projects"
								if sel != nil {
									newKey = sel.ID
									newName = sel.Label
								}
								if newKey != a.projectKey {
									a.projectKey = newKey
									a.projectName = newName
									a.state = stateLoading
									a.detail = nil
									a.detailKey = ""
									a.filterBar.ClearAll()
									return a, tea.Batch(
										a.spinner.Tick,
										fetchIssues(a.client, a.sort, a.projectKey),
										saveProjectToConfig(a.configPath, a.profileName, a.projectKey),
										fetchStatusesAndTypes(a.client, a.projectKey),
									)
								}
								return a, nil
							}
						} else {
							// Click outside overlay — close it
							a.projectDrop.Close()
						}
					} else {
						a.projectDrop.Close()
					}
				}
				return a, nil
			}

			// FilterBar mouse handling
			filterBarH := a.filterBar.Height()

			// When filterBar has an active dropdown overlay, intercept all mouse events
			if a.filterBar.IsExpanded() && a.filterBar.ActiveDropdown() {
				if mouseMsg.Action == tea.MouseActionPress && mouseMsg.Button == tea.MouseButtonLeft {
					overlayLines, startLine, startCol := a.filterBar.OverlayLines()
					if overlayLines != nil {
						overlayH := len(overlayLines)
						overlayW := lipgloss.Width(overlayLines[0])
						// Check if click is within the overlay bounds
						if mouseMsg.Y >= startLine && mouseMsg.Y < startLine+overlayH &&
							mouseMsg.X >= startCol && mouseMsg.X < startCol+overlayW {
							overlayLine := mouseMsg.Y - startLine
							changed := false
							if a.filterBar.typeDrop.IsOpen() {
								changed = a.filterBar.typeDrop.HandleClick(overlayLine)
							} else if a.filterBar.statusDrop.IsOpen() {
								changed = a.filterBar.statusDrop.HandleClick(overlayLine)
							} else if a.filterBar.priorityDrop.IsOpen() {
								changed = a.filterBar.priorityDrop.HandleClick(overlayLine)
							} else if a.filterBar.assigneeDrop.IsOpen() {
								changed = a.filterBar.assigneeDrop.HandleClick(overlayLine)
							} else if a.filterBar.labelsDrop.IsOpen() {
								changed = a.filterBar.labelsDrop.HandleClick(overlayLine)
							} else if a.filterBar.createdFrom.IsOpen() {
								localX := mouseMsg.X - startCol
								changed = a.filterBar.createdFrom.HandleClick(overlayLine, localX, overlayW)
							} else if a.filterBar.createdUntil.IsOpen() {
								localX := mouseMsg.X - startCol
								changed = a.filterBar.createdUntil.HandleClick(overlayLine, localX, overlayW)
							} else if a.filterBar.sortDrop.IsOpen() {
								changed = a.filterBar.sortDrop.HandleClick(overlayLine)
								if changed && !a.filterBar.sortDrop.IsOpen() {
									return a, func() tea.Msg { return sortChangedMsg{} }
								}
								return a, nil
							}
							if changed {
								return a, func() tea.Msg { return filterChangedMsg{} }
							}
						} else {
							// Click outside overlay — close all dropdowns
							a.filterBar.closeAll()
						}
						return a, nil
					}
				}
				return a, nil
			}

			// Handle clicks in the filterBar area (Y < filterBarH)
			if mouseMsg.Action == tea.MouseActionPress && mouseMsg.Button == tea.MouseButtonLeft {
				if mouseMsg.Y < filterBarH {
					if !a.filterBar.IsExpanded() {
						a.filterBar.Expand()
						return a, nil
					}
					cmd := a.filterBar.HandleFieldClick(mouseMsg.X, mouseMsg.Y)
					return a, cmd
				}
			}

			// Adjust Y for content area (below headers)
			contentMouseMsg := mouseMsg
			contentMouseMsg.Y = mouseMsg.Y - filterBarH

			listW := a.listWidth

			// Track border hover for visual feedback
			if !a.dragging {
				wasHover := a.borderHover
				a.borderHover = mouseMsg.X >= listW-1 && mouseMsg.X <= listW+1 && mouseMsg.Y >= filterBarH
				// Start fade-in or fade-out animation on hover change
				if a.borderHover != wasHover && !a.borderFading {
					a.borderFading = true
					return a, tea.Tick(30*time.Millisecond, func(t time.Time) tea.Msg {
						return borderFadeTickMsg{}
					})
				}
			}

			// Handle border drag
			if mouseMsg.Action == tea.MouseActionPress && mouseMsg.Button == tea.MouseButtonLeft {
				if mouseMsg.X >= listW-1 && mouseMsg.X <= listW+1 && mouseMsg.Y >= filterBarH {
					a.dragging = true
					return a, nil
				}
			}
			if mouseMsg.Action == tea.MouseActionRelease {
				a.dragging = false
				a.borderHover = false
				if mouseMsg.Button != tea.MouseButtonWheelDown && mouseMsg.Button != tea.MouseButtonWheelUp {
					return a, nil
				}
			}
			if a.dragging && mouseMsg.Action == tea.MouseActionMotion {
				newWidth := mouseMsg.X
				if newWidth < 20 {
					newWidth = 20
				}
				maxListW := a.usableWidth() - minDetailWidth
				if maxListW < 20 {
					maxListW = 20
				}
				if newWidth > maxListW {
					newWidth = maxListW
				}
				a.listWidth = newWidth
				if a.detail != nil {
					detailWidth := a.detailPaneWidth()
					d := *a.detail
					d.width = detailWidth
					a.detail = &d
				}
				return a, nil
			}

			// Route to list or detail pane with Y-adjusted coordinates
			if contentMouseMsg.X < listW {
				var cmd tea.Cmd
				a.list, cmd = a.list.Update(contentMouseMsg)
				return a, cmd
			}
			if a.detail != nil {
				adjusted := contentMouseMsg
				adjusted.X = contentMouseMsg.X - listW - 1
				d := *a.detail
				var cmd tea.Cmd
				d, cmd = d.Update(adjusted)
				a.detail = &d
				return a, cmd
			}
			return a, nil
		}
		var cmd tea.Cmd
		a.list, cmd = a.list.Update(msg)
		return a, cmd
	}

	return a, nil
}

// View renders the full app.
func (a App) View() string {
	// Guard: don't render until we know terminal dimensions
	if a.width == 0 || a.height == 0 {
		return ""
	}

	// Screen-level elements with fixed dimensions
	var filterBarView string
	filterBarH := 0
	if a.err == nil {
		filterBarView = a.filterBar.View()
		filterBarH = a.filterBar.Height()
	}

	contentH := a.height - 1 - filterBarH // help bar at bottom + filter bar at top
	if contentH < 1 {
		contentH = 1
	}

	// Main content — always split layout
	var content string
	if a.err != nil {
		errStyle := lipgloss.NewStyle().
			Foreground(colorError).
			PaddingLeft(2).
			PaddingTop(1)
		content = errStyle.Render("Error: " + a.err.Error())
	} else {
		listW := a.listWidth
		detailW := a.detailPaneWidth()

		// Left pane: list (or loading placeholder)
		var left string
		if a.state == stateLoading {
			loadStyle := lipgloss.NewStyle().
				Width(listW).
				Height(contentH).
				Foreground(colorText).
				Align(lipgloss.Center, lipgloss.Center)
			left = loadStyle.Render(a.spinner.View() + " Loading...")
		} else {
			listView := a.list.ViewWithWidth(listW, contentH)
			left = listView
		}

		// Border — animated highlight when hovering or dragging
		borderLines := make([]string, contentH)
		var borderStyle lipgloss.Style
		borderChar := "│"
		if a.borderFade > 0.01 {
			shade := int(a.borderFade * 5)
			shades := []lipgloss.Color{"#3a3a4a", "#4a4a6a", "#5a5a8a", "#7a7aaa", "#8a8acc"}
			if shade >= len(shades) {
				shade = len(shades) - 1
			}
			borderStyle = lipgloss.NewStyle().Foreground(shades[shade])
		} else {
			borderStyle = lipgloss.NewStyle().Foreground(colorBorder)
		}
		for i := range borderLines {
			borderLines[i] = borderStyle.Render(borderChar)
		}
		border := strings.Join(borderLines, "\n")

		// Right pane: detail
		var right string
		if a.state == stateLoading || a.detailLoading {
			loadStyle := lipgloss.NewStyle().
				Width(detailW).
				Height(contentH).
				Foreground(colorText).
				Align(lipgloss.Center, lipgloss.Center)
			msg := " Loading..."
			if a.detailLoading {
				msg = a.spinner.View() + " Loading issue..."
			}
			right = loadStyle.Render(msg)
		} else if a.detail != nil {
			right = a.detail.View()
		} else {
			emptyStyle := lipgloss.NewStyle().
				Width(detailW).
				Height(contentH).
				Foreground(colorSubtle).
				Align(lipgloss.Center, lipgloss.Center)
			right = emptyStyle.Render("No issues found")
		}

		content = lipgloss.JoinHorizontal(lipgloss.Top, left, border, right)
	}

	// Build all lines, hard-cap to exact dimensions
	contentLines := strings.Split(content, "\n")
	if len(contentLines) > contentH {
		contentLines = contentLines[:contentH]
	}
	for len(contentLines) < contentH {
		contentLines = append(contentLines, "")
	}

	allLines := make([]string, 0, a.height)

	if a.showHelp {
		// Full-screen help view
		allLines = strings.Split(a.renderHelpScreen(), "\n")
	} else {
		// Composite any bottom-right overlay (project or profile dropdown)
		var bottomOverlay []string
		if ov := a.projectDrop.RenderStandaloneOverlay(); ov != nil {
			bottomOverlay = ov
		} else if ov := a.profileDrop.RenderStandaloneOverlay(); ov != nil {
			bottomOverlay = ov
		}
		if bottomOverlay != nil {
			overlayH := len(bottomOverlay)
			startLine := len(contentLines) - overlayH
			if startLine < 0 {
				startLine = 0
			}
			overlayW := lipgloss.Width(bottomOverlay[0])
			xOff := a.usableWidth() - overlayW
			if xOff < 0 {
				xOff = 0
			}
			for i, oLine := range bottomOverlay {
				idx := startLine + i
				if idx < len(contentLines) {
					existing := contentLines[idx]
					existVis := lipgloss.Width(existing)
					if existVis > xOff {
						existing = truncateAnsi(existing, xOff)
					} else {
						existing += strings.Repeat(" ", xOff-existVis)
					}
					contentLines[idx] = existing + oLine
				}
			}
		}
		// Assemble: filterBar at top, then content, then help bar
		if filterBarH > 0 {
			allLines = append(allLines, strings.Split(filterBarView, "\n")...)
		}
		allLines = append(allLines, contentLines...)
		allLines = append(allLines, a.renderHelpBar())

		// Composite dropdown overlays onto allLines using absolute positions.
		// Preserve content to the left and right of the overlay.
		compositeOverlay := func(overlayLines []string, startLine, startCol int) {
			for i, oLine := range overlayLines {
				idx := startLine + i
				if idx >= 0 && idx < len(allLines) {
					original := allLines[idx]
					origVis := lipgloss.Width(original)
					overlayW := lipgloss.Width(oLine)

					var left string
					if origVis > startCol {
						left = truncateAnsi(original, startCol)
					} else {
						left = original + strings.Repeat(" ", startCol-origVis)
					}

					right := ""
					if origVis > startCol+overlayW {
						right = skipAnsi(original, startCol+overlayW)
					}

					allLines[idx] = left + oLine + right
				}
			}
		}

		// Filter bar overlays
		if a.filterBar.IsExpanded() && filterBarH > 0 {
			overlayLines, startLine, startCol := a.filterBar.OverlayLines()
			compositeOverlay(overlayLines, startLine, startCol)
		}

	}

	// Truncate every line to terminal width and cap total lines
	if len(allLines) > a.height {
		allLines = allLines[:a.height]
	}
	for i, line := range allLines {
		if lipgloss.Width(line) > a.usableWidth() {
			allLines[i] = truncateAnsi(line, a.usableWidth())
		}
	}

	return strings.Join(allLines, "\n")
}


// renderHelpScreen renders a full-screen centered help view.
func (a App) renderHelpScreen() string {
	w := a.usableWidth()

	keyStyle := lipgloss.NewStyle().Foreground(colorAccent).Width(16)
	descStyle := lipgloss.NewStyle().Foreground(colorText)
	sectionStyle := lipgloss.NewStyle().Foreground(colorWarning).Bold(true)
	subtleStyle := lipgloss.NewStyle().Foreground(colorSubtle)

	var box strings.Builder

	section := func(name string) {
		box.WriteString("\n")
		box.WriteString("  " + sectionStyle.Render(name) + "\n")
	}
	entry := func(key, desc string) {
		box.WriteString("  " + keyStyle.Render(key) + descStyle.Render(desc) + "\n")
	}

	box.WriteString("\n")
	box.WriteString("  " + lipgloss.NewStyle().Foreground(colorText).Bold(true).Render("Keyboard Shortcuts") + "\n")

	section("Navigation")
	entry("j / ↓", "Move down")
	entry("k / ↑", "Move up")
	entry("PgDn / PgUp", "Page down / up")

	section("Detail Tabs")
	entry("1-5", "Switch tab")

	section("Filters")
	entry("f", "Toggle filter bar")
	entry("t / s / a / l", "Type / Status / Assignee / Labels")
	entry("d / D", "Created from / until")
	entry("/", "Search")
	entry("x", "Clear all filters")

	section("Actions")
	entry("o", "Open in browser")
	entry("r", "Refresh issues")
	entry("q", "Quit")
	entry("?", "Toggle this help")

	section("Mouse")
	entry("Click", "Select issue")
	entry("Scroll", "Navigate list")
	entry("Drag border", "Resize panes")
	entry("Drag header", "Resize columns")

	box.WriteString("\n")
	box.WriteString("  " + subtleStyle.Render("Press any key to close") + "\n")

	// Render as fixed-width left-aligned block, then center manually
	boxW := 44
	content := lipgloss.NewStyle().Width(boxW).Render(box.String())

	// Center horizontally with left padding
	padLeft := (w - boxW) / 2
	if padLeft < 0 {
		padLeft = 0
	}

	// Center vertically
	contentLines := strings.Split(content, "\n")
	padTop := (a.height - len(contentLines)) / 2
	if padTop < 0 {
		padTop = 0
	}

	var out strings.Builder
	for i := 0; i < padTop; i++ {
		out.WriteString("\n")
	}
	for _, line := range contentLines {
		out.WriteString(strings.Repeat(" ", padLeft) + line + "\n")
	}

	return out.String()
}

func (a App) renderHelpBar() string {
	bgStyle := lipgloss.NewStyle().Background(colorHeaderBg)
	helpStyle := bgStyle.Foreground(colorSubtle).PaddingLeft(1)
	profileStyle := bgStyle.Foreground(colorSuccess).PaddingRight(1)

	var help string
	if a.filterBar.IsExpanded() && (a.filterBar.createdFrom.IsOpen() || a.filterBar.createdUntil.IsOpen()) {
		help = "←→↑↓ navigate · enter select · t today · backspace clear · esc close"
	} else if a.filterBar.IsExpanded() && a.filterBar.ActiveDropdown() {
		help = "↑↓ navigate · enter/space toggle · esc close"
	} else if a.filterBar.IsExpanded() {
		help = "t type · s status · p priority · a assignee · l labels · d/D dates · / search · o sort · O dir · S save · x clear · esc"
	} else if a.projectDrop.IsOpen() || a.profileDrop.IsOpen() {
		help = "↑↓ navigate · enter select · esc close"
	} else if a.detail != nil && a.detail.Editing() {
		help = "esc close · enter confirm"
	} else {
		help = "f filters · p project · o browser · r refresh · q quit · ? help"
	}

	left := helpStyle.Render(help)

	projectLabel := a.projectName
	if a.projectKey != "" {
		projectLabel = a.projectKey
	}
	projectStyle := bgStyle.Foreground(colorInfo)
	right := projectStyle.Render(projectLabel+" ") + profileStyle.Render("● "+a.profileName)

	gap := a.usableWidth() - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 0 {
		gap = 0
	}
	spacer := bgStyle.Render(strings.Repeat(" ", gap))

	return left + spacer + right
}

// helpBarClickZones returns the X positions where the project label and profile label start.
func (a App) helpBarClickZones() (projectX, profileX int) {
	projectLabel := a.projectName
	if a.projectKey != "" {
		projectLabel = a.projectKey
	}
	profileLabel := "● " + a.profileName

	projectW := lipgloss.Width(projectLabel + " ")
	profileW := lipgloss.Width(profileLabel)

	rightW := projectW + profileW
	projectX = a.usableWidth() - rightW
	profileX = a.usableWidth() - profileW
	return
}

// Run starts the Bubble Tea program.
func Run(client *jira.Client, profileName, initialProject, configPath string, profileNames []string) error {
	app := NewApp(client, profileName, initialProject, configPath, profileNames)
	p := tea.NewProgram(app,
		tea.WithAltScreen(),
		tea.WithMouseAllMotion(),
	)
	_, err := p.Run()
	if err != nil {
		return fmt.Errorf("running TUI: %w", err)
	}
	return nil
}
