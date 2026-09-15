package control

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/agent"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/envprofile"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/selfupdate"
)

// registerEnvProfileRoutes wires isolated opencode2 environment ("ywai env")
// management into the control server. All handlers are JSON and hermetic in
// tests via envprofile.SetProfilesRootForTest.
func (s *Server) registerEnvProfileRoutes() {
	s.mux.HandleFunc("GET /api/envs", s.handleEnvList)
	s.mux.HandleFunc("POST /api/envs", s.handleEnvCreate)
	s.mux.HandleFunc("PATCH /api/envs/{name}", s.handleEnvPatch)
	s.mux.HandleFunc("DELETE /api/envs/{name}", s.handleEnvDelete)
	s.mux.HandleFunc("POST /api/envs/{name}/apply", s.handleEnvApply)
	s.mux.HandleFunc("POST /api/envs/{name}/start", s.handleEnvStart)
	s.mux.HandleFunc("POST /api/envs/{name}/stop", s.handleEnvStop)
	s.mux.HandleFunc("GET /api/envs/{name}/status", s.handleEnvStatus)
	s.mux.HandleFunc("GET /api/envs/{name}/logs", s.handleEnvLogs)
	s.mux.HandleFunc("POST /api/envs/{name}/import-providers", s.handleEnvImportProviders)
	s.mux.HandleFunc("POST /api/envs/{name}/import-ado", s.handleEnvImportAdo)
	s.registerEnvPresetRoutes()
}

// envItem is one profile in list responses, with live service state.
type envItem struct {
	Name      string `json:"name"`
	Preset    string `json:"preset"`
	Port      int    `json:"port"`
	CreatedAt string `json:"created_at"`
	Running   bool   `json:"running"`
	URL       string `json:"url"`
}

// toEnvItem attaches live status to a stored profile. A status probe error
// never fails the list: the profile shows as stopped.
func toEnvItem(p envprofile.Profile) envItem {
	running, _ := envprofile.Status(p)
	return envItem{
		Name:      p.Name,
		Preset:    p.Preset,
		Port:      p.Port,
		CreatedAt: p.CreatedAt,
		Running:   running,
		URL:       envprofile.Env(p)["OPENCODE_URL"],
	}
}

// handleEnvList returns every profile with preset/port/status plus presets.
// GET /api/envs
func (s *Server) handleEnvList(w http.ResponseWriter, r *http.Request) {
	profiles, err := envprofile.List()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	items := make([]envItem, 0, len(profiles))
	for _, p := range profiles {
		items = append(items, toEnvItem(p))
	}
	descriptions := map[string]string{}
	copyDefaults := map[string]bool{}
	for _, name := range envprofile.Presets() {
		if spec, err := envprofile.Preset(name); err == nil {
			if d, _ := spec["description"].(string); d != "" {
				descriptions[name] = d
			}
			copyDefaults[name] = envprofile.PresetCopyProviders(spec)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"envs":                items,
		"presets":             envprofile.Presets(),
		"preset_descriptions": descriptions,
		"custom_presets":      customPresetNames(),
		// Default of the "copy global providers" check per preset.
		"preset_copy_providers": copyDefaults,
		"catalog":               envCatalog(),
	})
}

// envCreateRequest is the POST /api/envs body.
type envCreateRequest struct {
	Name   string `json:"name"`
	Preset string `json:"preset"`
	// CopyProviders seeds the env with the global providers + credentials.
	// Absent = the preset default (copy_global_providers).
	CopyProviders *bool `json:"copy_providers"`
}

