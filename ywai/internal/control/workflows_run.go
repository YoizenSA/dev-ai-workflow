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
	"regexp"
	"strings"
	"time"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/host"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/workflows"
	"github.com/creack/pty"
)

// runRequest is the body of POST /api/workflows/{name}/run.
type runRequest struct {
	Args  string `json:"args"`            // forwarded to the orchestrator as $ARGUMENTS
	Model string `json:"model,omitempty"` // optional model override
	// Host selects the runtime: opencode (default), pi, omp.
	Host string `json:"host,omitempty"`
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

	// Export to the selected host so the runtime reads current agents from disk.
	hostID, err := host.ParseID(req.Host)
	if err != nil {
		cancel()
		a.runs.finish(name, 1, err)
		writeWorkflowsError(w, http.StatusBadRequest, err)
		return
	}
	req.Host = string(hostID)
	exporter := workflows.NewExporterForTarget(string(hostID))
	if _, err := exporter.Apply(wf); err != nil {
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

// handleInput sends user text to the running workflow's PTY stdin.
func (a *workflowsAPI) handleInput(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	var req struct {
		Text string `json:"text"`
	}
	if err := decodeJSONBody(r, &req); err != nil || strings.TrimSpace(req.Text) == "" {
		writeWorkflowsError(w, http.StatusBadRequest, fmt.Errorf("text is required"))
		return
	}
	if err := a.runs.writeInput(name, req.Text); err != nil {
		writeWorkflowsError(w, http.StatusConflict, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "sent"})
}

// runWorkflow is the background worker: spawns the selected host CLI, streams
// output to the hub, and records the result in the run store when it exits.
func (a *workflowsAPI) runWorkflow(ctx context.Context, wf *workflows.Workflow, req runRequest, runID string) {
	hub := a.hub
	hub.broadcastEvent(eventRunStarted, RunStartedEvent{Workflow: wf.Name, RunID: runID})

	exitCode, runErr := a.spawnHostRun(ctx, wf, req, runID)

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

// spawnHostRun launches the host CLI (opencode / pi / omp) under a PTY so output
// flushes line-by-line. Host defaults to opencode.
func (a *workflowsAPI) spawnHostRun(ctx context.Context, wf *workflows.Workflow, req runRequest, runID string) (int, error) {
	hostID, err := host.ParseID(req.Host)
	if err != nil {
		return 1, err
	}
	snap := host.Snapshot(hostID)
	if !snap.WorkflowRun {
		return 1, fmt.Errorf("host %s does not support workflow run from the control UI", hostID)
	}

	orchestrator := wf.Name + "-orchestrator"
	task := strings.TrimSpace(req.Args)
	if task == "" {
		task = "Run the workflow."
	}

	// Layer a generous timeout on top of the caller's cancel context.
	ctx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()

	cmd, err := host.Command(ctx, host.RunSpec{
		Host:   hostID,
		Agent:  orchestrator,
		Model:  req.Model,
		Prompt: task,
	})
	if err != nil {
		return 1, err
	}

	// Start under a PTY so the agent flushes as it works instead of dumping the
	// whole buffer on exit.
	ptmx, err := pty.Start(cmd)
	if err != nil {
		return 1, fmt.Errorf("start %s (pty): %w", hostID, err)
	}
	// Store ptmx in the run record so the user can send input during execution.
	a.runs.setPtmx(wf.Name, ptmx)
	// Closing the master end after the process exits avoids a goroutine leak.
	defer func() {
		a.runs.setPtmx(wf.Name, nil)
		_ = ptmx.Close()
	}()

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

// ansiRe matches ANSI escape sequences in PTY output.
var ansiRe = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

// stripANSI removes terminal escape codes from a line of PTY output.
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
