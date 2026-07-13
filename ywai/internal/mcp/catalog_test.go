package mcp

import (
	"slices"
	"strings"
	"testing"
)

// catalog_test.go — pins the contract of the catalog package from
// ywai/internal/mcp/catalog.go (TDD slice 2 of the "Real MCP Install"
// plan). The file under test will define a CatalogEntry struct and two
// functions: Catalog() returning the full list, and CatalogByID(id)
// returning a single entry plus an "ok" bool. These tests are RED right
// now: the file does not exist yet, so the test binary will not compile
// until @dev implements it. The expected compile error is:
//   undefined: CatalogEntry
//   undefined: Catalog
//   undefined: CatalogByID
//
// The 12 final catalog entries were decided by the user during slice 1
// of this TDD flow (see memory #192 in the ywai project). The IDs are:
//
//   1. context7         (remote, no install, no env)
//   2. microsoft-learn  (remote, no install, no env)
//   3. jam              (remote, no install, no env)
//   4. chrome-devtools  (local, npx command, no env)
//   5. playwright       (local, npx command, no env)
//   6. git              (local, npx command, no env)
//   7. github           (local, npx command, GITHUB_PERSONAL_ACCESS_TOKEN*)
//   8. postgres         (local, npx command, DATABASE_URL*)
//   9. docker           (local, npx command, no env)
//   10. engram          (local, go install, no env)
//   11. codegraph       (local, go install, no env)
//   12. ywai-kanban     (local, ywai serve --mcp-only, no install, no env)
//
// `*` = required and secret env var.
//
// Assumptions baked into the tests (anything not pinned here is left to
// @dev to decide without being locked down):
//
//   - CatalogEntry is the struct shape documented in the slice 2 brief.
//     These tests only assert on the fields they need: ID, Name, Type,
//     Command, URL, InstallCmd, RequiredEnv. Description, Category,
//     Icon, Popular, Tools, and Docs are NOT asserted on here (per the
//     slice 2 constraint to not pin unverified shape details).
//   - Catalog() returns a slice of length 12 (post-decisions; sharptools
//     and kubernetes were removed).
//   - CatalogByID uses idiomatic second-return-`ok` style: an unknown id
//     returns a zero-value CatalogEntry and false.
//   - For remote entries: Type=="remote", URL!="", Command is nil or
//     empty, InstallCmd=="".
//   - For local entries: Type=="local", Command has >= 1 element,
//     URL=="", InstallCmd!="". Local entry ywai-kanban is the one
//     exception: InstallCmd=="" because the binary is the ywai binary
//     itself, already on PATH.
//   - Required env vars that hold tokens / connection strings must be
//     marked Required=true and Secret=true. A non-empty Description
//     is pinned for github's GITHUB_PERSONAL_ACCESS_TOKEN (the user
//     wants a human-readable label in the install UI).
//   - The exact tool list per entry is NOT pinned in this file. Tools
//     are scout-estimated "3-5" or "5+" and will be verified in a
//     later slice via the discovery probe. The tests here only assert
//     the entries that DO have a known tool list (context7,
//     microsoft-learn, jam) are present and lookable, not what tools
//     they expose.
//
// Test ordering: each TestXxx is independent. Subtests / shared state
// are not used — failures from one test cannot mask or contaminate
// another.

// Compile-time pin: the CatalogEntry type must be in scope in this
// package. The tests below use type inference for `entry, ok :=
// CatalogByID(...)`, so without this line a missing CatalogEntry would
// only show up transitively. This declaration makes the "undefined:
// CatalogEntry" error explicit in the test build, so a future
// implementer cannot accidentally rename or remove the type without
// this file's compile error pointing at the right symbol.
var _ = CatalogEntry{}

// ─── Catalog() — size & membership ────────────────────────────────────────

// TestCatalog_Len pins the size of the catalog. The user removed
// sharptools and kubernetes from the scout's 14-entry draft, leaving
// exactly 12 entries. If a future addition sneaks in, this test fails.
func TestCatalog_Len(t *testing.T) {
	got := len(Catalog())
	if got != 13 {
		t.Errorf("len(Catalog()) = %d, want 13", got)
	}
}

