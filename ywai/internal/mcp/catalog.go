package mcp

// catalog.go — the canonical list of MCP servers ywai knows how to install.
//
// Each CatalogEntry pins what the install UI needs to render a row and
// what the runtime needs to launch the server. The 12 entries here are
// the ones settled in slice 1 of the "Real MCP Install" plan (see
// memory #192 in the ywai project). The list is closed: kubernetes and
// sharptools were removed. To add or remove a server, edit catalog.

// CatalogEntry describes a single MCP server ywai can install.
//
//   - ID, Name, Description, Category, Icon, Popular drive the install UI row.
//   - Type is "local" (stdio subprocess) or "remote" (HTTP endpoint).
//   - For local entries: Command is the argv the runtime spawns, and
//     InstallCmd is the human-readable install line. URL is empty.
//   - For remote entries: URL is the HTTP(S) endpoint, Command is empty,
//     and InstallCmd is empty (there is nothing to install).
//   - RequiredEnv lists credentials / connection strings the install UI
//     must collect. Secret=true entries are redacted from log output by
//     RedactMessage.
//   - Tools is a scout-estimated list of tool names. The runtime re-probes
//     via DiscoverStdio / DiscoverHTTP and replaces this with the real one;
//     Tools is the fallback for offline / pre-install display.
//   - Docs points at the upstream project page.
type CatalogEntry struct {
	ID          string
	Name        string
	Description string
	Category    string
	Icon        string
	Popular     bool
	Type        string
	Command     []string
	URL         string
	InstallCmd  string
	RequiredEnv []EnvSpec
	Tools       []string
	Docs        string

	// OAuth fields for remote servers that need authentication.
	// AuthType is "oauth" when OAuth is required; empty otherwise.
<<<<<<< Updated upstream
	AuthType         string
	ClientID         string
	ClientSecret     string
	Scopes           []string
	AuthorizationURL string
	TokenURL         string
=======
	AuthType        string
	ClientID        string
	ClientSecret    string
	Scopes          []string
	AuthorizationURL string
	TokenURL        string
>>>>>>> Stashed changes
}

