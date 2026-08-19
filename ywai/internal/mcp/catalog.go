package mcp

// catalog.go — the canonical list of MCP servers ywai knows how to install.
//
// Each CatalogEntry pins what the install UI needs to render a row and
// what the runtime needs to launch the server. Edit this slice to add
// or remove servers; catalog_test.go pins IDs and length.

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

	// DefaultDisabled: OpenCode v2 treats a missing enabled flag as on.
	// Set this so install writes enabled:false instead of omitting it.
	DefaultDisabled bool

	// OAuth fields for remote servers that need authentication.
	// AuthType is "oauth" when OAuth is required; empty otherwise.
	AuthType         string
	ClientID         string
	ClientSecret     string
	Scopes           []string
	AuthorizationURL string
	TokenURL         string
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
		InstallCmd: "go install github.com/Gentleman-Programming/engram/cmd/engram@latest",
		Tools:      []string{"mem_search", "mem_save", "mem_get", "mem_context"},
		Docs:       "https://github.com/Gentleman-Programming/engram",
	},
	{
		ID: "graft", Name: "Graft",
		Description: "Repo context graph: symbol lookup, call tracing, and file API over the indexed codebase",
		Category:    "memory", Icon: "🕸️",
		Type: "local", Command: []string{"graft", "mcp"},
		InstallCmd: "npm i -g @nanonets/graft",
		Tools:      []string{"graft_find_code", "graft_trace_calls", "graft_find_all", "graft_file_api", "graft_repo_map", "graft_check_freshness"},
		Docs:       "https://github.com/nanonets/graft",
	},
	{
		ID: "filesystem", Name: "Filesystem",
		Description: "Read and write files under allowed directories (defaults to current workspace)",
		Category:    "core", Icon: "📁", Popular: true,
		// "." is resolved relative to the agent process cwd (usually the project root).
		Type: "local", Command: []string{"npx", "-y", "@modelcontextprotocol/server-filesystem", "."},
		InstallCmd: "npx -y @modelcontextprotocol/server-filesystem",
		Tools:      []string{"read_file", "write_file", "list_directory", "search_files", "get_file_info"},
		Docs:       "https://github.com/modelcontextprotocol/servers/tree/main/src/filesystem",
	},
	{
		ID: "brave-search", Name: "Brave Search",
		Description: "Web search via Brave Search API",
		Category:    "search", Icon: "🔍", Popular: true,
		Type: "local", Command: []string{"npx", "-y", "@modelcontextprotocol/server-brave-search"},
		InstallCmd: "npx -y @modelcontextprotocol/server-brave-search",
		RequiredEnv: []EnvSpec{{
			Name:        "BRAVE_API_KEY",
			Description: "API key from https://brave.com/search/api/",
			Required:    true,
			Secret:      true,
		}},
		Tools: []string{"brave_web_search", "brave_local_search"},
		Docs:  "https://github.com/modelcontextprotocol/servers/tree/main/src/brave-search",
	},
	{
		ID: "mysql", Name: "MySQL",
		Description: "Query MySQL databases (read-oriented MCP server)",
		Category:    "database", Icon: "🐬", Popular: true,
		Type: "local", Command: []string{"npx", "-y", "mcp-server-mysql@latest"},
		InstallCmd: "npx -y mcp-server-mysql@latest",
		RequiredEnv: []EnvSpec{
			{
				Name:        "MYSQL_HOST",
				Description: "MySQL host (e.g. 127.0.0.1)",
				Required:    true,
				Secret:      false,
			},
			{
				Name:        "MYSQL_PORT",
				Description: "MySQL port (default 3306)",
				Required:    false,
				Secret:      false,
			},
			{
				Name:        "MYSQL_USER",
				Description: "MySQL username",
				Required:    true,
				Secret:      false,
			},
			{
				Name:        "MYSQL_PASS",
				Description: "MySQL password",
				Required:    true,
				Secret:      true,
			},
			{
				Name:        "MYSQL_DB",
				Description: "Database name",
				Required:    true,
				Secret:      false,
			},
		},
		Tools: []string{"mysql_query", "list_tables", "describe_table"},
		Docs:  "https://www.npmjs.com/package/mcp-server-mysql",
	},
	{
		ID: "puppeteer", Name: "Puppeteer",
		Description: "Browser automation with Puppeteer (navigate, screenshot, click)",
		Category:    "testing", Icon: "🐶",
		Type: "local", Command: []string{"npx", "-y", "@modelcontextprotocol/server-puppeteer"},
		InstallCmd: "npx -y @modelcontextprotocol/server-puppeteer",
		Tools:      []string{"puppeteer_navigate", "puppeteer_screenshot", "puppeteer_click", "puppeteer_fill", "puppeteer_evaluate"},
		Docs:       "https://github.com/modelcontextprotocol/servers/tree/main/src/puppeteer",
	},
	{
		ID: "codemod", Name: "Codemod",
		Description: "Large-scale code migrations and AST-based codemods via the Codemod CLI",
		Category:    "devtools", Icon: "♻️",
		DefaultDisabled: true,
		Type:            "local", Command: []string{"npx", "-y", "codemod", "mcp"},
		InstallCmd: "npm i -g codemod",
		Tools:      []string{"codemod_run", "codemod_search", "codemod_status"},
		Docs:       "https://docs.codemod.com",
	},
}

// Catalog returns the canonical list of MCP servers ywai can install.
// Order matches the catalog var.
func Catalog() []CatalogEntry { return catalog }

// CatalogByID looks up a single catalog entry by its ID. The second
// return is true when the ID is known, false otherwise (in which case
// the returned entry is the zero-value CatalogEntry).
func CatalogByID(id string) (CatalogEntry, bool) {
	for _, e := range catalog {
		if e.ID == id {
			return e, true
		}
	}
	return CatalogEntry{}, false
}