// TestCatalog_ContainsAllExpectedIDs pins that every one of the 12 final
// IDs is present and lookable via CatalogByID. The order of entries in
// the underlying slice is intentionally NOT pinned — each ID is found
// by lookup, not by position.
func TestCatalog_ContainsAllExpectedIDs(t *testing.T) {
	want := []string{
		"context7",
		"microsoft-learn",
		"jam",
		"chrome-devtools",
		"playwright",
		"git",
		"github",
		"postgres",
		"docker",
		"engram",
		"codegraph",
		"ywai-kanban",
		"ywai-fastfs",
	}
	for _, id := range want {
		entry, ok := CatalogByID(id)
		if !ok {
			t.Errorf("CatalogByID(%q) ok=false, want true", id)
			continue
		}
		if entry.ID != id {
			t.Errorf("CatalogByID(%q) entry.ID = %q, want %q",
				id, entry.ID, id)
		}
	}
}

// TestCatalog_ExcludesRemovedIDs pins the removal decision from
// slice 1: sharptools and kubernetes were dropped from the final 12.
// If they come back accidentally in a future merge, this test catches
// it before the install UI starts showing them again.
func TestCatalog_ExcludesRemovedIDs(t *testing.T) {
	removed := []string{"sharptools", "kubernetes"}
	for _, id := range removed {
		_, ok := CatalogByID(id)
		if ok {
			t.Errorf("CatalogByID(%q) ok=true, want false (removed from final catalog)",
				id)
		}
	}
}

// ─── Catalog() — per-entry invariants ─────────────────────────────────────

// TestCatalog_AllEntriesHaveID pins that no entry is missing an ID. An
// empty ID would make CatalogByID misroute lookups and would render
// the install UI as a blank row.
func TestCatalog_AllEntriesHaveID(t *testing.T) {
	for i, e := range Catalog() {
		if e.ID == "" {
			t.Errorf("Catalog()[%d].ID is empty", i)
		}
	}
}

// TestCatalog_AllEntriesHaveName pins that no entry is missing a human
// name. An empty Name would render the install UI without a label —
// useless even if the entry otherwise works.
func TestCatalog_AllEntriesHaveName(t *testing.T) {
	for i, e := range Catalog() {
		if e.Name == "" {
			t.Errorf("Catalog()[%d].Name is empty (id=%q)", i, e.ID)
		}
	}
}

// TestCatalog_AllLocalHaveCommand pins that every local entry has a
// non-empty Command slice. A local entry without a command cannot be
// launched by the stdio transport (DiscoverStdio refuses empty
// commands, see discovery.go).
func TestCatalog_AllLocalHaveCommand(t *testing.T) {
	for _, e := range Catalog() {
		if e.Type == "local" && len(e.Command) == 0 {
			t.Errorf("entry %q: Type=local but len(Command)==0", e.ID)
		}
	}
}

// TestCatalog_AllRemoteHaveURL pins that every remote entry has a
// non-empty URL. A remote entry without a URL has no endpoint to POST
// the tools/list probe to — DiscoverHTTP would have nothing to hit.
func TestCatalog_AllRemoteHaveURL(t *testing.T) {
	for _, e := range Catalog() {
		if e.Type == "remote" && e.URL == "" {
			t.Errorf("entry %q: Type=remote but URL is empty", e.ID)
		}
	}
}

// TestCatalog_UniqueIDs pins that the 12 IDs are unique. A duplicate
// would make CatalogByID return whichever entry happens to be first
// in the slice, silently masking a bug in the catalog definition (the
// second entry would be unreachable).
func TestCatalog_UniqueIDs(t *testing.T) {
	seen := make(map[string]int, len(Catalog()))
	for _, e := range Catalog() {
		if prev, dup := seen[e.ID]; dup {
			t.Errorf("duplicate ID %q: appears at index %d and again later",
				e.ID, prev)
			continue
		}
		seen[e.ID]++
	}
}

// ─── CatalogByID — lookup ─────────────────────────────────────────────────

