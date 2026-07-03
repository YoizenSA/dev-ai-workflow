package scheduler

import (
	"encoding/json"
	"net/http"
)

// Handler serves the scheduler HTTP API.
type Handler struct {
	scheduler *Scheduler
	mux       http.Handler
}

// NewHandler creates a new Handler from a Scheduler.
func NewHandler(scheduler *Scheduler) *Handler {
	h := &Handler{scheduler: scheduler}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/schedules", h.listSchedules)
	mux.HandleFunc("POST /api/schedules", h.createSchedule)
	mux.HandleFunc("DELETE /api/schedules/{id}", h.deleteSchedule)
	mux.HandleFunc("GET /api/schedules/{id}/runs", h.getRuns)
	h.mux = mux
	return h
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mux.ServeHTTP(w, r)
}

func (h *Handler) listSchedules(w http.ResponseWriter, r *http.Request) {
	schedules, err := h.scheduler.ListSchedules()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if schedules == nil {
		schedules = []Schedule{}
	}
	writeJSON(w, http.StatusOK, schedules)
}

func (h *Handler) createSchedule(w http.ResponseWriter, r *http.Request) {
	var sched Schedule
	if err := json.NewDecoder(r.Body).Decode(&sched); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	if sched.Config.Goal == "" || sched.Config.Repo == "" || sched.Config.Agent == "" {
		http.Error(w, "missing required fields", http.StatusBadRequest)
		return
	}
	if _, err := ParseCron(sched.CronExpr); err != nil {
		http.Error(w, "invalid cron expression", http.StatusBadRequest)
		return
	}
	created, err := h.scheduler.AddSchedule(sched)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func (h *Handler) deleteSchedule(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "missing id", http.StatusBadRequest)
		return
	}
	if err := h.scheduler.RemoveSchedule(id); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) getRuns(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	runs, err := h.scheduler.GetRuns(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if runs == nil {
		runs = []ScheduleRun{}
	}
	writeJSON(w, http.StatusOK, runs)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