// catalog is the package-private backing slice. Callers must not mutate it.
var catalog = []CatalogEntry{
	{
		ID: "context7", Name: "Context7",
		Description: "Up-to-date library documentation and code context",
		Category:    "docs", Icon: "📚", Popular: true,
		Type: "remote", URL: "https://mcp.context7.com/mcp",
		Tools: []string{"get-library-docs", "resolve-library-id", "search"},
		Docs:  "https://context7.com",
	},
	{
		ID: "microsoft-learn", Name: "Microsoft Learn",
		Description: "Microsoft official documentation, API refs, and code samples",
		Category:    "docs", Icon: "📘", Popular: true,
		Type: "remote", URL: "https://learn.microsoft.com/api/mcp",
		Tools: []string{"microsoft_docs_search", "microsoft_docs_fetch", "microsoft_code_sample_search"},
		Docs:  "https://learn.microsoft.com",
	},
	{
		ID: "jam", Name: "Jam",
		Description: "Capture browser bugs, console errors, and network requests",
		Category:    "testing", Icon: "🐛",
		Type: "remote", URL: "https://mcp.jam.dev/mcp",
		Tools: []string{"get_bug", "list_bugs", "create_bug", "search_bugs"},
		Docs:  "https://jam.dev",
	},
	{
		ID: "chrome-devtools", Name: "Chrome DevTools",
		Description: "Drive a real Chrome browser: navigate, click, screenshot, evaluate",
		Category:    "testing", Icon: "🧪", Popular: true,
		Type: "local", Command: []string{"npx", "-y", "@anthropic-ai/chrome-devtools-mcp"},
		InstallCmd: "npx -y @anthropic-ai/chrome-devtools-mcp",
		Tools:      []string{"navigate", "screenshot", "click", "evaluate"},
		Docs:       "https://github.com/anthropics/chrome-devtools-mcp",
	},
	{
		ID: "playwright", Name: "Playwright",
		Description: "Cross-browser end-to-end testing via Playwright",
		Category:    "testing", Icon: "🎭", Popular: true,
		Type: "local", Command: []string{"npx", "-y", "@playwright/mcp@latest"},
		InstallCmd: "npx -y @playwright/mcp@latest",
		Tools:      []string{"browser_navigate", "browser_snapshot", "browser_click", "browser_screenshot"},
		Docs:       "https://github.com/microsoft/playwright-mcp",
	},
	{
		ID: "git", Name: "Git",
		Description: "Read and inspect local git repositories",
		Category:    "vcs", Icon: "🔧",
		Type: "local", Command: []string{"npx", "-y", "@modelcontextprotocol/server-git"},
		InstallCmd: "npx -y @modelcontextprotocol/server-git",
		Tools:      []string{"git_status", "git_log", "git_diff", "git_show"},
		Docs:       "https://github.com/modelcontextprotocol/servers",
	},
	{
		ID: "github", Name: "GitHub",
		Description: "Read and write GitHub repos, issues, and PRs",
		Category:    "vcs", Icon: "🐙", Popular: true,
		Type: "local", Command: []string{"npx", "-y", "@modelcontextprotocol/server-github"},
		InstallCmd: "npx -y @modelcontextprotocol/server-github",
		RequiredEnv: []EnvSpec{{
			Name:        "GITHUB_PERSONAL_ACCESS_TOKEN",
			Description: "Personal access token with repo, read:user, and read:org scopes",
			Required:    true,
			Secret:      true,
		}},
		Tools: []string{"create_or_update_file", "search_repositories", "create_issue", "list_issues", "get_file_contents"},
		Docs:  "https://github.com/modelcontextprotocol/servers",
	},
	{
		ID: "postgres", Name: "PostgreSQL",
		Description: "Query and inspect PostgreSQL databases",
		Category:    "database", Icon: "🐘",
		Type: "local", Command: []string{"npx", "-y", "@modelcontextprotocol/server-postgres"},
		InstallCmd: "npx -y @modelcontextprotocol/server-postgres",
		RequiredEnv: []EnvSpec{{
			Name:        "DATABASE_URL",
			Description: "PostgreSQL connection string, e.g. postgres://user:pass@host:5432/db",
			Required:    true,
			Secret:      true,
		}},
		Tools: []string{"query", "list_tables", "describe_table", "list_schemas"},
		Docs:  "https://github.com/modelcontextprotocol/servers",
	},
	{
		ID: "docker", Name: "Docker",
		Description: "Manage Docker containers, images, and networks",
		Category:    "devops", Icon: "🐳",
		Type: "local", Command: []string{"npx", "-y", "@modelcontextprotocol/server-docker"},
		InstallCmd: "npx -y @modelcontextprotocol/server-docker",
		Tools:      []string{"list_containers", "list_images", "create_container", "start_container"},
		Docs:       "https://github.com/modelcontextprotocol/servers",
	},
	{
		ID: "engram", Name: "Engram",
		Description: "Persistent memory for AI sessions: save, search, and recall past decisions",
		Category:    "memory", Icon: "🧠", Popular: true,
		Type: "local", Command: []string{"engram", "mcp"},
		InstallCmd: "go install github.com/nahuelyoizen/engram/cmd/engram@latest",
		Tools:      []string{"mem_search", "mem_save", "mem_get", "mem_context"},
		Docs:       "https://github.com/nahuelyoizen/engram",
	},
	{
		ID: "codegraph", Name: "CodeGraph",
		Description: "Semantic code search and dependency context across the repo",
		Category:    "memory", Icon: "🕸️",
		Type: "local", Command: []string{"codegraph", "serve", "--mcp"},
		InstallCmd: "npm i -g @colbymchenry/codegraph",
		Tools:      []string{"codegraph_search", "codegraph_context", "codegraph_dependencies"},
		Docs:       "https://github.com/colbymchenry/codegraph",
	},
	{
		ID: "ywai-kanban", Name: "ywai Kanban",
		Description: "Track ywai dev tasks on a local kanban board (uses your ywai binary)",
		Category:    "productivity", Icon: "🗂️",
		Type: "local", Command: []string{"ywai", "serve", "--mcp-only"},
		Tools: []string{"create_task", "list_boards", "move_card", "add_comment"},
		Docs:  "https://github.com/Yoizen/dev-ai-workflow",
	},
}

// Catalog returns the canonical list of MCP servers ywai can install.
// Order matches the catalog var.
func Catalog() []CatalogEntry { return catalog }

// CatalogByID looks up a single catalog entry by its ID. The second
// return is true when the ID is known, false otherwise (in which case
// the returned entry is the zero-value CatalogEntry). Linear scan is
// fine: the catalog is closed at 12 entries.
func CatalogByID(id string) (CatalogEntry, bool) {
	for _, e := range catalog {
		if e.ID == id {
			return e, true
		}
	}
	return CatalogEntry{}, false
}