// TestCatalogByID_Unknown pins the not-found path. An unknown ID must
// return a zero-value CatalogEntry and false. The zero-value check on
// ID, Name, Type, and URL is sufficient: a real entry would populate
// at least one of those, so seeing all four empty means the
// implementer did the right thing.
func TestCatalogByID_Unknown(t *testing.T) {
	entry, ok := CatalogByID("nope-not-real")
	if ok {
		t.Errorf("CatalogByID(unknown) ok=true, want false")
	}
	if entry.ID != "" || entry.Name != "" || entry.Type != "" || entry.URL != "" {
		t.Errorf("CatalogByID(unknown) entry = %+v, want zero-value CatalogEntry",
			entry)
	}
}

// ─── CatalogByID — per-entry shape ────────────────────────────────────────

// TestCatalogByID_Remote_Context7 pins the remote shape using
// context7 as the canonical example. Remotes use URL, have no
// Command (the HTTP transport does not spawn a subprocess), and the
// install UI must skip the install step (InstallCmd == "").
func TestCatalogByID_Remote_Context7(t *testing.T) {
	entry, ok := CatalogByID("context7")
	if !ok {
		t.Fatal("CatalogByID(context7) ok=false, want true")
	}
	if entry.Type != "remote" {
		t.Errorf("context7.Type = %q, want \"remote\"", entry.Type)
	}
	if entry.URL != "https://mcp.context7.com/mcp" {
		t.Errorf("context7.URL = %q, want \"https://mcp.context7.com/mcp\"",
			entry.URL)
	}
	if len(entry.Command) != 0 {
		t.Errorf("context7.Command = %v, want nil or empty (remote has no subprocess)",
			entry.Command)
	}
	if entry.InstallCmd != "" {
		t.Errorf("context7.InstallCmd = %q, want \"\" (remote skips install)",
			entry.InstallCmd)
	}
}

// TestCatalogByID_LocalWithCommand_Playwright pins the local shape
// using playwright as the canonical npx example. Locals have a
// Command slice and a non-empty InstallCmd. The InstallCmd must
// mirror the command tokens (so a user copying the install line into
// a terminal gets the same invocation as the catalog's command).
func TestCatalogByID_LocalWithCommand_Playwright(t *testing.T) {
	entry, ok := CatalogByID("playwright")
	if !ok {
		t.Fatal("CatalogByID(playwright) ok=false, want true")
	}
	if entry.Type != "local" {
		t.Errorf("playwright.Type = %q, want \"local\"", entry.Type)
	}
	wantCmd := []string{"npx", "-y", "@playwright/mcp@latest"}
	if !slices.Equal(entry.Command, wantCmd) {
		t.Errorf("playwright.Command = %v, want %v", entry.Command, wantCmd)
	}
	if entry.URL != "" {
		t.Errorf("playwright.URL = %q, want \"\" (local has no URL)", entry.URL)
	}
	if entry.InstallCmd == "" {
		t.Errorf("playwright.InstallCmd is empty, want non-empty (local needs install)")
	}
	// InstallCmd should mirror the command as a string. We assert on
	// the two tokens a user would visually recognize — "npx" and
	// "playwright" — not the full string, so a future reordering
	// inside InstallCmd does not break this pin.
	for _, tok := range []string{"npx", "playwright"} {
		if !strings.Contains(entry.InstallCmd, tok) {
			t.Errorf("playwright.InstallCmd = %q, missing token %q",
				entry.InstallCmd, tok)
		}
	}
}

// TestCatalogByID_GoInstall_Engram pins the go-install path. The
// engram entry uses `go install` instead of npx; the InstallCmd must
// reflect that. This is the path that distinguishes the two CLI-style
// MCP servers (engram, codegraph) from the npm-style ones.
func TestCatalogByID_GoInstall_Engram(t *testing.T) {
	entry, ok := CatalogByID("engram")
	if !ok {
		t.Fatal("CatalogByID(engram) ok=false, want true")
	}
	if entry.Type != "local" {
		t.Errorf("engram.Type = %q, want \"local\"", entry.Type)
	}
	if !strings.Contains(entry.InstallCmd, "go install") {
		t.Errorf("engram.InstallCmd = %q, missing \"go install\"", entry.InstallCmd)
	}
	if !strings.Contains(entry.InstallCmd, "engram") {
		t.Errorf("engram.InstallCmd = %q, missing \"engram\" (package name)",
			entry.InstallCmd)
	}
}