// handleEnvCreate creates one profile. Unknown presets are 400, duplicate
// names are 409, invalid names are 400.
// POST /api/envs
func (s *Server) handleEnvCreate(w http.ResponseWriter, r *http.Request) {
	var req envCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid body: " + err.Error()})
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	req.Preset = strings.TrimSpace(req.Preset)
	if req.Name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name is required"})
		return
	}
	if req.Preset != "" {
		if _, err := envprofile.Preset(req.Preset); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
	}
	p, err := envprofile.Create(req.Name, req.Preset)
	if err != nil {
		switch {
		case strings.Contains(err.Error(), "already exists"):
			writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
		case strings.Contains(err.Error(), "invalid profile name"),
			strings.Contains(err.Error(), "reserved by a ywai command"):
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		default:
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
		return
	}
	copyProviders := false
	if req.CopyProviders != nil {
		copyProviders = *req.CopyProviders
	} else if spec, err := envprofile.Preset(p.Preset); err == nil {
		copyProviders = envprofile.PresetCopyProviders(spec)
	}
	resp := map[string]any{"profile": p, "running": false, "url": envprofile.Env(p)["OPENCODE_URL"]}
	if copyProviders {
		copied, err := envprofile.CopyGlobalProviders(p)
		if err != nil {
			// The env exists; report the copy failure without failing create.
			resp["copy_error"] = err.Error()
		} else {
			resp["copied"] = copied
			restartEnvServiceForLogins(p, copied)
		}
	}
	// Ado profiles follow their own preset key, independent of providers.
	if spec, err := envprofile.Preset(p.Preset); err == nil && envprofile.PresetCopyAdo(spec) {
		// Seed `ado init` profiles too (env wins on conflicts); warn-only.
		if adopted, aerr := envprofile.CopyAdoConfig(p); aerr != nil {
			resp["copy_ado_error"] = aerr.Error()
		} else {
			resp["copied_ado"] = adopted
		}
	}
	writeJSON(w, http.StatusCreated, resp)
}

// handleEnvImportProviders copies the global providers + credentials into an
// existing env (entries the env already has are kept).
// POST /api/envs/{name}/import-providers
func (s *Server) handleEnvImportProviders(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimSpace(r.PathValue("name"))
	if err := envprofile.ValidateName(name); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	p, err := envprofile.Get(name)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": fmt.Sprintf("unknown profile %q", name)})
		return
	}
	copied, err := envprofile.CopyGlobalProviders(p)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	restartEnvServiceForLogins(p, copied)
	writeJSON(w, http.StatusOK, map[string]any{"name": name, "copied": copied})
}

// handleEnvImportAdo merges the global `ado` CLI profiles into an existing
// env (entries the env already has are kept).
// POST /api/envs/{name}/import-ado
func (s *Server) handleEnvImportAdo(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimSpace(r.PathValue("name"))
	if err := envprofile.ValidateName(name); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	p, err := envprofile.Get(name)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": fmt.Sprintf("unknown profile %q", name)})
		return
	}
	copied, err := envprofile.CopyAdoConfig(p)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"name": name, "copied_ado": copied})
}

// restartEnvServiceForLogins stops the env's managed opencode service after
// new logins or providers landed, so its next start (TUI/run restart it on
// demand) loads them. Best effort: a missing binary or stop failure is not
// fatal to the import.
func restartEnvServiceForLogins(p envprofile.Profile, copied envprofile.CopiedProviders) {
	if len(copied.Credentials)+len(copied.Providers) == 0 {
		return
	}
	if bin, _ := agent.FindOpenCode(); bin != "" {
		_ = envprofile.StopManagedService(p, bin)
	}
}

// envPatchRequest is the PATCH /api/envs/{name} body: preset change and/or
// content-list overrides. A present (even empty) groups/skills/mcp array
// replaces the preset list; an absent (or null) one inherits the preset.
type envPatchRequest struct {
	Preset *string   `json:"preset"`
	Groups *[]string `json:"groups"`
	Skills *[]string `json:"skills"`
	MCP    *[]string `json:"mcp"`
	// Reset drops the named overrides ("groups", "skills", "mcp") so they
	// inherit the preset again. JSON null cannot express this: a null list
	// decodes to an absent field and leaves the override untouched.
	Reset []string `json:"reset"`
}

