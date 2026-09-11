package control

import (
	"net/http"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/wshub"
	"github.com/gorilla/websocket"
)

// Workflow events streamed to the UI during a run. Each message is
// {"type": <event>, "payload": <data>}.
const (
	eventRunStarted = "workflow_run_started" // payload: RunStartedEvent
	eventRunOutput  = "workflow_run_output"  // payload: RunOutputEvent (one line/chunk)
	eventRunDone    = "workflow_run_done"    // payload: RunDoneEvent
)

// RunStartedEvent is sent when a workflow run begins.
type RunStartedEvent struct {
	Workflow string `json:"workflow"`
	RunID    string `json:"runId"`
}

// RunOutputEvent carries one chunk of the orchestrator's stdout/stderr.
type RunOutputEvent struct {
	Workflow string `json:"workflow"`
	RunID    string `json:"runId"`
	Stream   string `json:"stream"` // "stdout" | "stderr"
	Text     string `json:"text"`
}

// RunDoneEvent is sent when a run finishes (success or failure).
type RunDoneEvent struct {
	Workflow string `json:"workflow"`
	RunID    string `json:"runId"`
	ExitCode int    `json:"exitCode"`
	Error    string `json:"error,omitempty"`
}

// wsHub fans workflow run events out to the Run panel's WebSocket clients.
// It wraps the shared wshub.Hub and keeps the lowercase broadcastEvent
// helper so existing call sites keep their spelling.
type wsHub struct {
	*wshub.Hub
}

func newWsHub() *wsHub {
	return &wsHub{Hub: wshub.New(wshub.Options{ReadLimit: 4096})}
}

// broadcastEvent sends one structured event to every connected client. It
// is nil-safe: a nil hub drops the event.
func (h *wsHub) broadcastEvent(eventType string, payload any) {
	if h == nil {
		return
	}
	h.Hub.BroadcastEvent(eventType, payload)
}

// serveWorkflowWS upgrades the request and attaches the connection to h
// for the lifetime of the WebSocket. Registered clients receive
// run_output/run_done events.
func serveWorkflowWS(h *wsHub, w http.ResponseWriter, r *http.Request) {
	conn, err := wfUpgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	client := wshub.NewClient(h.Hub, conn)
	h.Register(client)
	client.Serve()
}

// upgrader is the WebSocket upgrader for the workflows endpoint. Same limits as
// missions/web's upgrader.
var wfUpgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}