// TestCatalogByID_RequiredEnv_GitHub pins the github entry's single
// required+secret env var: GITHUB_PERSONAL_ACCESS_TOKEN. The token
// is a secret, so Secret must be true; it's also required, so
// Required must be true. The Description must be non-empty so the
// install UI can show a human-readable label like "Personal access
// token from github.com/settings/tokens".
func TestCatalogByID_RequiredEnv_GitHub(t *testing.T) {
	entry, ok := CatalogByID("github")
	if !ok {
		t.Fatal("CatalogByID(github) ok=false, want true")
	}
	if len(entry.RequiredEnv) != 1 {
		t.Fatalf("github.RequiredEnv has %d entries, want 1: %+v",
			len(entry.RequiredEnv), entry.RequiredEnv)
	}
	spec := entry.RequiredEnv[0]
	if spec.Name != "GITHUB_PERSONAL_ACCESS_TOKEN" {
		t.Errorf("github.RequiredEnv[0].Name = %q, want GITHUB_PERSONAL_ACCESS_TOKEN",
			spec.Name)
	}
	if !spec.Required {
		t.Errorf("github.RequiredEnv[0].Required = false, want true")
	}
	if !spec.Secret {
		t.Errorf("github.RequiredEnv[0].Secret = false, want true (token is a secret)")
	}
	if spec.Description == "" {
		t.Errorf("github.RequiredEnv[0].Description is empty, want non-empty")
	}
}

// TestCatalogByID_RequiredEnv_Postgres pins the postgres entry's
// single required+secret env var: DATABASE_URL. The connection
// string contains a password in the URL, which is why Secret=true —
// the redactor (RedactMessage) keys off this flag to mask the
// password segment when logging the URL.
func TestCatalogByID_RequiredEnv_Postgres(t *testing.T) {
	entry, ok := CatalogByID("postgres")
	if !ok {
		t.Fatal("CatalogByID(postgres) ok=false, want true")
	}
	if len(entry.RequiredEnv) != 1 {
		t.Fatalf("postgres.RequiredEnv has %d entries, want 1: %+v",
			len(entry.RequiredEnv), entry.RequiredEnv)
	}
	spec := entry.RequiredEnv[0]
	if spec.Name != "DATABASE_URL" {
		t.Errorf("postgres.RequiredEnv[0].Name = %q, want DATABASE_URL",
			spec.Name)
	}
	if !spec.Required {
		t.Errorf("postgres.RequiredEnv[0].Required = false, want true")
	}
	if !spec.Secret {
		t.Errorf("postgres.RequiredEnv[0].Secret = false, want true (connection string is a secret)")
	}
}

// TestCatalogByID_NoRequiredEnv_Context7 pins that a remote entry
// with no env-var requirements has an empty (or nil) RequiredEnv.
// This is the canonical "no-credentials" case: the install UI should
// not even render the credentials form for context7.
func TestCatalogByID_NoRequiredEnv_Context7(t *testing.T) {
	entry, ok := CatalogByID("context7")
	if !ok {
		t.Fatal("CatalogByID(context7) ok=false, want true")
	}
	if len(entry.RequiredEnv) != 0 {
		t.Errorf("context7.RequiredEnv has %d entries, want 0: %+v",
			len(entry.RequiredEnv), entry.RequiredEnv)
	}
}

// TestCatalogByID_SkipInstall_Remote pins that every remote entry
// has an empty InstallCmd. The install UI must skip these — there is
// nothing to install; the server is reached over HTTP. If any remote
// entry accidentally gets an InstallCmd, the UI will show a
// misleading "install" button.
func TestCatalogByID_SkipInstall_Remote(t *testing.T) {
	remotes := []string{"context7", "microsoft-learn", "jam"}
	for _, id := range remotes {
		entry, ok := CatalogByID(id)
		if !ok {
			t.Errorf("CatalogByID(%q) ok=false, want true", id)
			continue
		}
		if entry.InstallCmd != "" {
			t.Errorf("%s.InstallCmd = %q, want \"\" (remote skips install)",
				id, entry.InstallCmd)
		}
	}
}

