package control

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strings"
	"time"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/missions"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/workflows"
	"github.com/creack/pty"
)

// runRequest is the body of POST /api/workflows/{name}/run.
type runRequest struct {
	Args  string `json:"args"`            // forwarded to the orchestrator as $ARGUMENTS
	Model string `json:"model,omitempty"` // optional model override
}

// runResponse is the 202 reply confirming a run started.
type runResponse struct {
	Status string `json:"status"` // "started" | "already-running"
	RunID  string `json:"runId"`
}

// handleRun exports the workflow (so the orchestrator + agents are current) and
// launches `opencode run --agent <name>-orchestrator "<args>"` in a goroutine,
// streaming its output to the workflows WebSocket hub. Responds 202 immediately.
//
// Pattern mirrors missions' RunMission (handlers.go:985): idempotency guard via
// the run store, background goroutine, 202 Accepted. The opencode invocation
// reuses the SpawnWorker/aiEditWorkflow CLI approach (see workflows_ai.go:65).
func (a *workflowsAPI) handleRun(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	wf, err := a.store.Load(name)
	if err != nil {
		writeWorkflowsError(w, statusForWorkflowError(err), err)
		return
	}

	// Already running? Decline a second spawn (return the active run id).
	if a.runs.isRunning(name) {
		writeJSON(w, http.StatusOK, runResponse{Status: "already-running"})
		return
	}

	var req runRequest
	if err := decodeJSONBody(r, &req); err != nil && err.Error() != "EOF" {
		// Body is optional (default args = "").
		writeWorkflowsError(w, http.StatusBadRequest, err)
		return
	}

	runID := newRunID()
	// Cancellable context created here so the run store can kill the opencode
	// process via Stop. The cancel func is stored in the run record.
	ctx, cancel := context.WithCancel(context.Background())
	if _, err := a.runs.start(name, runID, cancel); err != nil {
		cancel()
		// Lost the race with another request; report already-running.
		writeJSON(w, http.StatusOK, runResponse{Status: "already-running", RunID: runID})
		return
	}

	// Export first so the orchestrator/agents/command reflect the saved workflow.
	// Dry-run is not useful here: the opencode CLI reads the files from disk, so
	// we must actually write them.
	if _, err := a.exporter.Apply(wf); err != nil {
		cancel()
		a.runs.finish(name, 1, fmt.Errorf("export failed: %w", err))
		writeWorkflowsError(w, http.StatusInternalServerError, err)
		return
	}

	go a.runWorkflow(ctx, wf, req, runID)

	writeJSON(w, http.StatusAccepted, runResponse{Status: "started", RunID: runID})
}

// handleStop cancels an in-progress workflow run. The opencode process receives
// the cancellation via its context (SIGKILL) and the run is marked done.
func (a *workflowsAPI) handleStop(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if a.runs.cancel(name) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "stopped", "workflow": name})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "not-running", "workflow": name})
}

// runWorkflow is the background worker: spawns opencode, streams output to the
// hub, and records the result in the run store when it exits.
func (a *workflowsAPI) runWorkflow(ctx context.Context, wf *workflows.Workflow, req runRequest, runID string) {
	hub := a.hub
	hub.broadcastEvent(eventRunStarted, RunStartedEvent{Workflow: wf.Name, RunID: runID})

	exitCode, runErr := a.spawnOpencode(ctx, wf, req, runID)

	a.runs.finish(wf.Name, exitCode, runErr)
	var errMsg string
	if runErr != nil {
		errMsg = runErr.Error()
	}
	hub.broadcastEvent(eventRunDone, RunDoneEvent{
		Workflow: wf.Name,
		RunID:    runID,
		ExitCode: exitCode,
		Error:    errMsg,
	})
}

