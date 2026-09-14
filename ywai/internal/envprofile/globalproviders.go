package envprofile

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	_ "modernc.org/sqlite" // pure-Go SQLite driver (opencode.db credentials)

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/config"
)

// An env is a fresh opencode home: no providers in its opencode.json and no
// logins in its database, so a model the global install reaches fine fails
// inside the env. CopyGlobalProviders seeds both from the global install on
// request (web "copy providers" check, `env create --copy-providers`, preset
// key copy_global_providers).
//
// opencode2 keeps logins (`opencode2 auth login`) in the `credential` table
// of opencode.db, not in auth.json, so that table is what gets copied.

// PresetCopyProviders reports the preset's default for copying the global
// providers into a new env (preset key copy_global_providers).
func PresetCopyProviders(spec map[string]any) bool {
	v, _ := spec["copy_global_providers"].(bool)
	return v
}

// globalOpenCodeDataDir is the user's own opencode data dir (opencode.db):
// $XDG_DATA_HOME/opencode or ~/.local/share/opencode.
func globalOpenCodeDataDir() string {
	if xdg := strings.TrimSpace(os.Getenv("XDG_DATA_HOME")); xdg != "" {
		return filepath.Join(xdg, "opencode")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "share", "opencode")
}

// CopiedProviders reports what CopyGlobalProviders added to an env.
type CopiedProviders struct {
	Providers   []string `json:"providers"`   // opencode.json provider entries
	Credentials []string `json:"credentials"` // opencode.db logins (integration ids)
	// Note explains a skipped step (e.g. the env database does not exist yet).
	Note string `json:"note,omitempty"`
}

// CopyGlobalProviders merges the global opencode.json "provider" section and
// the global logins into the env. Entries the env already has win, so
// re-running never clobbers env-specific setup. It must run outside a
// profile sandbox (the global paths would resolve to the env itself). After
// credentials are copied, the env's managed service must restart to see them
// (StopManagedService).
func CopyGlobalProviders(p Profile) (CopiedProviders, error) {
	var out CopiedProviders
	if InProfileScope() {
		return out, fmt.Errorf("copy global providers must run outside a profile scope")
	}
	dirs := Dirs(p)
	globalCfg := filepath.Join(config.OpenCodeUserConfigDir(), "opencode.json")
	envCfg := filepath.Join(dirs["config"], "opencode", "opencode.json")
	if filepath.Clean(globalCfg) == filepath.Clean(envCfg) {
		return out, fmt.Errorf("global and env config resolve to the same file")
	}

	// Providers (opencode.json → "provider").
	global, err := readJSONObject(globalCfg)
	if err != nil {
		return out, err
	}
	if gp, ok := global["provider"].(map[string]any); ok && len(gp) > 0 {
		env, err := readJSONObject(envCfg)
		if err != nil {
			return out, err
		}
		ep, _ := env["provider"].(map[string]any)
		if ep == nil {
			ep = map[string]any{}
		}
		for name, def := range gp {
			if _, exists := ep[name]; !exists {
				ep[name] = def
				out.Providers = append(out.Providers, name)
			}
		}
		if len(out.Providers) > 0 {
			env["provider"] = ep
			if _, ok := env["$schema"]; !ok {
				if s, ok := global["$schema"]; ok {
					env["$schema"] = s
				}
			}
			// 0600: provider options usually carry an apiKey.
			if err := writeJSONObject(envCfg, env, 0o600); err != nil {
				return out, err
			}
		}
	}

	// Logins (opencode.db → credential).
	creds, note, err := copyGlobalCredentials(Env(p)["OPENCODE_DB"])
	if err != nil {
		return out, err
	}
	out.Credentials, out.Note = creds, note
	sort.Strings(out.Providers)
	sort.Strings(out.Credentials)
	return out, nil
}

