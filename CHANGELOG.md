
### ♻️ Refactor

- Remove old inline filter from List component

### 🐛 Bug Fixes

- Migrate to /rest/api/3/search/jql endpoint
- Migrate to cursor-based pagination for /search/jql endpoint
- Detail view scroll clamp and resize dimension fixes
- Extract colorSelection constant and fix resize in loading state
- Field overflow, help bar, and layout flash on startup
- Guarantee help bar by hard-capping content to exact height
- Truncate wide lines to prevent terminal scrollbar
- Truncate ALL lines including status/help bars to terminal width
- Reserve 1-col right margin to avoid terminal scrollbar overlap
- Replace help overlay with full-screen help view
- Center help screen as a block instead of per-line
- Center help screen as fixed-width block with manual padding
- Show Updated as datestamp matching Created format
- Use visual width for field padding (fixes em dash/unicode alignment)
- Preserve sort indicators after re-fetching issues

### 📚 Documentation

- Add filter bar design spec
- Add filter bar implementation plan

### 🔧 Miscellaneous Tasks

- Remove OAuth scopes note from auth flow
- Add bubbletea, lipgloss, bubbles, browser deps for TUI
- Add release workflow with homebrew tap integration

### 🚀 Features

- Scaffold project with cobra root command
- Add config package with load/save and profile management
- Add auth CLI commands (add, list, switch)
- Add first-run detection that triggers auth setup
- Add domain models for Issue, Comment, User, etc.
- Add Jira API response types for v3 REST API
- Add Jira HTTP client with auth and ADF text extraction
- Add issue search and detail fetching with domain model mapping
- Add issues CLI command for testing API integration
- Obfuscate API token input and verify credentials on auth add
- Default Jira URL from profile name
- Handle duplicate profile names in auth add
- Show asterisks while typing API token
- Use lock icon for API token prompt
- Add icons to all auth prompts
- Exclude completed issues from default search
- Add Tokyo Night theme colors and priority/status styling
- Add list view key bindings
- Add list model with table, filter, and row rendering
- Add root app model with loading spinner, status bar, and help bar
- Launch TUI as default command
- Custom table renderer with per-row urgency colors
- Add detail view with 5 tabs and enter/esc navigation
- Responsive split layout for detail view (wide/narrow)
- Detail view polish — tab clicks, scroll clamping, key fixes
- Always-split layout, mouse routing, and page up/down
- Always-visible detail pane, tab underline, attachments tab
- Tab underline overlaps divider, draggable border, fix help bar
- Form-field Details tab, fix tab clicks, merge description
- Remove title bar, simplify help bar, fix scrollbar
- Adjustable columns, description fills height, fix clicks
- Add ? help overlay with keyboard shortcuts
- Add visible column divider between Key and Summary
- Auto-size Key column to widest issue key
- Scroll within description box only, not the whole details page
- Cap description scroll to content length
- Click column headers to sort by Key or Summary
- Detail view with editable fields, dropdowns, date picker
- Reorganize layout and add parent search dropdown
- Add comment CRUD API methods and markdown-to-ADF converter
- Interactive comments tab with add/edit/reply/delete UI
- Add issue link CRUD API methods and LinkType model
- Merge Subtasks+Links into Associations tab with link CRUD UI
- Add GetProjectStatusesAndTypes API method
- Add MultiSelect component with toggle checkmarks
- Add FilterBar component with JQL builder
- Integrate FilterBar into App with JQL-based filtering
- Filters, sort, description editing, inline media, and UX improvements
- Saved views system with Default view, compact action buttons, and unified query path

### 🧪 Testing

- Add profile operations tests