// spawnOpencode launches `opencode run --agent <name>-orchestrator [model] args`
// under a PTY so the agent flushes output line-by-line (opencode block-buffers
// when stdout is not a TTY, which would starve the live panel). The PTY merges
// stdout+stderr into one stream, so we read it as "stdout". Returns the exit
// code (0 on success) and any error.
//
// ctx is the caller's cancellable context: cancelling it (via Stop) kills the
// opencode process. A 30m timeout is layered on top so a forgotten run can't
// run forever.
func (a *workflowsAPI) spawnOpencode(ctx context.Context, wf *workflows.Workflow, req runRequest, runID string) (int, error) {
	opencodePath, err := missions.DetectOpencode()
	if err != nil {
		return 1, fmt.Errorf("opencode is not available: %w", err)
	}

	orchestrator := wf.Name + "-orchestrator"
	args := []string{"run", "--agent", orchestrator}
	if strings.TrimSpace(req.Model) != "" {
		args = append(args, "--model", req.Model)
	}
	// The orchestrator prompt forwards $ARGUMENTS; passing it as the task text
	// makes the args available to the workflow.
	task := strings.TrimSpace(req.Args)
	if task == "" {
		task = "Run the workflow."
	}
	args = append(args, task)

	// Layer a generous timeout on top of the caller's cancel context: workflows
	// can run long, but a forgotten run shouldn't run forever.
	ctx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, opencodePath, args...)

	// Start under a PTY so opencode flushes as it works instead of dumping the
	// whole buffer on exit. pty.StartWithSize attaches cmd to a new PTY and
	// returns the PTY's master end as the combined stdout+stderr reader.
	ptmx, err := pty.Start(cmd)
	if err != nil {
		return 1, fmt.Errorf("start opencode (pty): %w", err)
	}
	// Closing the master end after the process exits avoids a goroutine leak.
	defer func() { _ = ptmx.Close() }()

	// Stream the PTY output. Each line is broadcast as a run_output event and
	// appended to the run record.
	done := make(chan struct{}, 1)
	go a.streamPipe(wf.Name, runID, "stdout", ptmx, done)
	<-done

	waitErr := cmd.Wait()
	exitCode := 0
	if waitErr != nil {
		var exitErr *exec.ExitError
		if errors.As(waitErr, &exitErr) {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = 1
		}
	}
	return exitCode, nil
}

// streamPipe reads one pipe line by line and broadcasts/appends each line.
// ANSI escape sequences (colors, cursor moves) are stripped because the PTY
// makes the process emit terminal control codes.
func (a *workflowsAPI) streamPipe(workflow, runID, stream string, r io.Reader, done chan<- struct{}) {
	defer func() { done <- struct{}{} }()
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024) // allow long lines
	for scanner.Scan() {
		line := stripANSI(scanner.Text())
		if strings.TrimSpace(line) == "" {
			continue
		}
		chunk := line + "\n"
		a.runs.appendOutput(workflow, chunk)
		a.hub.broadcastEvent(eventRunOutput, RunOutputEvent{
			Workflow: workflow,
			RunID:    runID,
			Stream:   stream,
			Text:     line,
		})
	}
}

// stripANSI removes terminal escape codes from a line of PTY output. Reuses
// the package-level ansiRe defined in kanban_projector.go.
func stripANSI(s string) string {
	return ansiRe.ReplaceAllString(s, "")
}

// handleWorkflowWS upgrades the connection and registers it with the workflows
// hub so the client receives run_output/run_done events. Mirrors missions'
// HandleWebSocket (handlers.go:1385) without the initial state push (workflows
// don't need one — the UI subscribes after kicking off a run).
func (a *workflowsAPI) handleWorkflowWS(w http.ResponseWriter, r *http.Request) {
	conn, err := wfUpgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	client := &wsClient{hub: a.hub, conn: conn, send: make(chan []byte, 256)}
	a.hub.register(client)
	go client.writePump()
	client.readPump()
}

// newRunID returns a short hex id for a run (8 chars, enough to disambiguate).
func newRunID() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
