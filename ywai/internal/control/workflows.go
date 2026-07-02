package control

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/config"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/workflows"
)

// workflowsRoutes attaches the Workflow Studio store to the control server.
// Initialized lazily so a server can run even before the data dir is writable.
type workflowsAPI struct {
	store    *workflows.Store
	exporter *workflows.Exporter
	hub      *wsHub
	runs     *runStore
}

func newWorkflowsAPI() *workflowsAPI {
	return &workflowsAPI{
		store:    workflows.NewStore(config.DataWorkflowsDir()),
		exporter: workflows.NewExporter(),
		hub:      newWsHub(),
		runs:     newRunStore(config.DataWorkflowsDir() + "/runs"),
	}
}

// registerWorkflowsRoutes mounts the Workflow Studio REST API.
func (s *Server) registerWorkflowsRoutes() {
	api := newWorkflowsAPI()
	s.workflows = api

	// List + create.
	s.mux.HandleFunc("GET /api/workflows", api.handleList)
	s.mux.HandleFunc("POST /api/workflows", api.handleCreate)

	s.mux.HandleFunc("POST /api/workflows/import", api.handleImport)

	// Per-workflow CRUD.
	s.mux.HandleFunc("GET /api/workflows/{name}", api.handleGet)
	s.mux.HandleFunc("PUT /api/workflows/{name}", api.handleSave)
	s.mux.HandleFunc("PATCH /api/workflows/{name}", api.handleRename)
	s.mux.HandleFunc("DELETE /api/workflows/{name}", api.handleDelete)

	// Validate + export.
	s.mux.HandleFunc("POST /api/workflows/{name}/validate", api.handleValidate)
	s.mux.HandleFunc("POST /api/workflows/{name}/export", api.handleExport)

	// Edit with AI (opencode CLI).
	s.mux.HandleFunc("POST /api/workflows/{name}/ai-edit", api.handleAIEdit)

	// Run: export + spawn the orchestrator via opencode, streaming output live.
	s.mux.HandleFunc("POST /api/workflows/{name}/run", api.handleRun)
	// Stop: cancel an in-progress run (kills the opencode process).
	s.mux.HandleFunc("POST /api/workflows/{name}/stop", api.handleStop)
	s.mux.HandleFunc("POST /api/workflows/{name}/input", api.handleInput)
	// Live run output stream (WebSocket) for the Run panel + Commentary.
	s.mux.HandleFunc("GET /api/workflows/ws", api.handleWorkflowWS)

	// MCP sync: replicate the workflow's referenced MCP servers from
	// opencode.json into claude.json (append-only).
	s.mux.HandleFunc("GET /api/workflows/{name}/mcp-sync/preview", api.handleMcpSyncPreview)
	s.mux.HandleFunc("POST /api/workflows/{name}/mcp-sync", api.handleMcpSyncApply)

	// Read-only catalogs the node editors populate from (skills live in the
	// opencode skills dir; MCP servers come from opencode.json).
	s.mux.HandleFunc("GET /api/workflows-meta/skills", api.handleSkillsList)
	s.mux.HandleFunc("GET /api/workflows-meta/mcps", api.handleMcpServersList)
	// Enumerate the tools a given MCP server exposes (live handshake). Used by
	// the MCP node editor when configuring a tool (manual/aiParameterConfig mode).
	s.mux.HandleFunc("GET /api/workflows-meta/mcps/{server}/tools", api.handleMcpServerTools)
}

// ─── handlers ──────────────────────────────────────────────────────────────

func (a *workflowsAPI) handleList(w http.ResponseWriter, r *http.Request) {
	summaries, err := a.store.List()
	if err != nil {
		writeWorkflowsError(w, http.StatusInternalServerError, err)
		return
	}
	// Guarantee a non-null array: the frontend maps over this unconditionally,
	// and a nil slice would JSON-serialize as `null`.
	if summaries == nil {
		summaries = []workflows.Summary{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"workflows": summaries})
}

func (a *workflowsAPI) handleGet(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	wf, err := a.store.Load(name)
	if err != nil {
		writeWorkflowsError(w, statusForWorkflowError(err), err)
		return
	}
	writeJSON(w, http.StatusOK, wf)
}