// handleEnvPatch changes one profile preset and/or its content overrides.
// PATCH /api/envs/{name}
func (s *Server) handleEnvPatch(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimSpace(r.PathValue("name"))
	if name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "profile name is required"})
		return
	}
	if err := envprofile.ValidateName(name); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	var req envPatchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid body: " + err.Error()})
		return
	}
	p, err := envprofile.Get(name)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": fmt.Sprintf("unknown profile %q", name)})
		return
	}
	if req.Preset != nil {
		preset := strings.TrimSpace(*req.Preset)
		if preset == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "preset is required"})
			return
		}
		if _, err := envprofile.Preset(preset); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		p.Preset = preset
		// A preset change resets content overrides: they belonged to the old base.
		p.Overrides = nil
	}
	if req.Groups != nil || req.Skills != nil || req.MCP != nil {
		if p.Overrides == nil {
			p.Overrides = &envprofile.ProfileOverrides{}
		}
		if req.Groups != nil {
			p.Overrides.Groups = cleanStringList(*req.Groups)
		}
		if req.Skills != nil {
			p.Overrides.Skills = cleanStringList(*req.Skills)
		}
		if req.MCP != nil {
			p.Overrides.MCP = cleanStringList(*req.MCP)
		}
	}
	if p.Overrides != nil {
		for _, key := range req.Reset {
			switch strings.TrimSpace(key) {
			case "groups":
				p.Overrides.Groups = nil
			case "skills":
				p.Overrides.Skills = nil
			case "mcp":
				p.Overrides.MCP = nil
			default:
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("unknown reset key %q", key)})
				return
			}
		}
		if p.Overrides.Groups == nil && p.Overrides.Skills == nil && p.Overrides.MCP == nil {
			p.Overrides = nil
		}
	}
	dirs := envprofile.Dirs(p)
	if err := p.SaveManifest(dirs["root"]); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"profile": p})
}

// cleanStringList trims and drops blanks. An all-blank input yields an empty
// (non-nil) list, which means "install none" rather than "inherit".
func cleanStringList(in []string) []string {
	out := make([]string, 0, len(in))
	for _, s := range in {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	return out
}

// handleEnvDelete stops the profile service, then removes its directory.
// DELETE /api/envs/{name}
func (s *Server) handleEnvDelete(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimSpace(r.PathValue("name"))
	if name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "profile name is required"})
		return
	}
	if err := envprofile.ValidateName(name); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	p, err := envprofile.Get(name)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": fmt.Sprintf("unknown profile %q", name)})
		return
	}
	// A missing or stale pid file is success, so Stop errors are advisory.
	_ = envprofile.Stop(p)
	if err := envprofile.Delete(name); err != nil {
		if strings.Contains(err.Error(), "files still locked") {
			writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
		} else {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted", "name": name})
}

// envApplyRunner shells out to `ywai install --profile <name>` in a child
// process. It is a variable so httptest can stub the subprocess without
// touching the real filesystem or the in-process env sandbox.
var envApplyRunner = defaultEnvApplyRunner

// defaultEnvApplyRunner runs the apply in a subprocess. It never mutates the
// server process environment: profile isolation happens in the child.
// --agent opencode is required: the child has no TTY and install refuses to
// run agent-wide non-interactively; profiles are opencode config anyway.
func defaultEnvApplyRunner(ctx context.Context, name string) (string, error) {
	exe, err := selfupdate.ResolvedExecutable()
	if err != nil {
		return "", err
	}
	cmd := exec.CommandContext(ctx, exe, "install", "--profile", name, "--agent", "opencode")
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err = cmd.Run()
	return buf.String(), err
}

// handleEnvStart starts the profile server (idempotent: already running is
// success). Needs the opencode2 binary on the server machine.
// POST /api/envs/{name}/start
func (s *Server) handleEnvStart(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimSpace(r.PathValue("name"))
	if err := envprofile.ValidateName(name); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	p, err := envprofile.Get(name)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": fmt.Sprintf("unknown profile %q", name)})
		return
	}
	if running, _ := envprofile.Status(p); running {
		writeJSON(w, http.StatusOK, map[string]any{"name": name, "running": true, "url": envprofile.Env(p)["OPENCODE_URL"]})
		return
	}
	bin, _ := agent.FindOpenCode()
	if bin == "" {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "opencode2 binary not found on server"})
		return
	}
	if err := envprofile.Start(r.Context(), p, bin); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"name": name, "running": true, "url": envprofile.Env(p)["OPENCODE_URL"]})
}

