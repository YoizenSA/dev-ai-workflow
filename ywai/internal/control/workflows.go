package control

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/config"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/workflows"
)

// workflowsRoutes attaches the Workflow Studio store to the control server.
// Initialized lazily so a server can run even before the data dir is writable.
type workflowsAPI struct {
	store    *workflows.Store
	exporter *workflows.Exporter
}

func newWorkflowsAPI() *workflowsAPI {
	return &workflowsAPI{
		store:    workflows.NewStore(config.DataWorkflowsDir()),
		exporter: workflows.NewExporter(),
	}
}

// registerWorkflowsRoutes mounts the Workflow Studio REST API.
func (s *Server) registerWorkflowsRoutes() {
	api := newWorkflowsAPI()
	s.workflows = api

	// List + create.
	s.mux.HandleFunc("GET /api/workflows", api.handleList)
	s.mux.HandleFunc("POST /api/workflows", api.handleCreate)

	// Import cc-wf-studio JSON.
	s.mux.HandleFunc("POST /api/workflows/import", api.handleImport)

	// Per-workflow CRUD.
	s.mux.HandleFunc("GET /api/workflows/{name}", api.handleGet)
	s.mux.HandleFunc("PUT /api/workflows/{name}", api.handleSave)
	s.mux.HandleFunc("DELETE /api/workflows/{name}", api.handleDelete)

	// Validate + export.
	s.mux.HandleFunc("POST /api/workflows/{name}/validate", api.handleValidate)
	s.mux.HandleFunc("POST /api/workflows/{name}/export", api.handleExport)

	// Edit with AI (opencode CLI).
	s.mux.HandleFunc("POST /api/workflows/{name}/ai-edit", api.handleAIEdit)

	// Read-only catalogs the node editors populate from (skills live in the
	// opencode skills dir; MCPs are served by the existing /api/mcp/catalog).
	s.mux.HandleFunc("GET /api/workflows-meta/skills", api.handleSkillsList)
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

func (a *workflowsAPI) handleImport(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeWorkflowsError(w, http.StatusBadRequest, err)
		return
	}
	defer func() { _ = r.Body.Close() }()

	var opts workflows.ImportOptions
	// The import endpoint accepts either raw cc-wf-studio JSON or a wrapper
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
		writeJSON(w, http.StatusOK, plan)
		return
	}

	plan, err := exporter.Apply(wf)
	if err != nil {
		writeWorkflowsError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, plan)
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