func (a *workflowsAPI) handleCreate(w http.ResponseWriter, r *http.Request) {
	var wf workflows.Workflow
	if err := decodeWorkflowBody(r, &wf); err != nil {
		writeWorkflowsError(w, http.StatusBadRequest, err)
		return
	}
	if err := a.store.Create(&wf); err != nil {
		writeWorkflowsError(w, statusForWorkflowError(err), err)
		return
	}
	writeJSON(w, http.StatusCreated, wf)
}

func (a *workflowsAPI) handleSave(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	var wf workflows.Workflow
	if err := decodeWorkflowBody(r, &wf); err != nil {
		writeWorkflowsError(w, http.StatusBadRequest, err)
		return
	}
	if wf.Name != name {
		writeWorkflowsError(w, http.StatusBadRequest, errors.New("workflow name in body must match URL"))
		return
	}
	if err := a.store.Save(&wf); err != nil {
		writeWorkflowsError(w, statusForWorkflowError(err), err)
		return
	}
	writeJSON(w, http.StatusOK, wf)
}

func (a *workflowsAPI) handleDelete(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if err := a.store.Delete(name); err != nil {
		writeWorkflowsError(w, statusForWorkflowError(err), err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// handleRename renames a workflow. Body: {"name": "new-name"}. The store
// renames the on-disk file and patches the workflow's Name/ID. Returns the
// reloaded workflow under its new name.
func (a *workflowsAPI) handleRename(w http.ResponseWriter, r *http.Request) {
	old := r.PathValue("name")
	var body struct {
		Name string `json:"name"`
	}
	if err := decodeJSONBody(r, &body); err != nil {
		writeWorkflowsError(w, http.StatusBadRequest, err)
		return
	}
	if strings.TrimSpace(body.Name) == "" {
		writeWorkflowsError(w, http.StatusBadRequest, errors.New("name is required"))
		return
	}
	if err := a.store.Rename(old, body.Name); err != nil {
		writeWorkflowsError(w, statusForWorkflowError(err), err)
		return
	}
	wf, err := a.store.Load(body.Name)
	if err != nil {
		writeWorkflowsError(w, statusForWorkflowError(err), err)
		return
	}
	writeJSON(w, http.StatusOK, wf)
}

func (a *workflowsAPI) handleImport(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeWorkflowsError(w, http.StatusBadRequest, err)
		return
	}
	defer func() { _ = r.Body.Close() }()

	var opts workflows.ImportOptions
	// The import endpoint accepts either raw workflow JSON or a wrapper
	// {"json": <raw>, "name": "..."}; detect the wrapper by probing for a
	// top-level "json" key.
	if wrapper := struct {
		JSON json.RawMessage `json:"json"`
		Name string          `json:"name"`
	}{}; json.Unmarshal(body, &wrapper) == nil && len(wrapper.JSON) > 0 {
		body = []byte(wrapper.JSON)
		opts.Name = wrapper.Name
	}

	result, err := workflows.Import(body, opts)
	if err != nil {
		writeWorkflowsError(w, http.StatusBadRequest, err)
		return
	}
	if err := a.store.Create(result.Workflow); err != nil {
		// If it already exists, overwrite — import is idempotent.
		if errors.Is(err, workflows.ErrWorkflowExists) {
			if err := a.store.Save(result.Workflow); err != nil {
				writeWorkflowsError(w, statusForWorkflowError(err), err)
				return
			}
		} else {
			writeWorkflowsError(w, statusForWorkflowError(err), err)
			return
		}
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *workflowsAPI) handleValidate(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	wf, err := a.store.Load(name)
	if err != nil {
		writeWorkflowsError(w, statusForWorkflowError(err), err)
		return
	}
	result := workflows.Validate(wf)
	writeJSON(w, http.StatusOK, result)
}

func (a *workflowsAPI) handleExport(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	wf, err := a.store.Load(name)
	if err != nil {
		writeWorkflowsError(w, statusForWorkflowError(err), err)
		return
	}

	// Target dialect: opencode (default) or claude-code, via ?target=.
	exporter := a.exporter
	if t := r.URL.Query().Get("target"); t != "" && t != workflows.TargetOpenCode {
		exporter = workflows.NewExporterForTarget(t)
	}

	// Dry-run preview unless ?apply=true.
	if r.URL.Query().Get("apply") != "true" {
		plan, _, err := exporter.Plan(wf)
		if err != nil {
			writeWorkflowsError(w, http.StatusInternalServerError, err)
			return
		}
		// Preview sub-flow artifacts too so the user sees what would be written.
		previewed := map[string]bool{name: true}
		previewSubFlows(a.store, exporter, wf, plan, previewed)
		writeJSON(w, http.StatusOK, plan)
		return
	}

	plan, err := exporter.Apply(wf)
	if err != nil {
		writeWorkflowsError(w, http.StatusInternalServerError, err)
		return
	}

	// Recursively export any sub-workflows referenced by subAgentFlow nodes.
	// Without this, the orchestrator's "Run the /<flowId> sub-workflow" step
	// points at a command that was never written and fails at runtime.
	exported := map[string]bool{name: true}
	exportSubFlows(a.store, exporter, wf, plan, exported)

	writeJSON(w, http.StatusOK, plan)
}

// previewSubFlows is the dry-run counterpart of exportSubFlows: it plans (but
// does not write) every referenced sub-flow so the preview shows them.
func previewSubFlows(store *workflows.Store, exporter *workflows.Exporter, wf *workflows.Workflow, plan *workflows.ExportPlan, previewed map[string]bool) {
	for _, n := range wf.Nodes {
		if n.Type != workflows.NodeTypeSubAgentFlow {
			continue
		}
		flowID := n.Data.FlowID
		if flowID == "" || previewed[flowID] {
			continue
		}
		previewed[flowID] = true
		sub, err := store.Load(flowID)
		if err != nil {
			continue
		}
		subPlan, _, err := exporter.Plan(sub)
		if err != nil {
			continue
		}
		plan.Files = append(plan.Files, subPlan.Files...)
		previewSubFlows(store, exporter, sub, plan, previewed)
	}
}

// exportSubFlows recursively exports every subAgentFlow referenced by wf. Each
// subflow is loaded from the store and applied with the same exporter/target so
// its slash command + agents land on disk. Already-exported names are tracked
// to avoid infinite loops on cyclic sub-flow references. The parent plan's
// Files slice is extended with the subflow artifacts so the UI shows them.
func exportSubFlows(store *workflows.Store, exporter *workflows.Exporter, wf *workflows.Workflow, plan *workflows.ExportPlan, exported map[string]bool) {
	for _, n := range wf.Nodes {
		if n.Type != workflows.NodeTypeSubAgentFlow {
			continue
		}
		flowID := n.Data.FlowID
		if flowID == "" || exported[flowID] {
			continue
		}
		exported[flowID] = true
		sub, err := store.Load(flowID)
		if err != nil {
			continue // sub-flow not on disk; skip silently (caller can import it)
		}
		subPlan, err := exporter.Apply(sub)
		if err != nil {
			continue
		}
		plan.Files = append(plan.Files, subPlan.Files...)
		// Recurse into the subflow's own sub-flow references (one level of
		// nesting is common; the exported map guards against cycles).
		exportSubFlows(store, exporter, sub, plan, exported)
	}
}

// ─── helpers ───────────────────────────────────────────────────────────────

func decodeWorkflowBody(r *http.Request, wf *workflows.Workflow) error {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return err
	}
	defer func() { _ = r.Body.Close() }()
	if len(body) == 0 {
		return errors.New("empty request body")
	}
	return json.Unmarshal(body, wf)
}

func statusForWorkflowError(err error) int {
	switch {
	case errors.Is(err, workflows.ErrWorkflowNotFound):
		return http.StatusNotFound
	case errors.Is(err, workflows.ErrWorkflowExists):
		return http.StatusConflict
	case errors.Is(err, workflows.ErrInvalidName):
		return http.StatusBadRequest
	default:
		return http.StatusInternalServerError
	}
}

func writeWorkflowsError(w http.ResponseWriter, code int, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
}