// copyGlobalCredentials copies rows of the global opencode.db `credential`
// table into the env database. Rows the env already has (same id, or same
// integration + label) are skipped. Only columns present in both schemas are
// copied, so a newer/older opencode on either side still works. A missing
// env database or table is not an error: opencode creates it on the env's
// first start, and the returned note says to import again then.
func copyGlobalCredentials(envDBPath string) ([]string, string, error) {
	globalPath := filepath.Join(globalOpenCodeDataDir(), "opencode.db")
	if filepath.Clean(globalPath) == filepath.Clean(envDBPath) {
		return nil, "", fmt.Errorf("global and env database resolve to the same file")
	}
	if _, err := os.Stat(globalPath); err != nil {
		return nil, "", nil // no global logins to copy
	}
	const notYet = "env database not created yet — start the env once, then import providers again to copy logins"
	if _, err := os.Stat(envDBPath); err != nil {
		return nil, notYet, nil
	}
	src, err := sql.Open("sqlite", "file:"+filepath.ToSlash(globalPath)+"?mode=ro&_pragma=busy_timeout(3000)")
	if err != nil {
		return nil, "", fmt.Errorf("open global database: %w", err)
	}
	defer src.Close()
	dst, err := sql.Open("sqlite", "file:"+filepath.ToSlash(envDBPath)+"?_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, "", fmt.Errorf("open env database: %w", err)
	}
	defer dst.Close()

	srcCols, err := tableColumns(src, "credential")
	if err != nil {
		return nil, "", fmt.Errorf("read global credential schema: %w", err)
	}
	if len(srcCols) == 0 {
		return nil, "", nil
	}
	dstCols, err := tableColumns(dst, "credential")
	if err != nil {
		return nil, "", fmt.Errorf("read env credential schema: %w", err)
	}
	if len(dstCols) == 0 {
		return nil, notYet, nil
	}
	inDst := map[string]bool{}
	for _, c := range dstCols {
		inDst[c] = true
	}
	var cols []string
	idx := map[string]int{}
	for _, c := range srcCols {
		if inDst[c] {
			idx[c] = len(cols)
			cols = append(cols, c)
		}
	}
	for _, need := range []string{"id", "integration_id", "label", "value"} {
		if _, ok := idx[need]; !ok {
			return nil, "", fmt.Errorf("credential table lacks column %q on one side; opencode versions differ too much", need)
		}
	}

	haveID, haveKey := map[string]bool{}, map[string]bool{}
	rows, err := dst.Query(`SELECT id, coalesce(integration_id, ''), label FROM credential`)
	if err != nil {
		return nil, "", fmt.Errorf("read env credentials: %w", err)
	}
	for rows.Next() {
		var id, integ, label string
		if err := rows.Scan(&id, &integ, &label); err != nil {
			rows.Close()
			return nil, "", err
		}
		haveID[id] = true
		haveKey[integ+"\x00"+label] = true
	}
	rows.Close()

	quoted := make([]string, len(cols))
	marks := make([]string, len(cols))
	for i, c := range cols {
		quoted[i] = "`" + c + "`"
		marks[i] = "?"
	}
	srcRows, err := src.Query("SELECT " + strings.Join(quoted, ", ") + " FROM credential")
	if err != nil {
		return nil, "", fmt.Errorf("read global credentials: %w", err)
	}
	type row []any
	var pending []row
	for srcRows.Next() {
		vals := make(row, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := srcRows.Scan(ptrs...); err != nil {
			srcRows.Close()
			return nil, "", err
		}
		pending = append(pending, vals)
	}
	srcRows.Close()

	insert := "INSERT INTO credential (" + strings.Join(quoted, ", ") + ") VALUES (" + strings.Join(marks, ", ") + ")"
	var copied []string
	for _, vals := range pending {
		id, integ, label := asString(vals[idx["id"]]), asString(vals[idx["integration_id"]]), asString(vals[idx["label"]])
		if haveID[id] || haveKey[integ+"\x00"+label] {
			continue
		}
		if _, err := dst.Exec(insert, vals...); err != nil {
			return copied, "", fmt.Errorf("copy credential %s: %w", integ, err)
		}
		name := integ
		if name == "" {
			name = label
		}
		copied = append(copied, name)
	}
	return copied, "", nil
}

// tableColumns lists a table's columns, or nil when the table is absent.
func tableColumns(db *sql.DB, table string) ([]string, error) {
	rows, err := db.Query(`SELECT name FROM pragma_table_info(?)`, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var cols []string
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			return nil, err
		}
		cols = append(cols, c)
	}
	return cols, rows.Err()
}

func asString(v any) string {
	switch s := v.(type) {
	case string:
		return s
	case []byte:
		return string(s)
	case nil:
		return ""
	default:
		return fmt.Sprint(s)
	}
}

// readJSONObject reads a JSON object file; a missing or empty file is {}.
func readJSONObject(path string) (map[string]any, error) {
	obj := map[string]any{}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return obj, nil
		}
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return obj, nil
	}
	if err := json.Unmarshal(data, &obj); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return obj, nil
}

func writeJSONObject(path string, obj map[string]any, perm os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create dir for %s: %w", path, err)
	}
	data, err := json.MarshalIndent(obj, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, append(data, '\n'), perm); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}