// handleEnvStop stops the profile server. Missing or stale pid files are
// success (already stopped).
// POST /api/envs/{name}/stop
func (s *Server) handleEnvStop(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimSpace(r.PathValue("name"))
	if err := envprofile.ValidateName(name); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	p, err := envprofile.Get(name)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": fmt.Sprintf("unknown profile %q", name)})
		return
	}
	_ = envprofile.Stop(p)
	writeJSON(w, http.StatusOK, map[string]any{"name": name, "running": false})
}

// handleEnvApply applies the profile pipeline via a subprocess, never via
// the in-process env sandbox (which mutates process state and cannot run
// concurrently). POST /api/envs/{name}/apply
func (s *Server) handleEnvApply(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimSpace(r.PathValue("name"))
	if name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "profile name is required"})
		return
	}
	if err := envprofile.ValidateName(name); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if _, err := envprofile.Get(name); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": fmt.Sprintf("unknown profile %q", name)})
		return
	}
	output, err := envApplyRunner(r.Context(), name)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error(), "output": output})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"name": name, "ok": true, "output": output})
}

// doctorCheck is one ywai-env doctor line in the status panel.
type doctorCheck struct {
	Name    string `json:"name"`
	OK      bool   `json:"ok"`
	Message string `json:"message"`
}

// handleEnvStatus reports service, database, auth hint and doctor summary.
// GET /api/envs/{name}/status
func (s *Server) handleEnvStatus(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimSpace(r.PathValue("name"))
	if name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "profile name is required"})
		return
	}
	if err := envprofile.ValidateName(name); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	p, err := envprofile.Get(name)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": fmt.Sprintf("unknown profile %q", name)})
		return
	}
	env := envprofile.Env(p)
	dirs := envprofile.Dirs(p)
	running, _ := envprofile.Status(p)

	dbPath := env["OPENCODE_DB"]
	dbExists := false
	var dbSize int64
	if st, err := os.Stat(dbPath); err == nil && !st.IsDir() {
		dbExists = true
		dbSize = st.Size()
	}

	logPath := envprofile.ServerLog(p)
	logExists := false
	var logSize int64
	if st, err := os.Stat(logPath); err == nil && !st.IsDir() {
		logExists = true
		logSize = st.Size()
	}

	credsConfigured := false
	if st, err := os.Stat(envprofile.CredsFile(p)); err == nil && !st.IsDir() {
		credsConfigured = true
	}
	authHint := "credentials generate on first start"
	if credsConfigured {
		authHint = "per-profile server credentials configured"
	}

	checks := buildEnvDoctorChecks(p, dirs, dbExists, dbSize)
	overall := true
	for _, c := range checks {
		if !c.OK {
			overall = false
			break
		}
	}

	spec, _ := envprofile.PresetSpec(p)
	if spec == nil {
		spec = map[string]any{}
	}
	// The raw preset (no overrides) lets the editor label what "inherit"
	// means for each list.
	presetSpec, _ := envprofile.Preset(p.Preset)
	if presetSpec == nil {
		presetSpec = map[string]any{}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"name":   p.Name,
		"preset": p.Preset,
		"port":   p.Port,
		"url":    env["OPENCODE_URL"],
		// Resolved content (preset + manifest overrides) drives the web
		// editor: checkbox options come from spec lists, checked state from
		// overrides (nil = inherits, shown checked).
		"spec":        spec,
		"preset_spec": presetSpec,
		"overrides":   p.Overrides,
		"service": map[string]any{
			"running": running,
			"port":    p.Port,
			"url":     env["OPENCODE_URL"],
		},
		"db": map[string]any{
			"path":       dbPath,
			"exists":     dbExists,
			"size_bytes": dbSize,
		},
		"serverLog": map[string]any{
			"path":       logPath,
			"exists":     logExists,
			"size_bytes": logSize,
		},
		"auth": map[string]any{
			"configured": credsConfigured,
			"hint":       authHint,
		},
		"doctor": map[string]any{
			"ok":     overall,
			"checks": checks,
		},
	})
}