// TestCatalogByID_SkipInstall_YwaiKanban pins the special case:
// ywai-kanban is local, but its binary is the ywai binary itself
// (already on PATH after `ywai install`). The install UI must skip
// the install step. This is the only local entry in the 12 that
// ships with InstallCmd == "".
func TestCatalogByID_SkipInstall_YwaiKanban(t *testing.T) {
	entry, ok := CatalogByID("ywai-kanban")
	if !ok {
		t.Fatal("CatalogByID(ywai-kanban) ok=false, want true")
	}
	if entry.Type != "local" {
		t.Errorf("ywai-kanban.Type = %q, want \"local\"", entry.Type)
	}
	if entry.InstallCmd != "" {
		t.Errorf("ywai-kanban.InstallCmd = %q, want \"\" (binary is ywai itself)",
			entry.InstallCmd)
	}
}

// ─── Catalog() — full-catalog invariants (added in slice 2 VALIDATE) ──────

// TestCatalog_AllTypesValid pins the runtime-dispatch contract: every
// entry's Type must be exactly "local" or "remote". The runtime keys
// off this string to pick the stdio vs. HTTP transport in
// internal/mcp/credentials.go and discovery.go. A typo like "Local"
// (capitalized) or a made-up value like "stdio" or "http" would make
// the entry silently unreachable from the install UI — no per-entry
// test catches this because the existing tests only assert equality
// against the *expected* value, not that the value is one of the
// *allowed* values. This is the only place that pins the closed set.
func TestCatalog_AllTypesValid(t *testing.T) {
	for _, e := range Catalog() {
		if e.Type != "local" && e.Type != "remote" {
			t.Errorf("entry %q: Type = %q, want \"local\" or \"remote\" "+
				"(runtime dispatch reads this string verbatim)",
				e.ID, e.Type)
		}
	}
}

// TestCatalog_AllLocalNonKanbanHaveInstallCmd pins the documented
// invariant: every local entry has a non-empty InstallCmd, with the
// single exception of ywai-kanban (whose binary IS the ywai binary,
// already on PATH). The per-entry tests pin specific values, but
// nothing asserts this as a *class invariant* — so a future entry
// that ships with Type=="local" and InstallCmd=="" by accident
// (e.g. someone copying ywai-kanban's shape) would render the
// install UI as a row that has no install step and no command to
// point at. The install UI's "skip install" branch is keyed off
// InstallCmd==", so this is a real failure mode, not a stylistic one.
func TestCatalog_AllLocalNonKanbanHaveInstallCmd(t *testing.T) {
	for _, e := range Catalog() {
		if e.Type != "local" {
			continue
		}
		if e.ID == "ywai-kanban" || e.ID == "ywai-fastfs" {
			continue // documented exception: binary is ywai itself
		}
		if e.InstallCmd == "" {
			t.Errorf("entry %q: Type=local and not ywai-kanban/fastfs, "+
				"but InstallCmd is empty (install UI cannot render "+
				"an install step)", e.ID)
		}
	}
}

// TestCatalog_AllRequiredEnvHaveName pins that every EnvSpec declared
// in any entry's RequiredEnv has a non-empty Name. The install UI
// keys env-var lookup and credential form rendering off Name (it
// also feeds the redactor in RedactMessage). An empty Name would
// silently render as `<input name="">` and the credential would
// never make it to the subprocess environment. The per-entry tests
// assert on specific names (GITHUB_PERSONAL_ACCESS_TOKEN, DATABASE_URL)
// but nothing catches the general "Name must be set" rule across
// the whole catalog.
func TestCatalog_AllRequiredEnvHaveName(t *testing.T) {
	for _, e := range Catalog() {
		for i, spec := range e.RequiredEnv {
			if spec.Name == "" {
				t.Errorf("entry %q: RequiredEnv[%d].Name is empty "+
					"(install UI keys credential form off Name; "+
					"redactor and env-var injection also key off it)",
					e.ID, i)
			}
		}
	}
}