// buildEnvDoctorChecks summarizes profile health: manifest, layout, preset,
// port, database and auth. Service state is informational only: a stopped
// profile is healthy.
func buildEnvDoctorChecks(p envprofile.Profile, dirs map[string]string, dbExists bool, dbSize int64) []doctorCheck {
	checks := []doctorCheck{
		{Name: "manifest", OK: true, Message: "manifest readable"},
	}

	var missing []string
	for _, d := range []string{"config", "data", "state", "cache", "tmp", "run", "evals"} {
		st, err := os.Stat(dirs[d])
		if err != nil || !st.IsDir() {
			missing = append(missing, d)
		}
	}
	if len(missing) == 0 {
		checks = append(checks, doctorCheck{Name: "layout", OK: true, Message: "profile layout complete"})
	} else {
		checks = append(checks, doctorCheck{Name: "layout", OK: false, Message: "missing dirs: " + strings.Join(missing, ", ")})
	}

	if _, err := envprofile.Preset(p.Preset); err != nil {
		checks = append(checks, doctorCheck{Name: "preset", OK: false, Message: fmt.Sprintf("unknown preset %q", p.Preset)})
	} else {
		checks = append(checks, doctorCheck{Name: "preset", OK: true, Message: fmt.Sprintf("preset %q known", p.Preset)})
	}

	if p.Port >= 5800 && p.Port <= 5899 {
		checks = append(checks, doctorCheck{Name: "port", OK: true, Message: fmt.Sprintf("port %d in 5800-5899", p.Port)})
	} else {
		checks = append(checks, doctorCheck{Name: "port", OK: false, Message: fmt.Sprintf("port %d outside 5800-5899", p.Port)})
	}

	if dbExists {
		checks = append(checks, doctorCheck{Name: "database", OK: true, Message: fmt.Sprintf("database present (%d bytes)", dbSize)})
	} else {
		checks = append(checks, doctorCheck{Name: "database", OK: true, Message: "no database yet — created on first start"})
	}
	return checks
}

// handleEnvLogs tails the profile-scoped opencode server log.
// GET /api/envs/{name}/logs?lines=N (default 200, max 2000)
func (s *Server) handleEnvLogs(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimSpace(r.PathValue("name"))
	if name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "profile name is required"})
		return
	}
	if err := envprofile.ValidateName(name); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	p, err := envprofile.Get(name)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": fmt.Sprintf("unknown profile %q", name)})
		return
	}
	want := 200
	if raw := strings.TrimSpace(r.URL.Query().Get("lines")); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 0 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "lines must be a non-negative integer"})
			return
		}
		want = n
	}
	if want > 2000 {
		want = 2000
	}
	path := envprofile.ServerLog(p)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			writeJSON(w, http.StatusOK, map[string]any{
				"name": name, "path": path, "exists": false,
				"lines": []string{}, "returned": 0, "total": 0, "truncated": false,
			})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	lines := strings.Split(text, "\n")
	// A trailing newline produces one empty final element that is not a line.
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	total := len(lines)
	if total == 0 {
		writeJSON(w, http.StatusOK, map[string]any{
			"name": name, "path": path, "exists": true,
			"lines": []string{}, "returned": 0, "total": 0, "truncated": false,
		})
		return
	}
	start := 0
	truncated := false
	if want < total {
		start = total - want
		truncated = true
	}
	if want == 0 {
		lines = []string{}
		truncated = total > 0
	} else {
		lines = lines[start:]
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"name": name, "path": path, "exists": true,
		"lines": lines, "returned": len(lines), "total": total, "truncated": truncated,
	})
}
