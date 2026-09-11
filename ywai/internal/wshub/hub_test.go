package wshub

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// wsEvent mirrors the JSON envelope BroadcastEvent emits.
type wsEvent struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

// clientRecorder captures each accepted server-side *Client for tests.
type clientRecorder struct {
	mu      sync.Mutex
	clients []*Client
}

func (r *clientRecorder) add(c *Client) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.clients = append(r.clients, c)
}

func (r *clientRecorder) get(t *testing.T, i int) *Client {
	t.Helper()
	r.mu.Lock()
	defer r.mu.Unlock()
	if i >= len(r.clients) {
		t.Fatalf("client #%d not recorded (have %d)", i, len(r.clients))
	}
	return r.clients[i]
}

// newTestHubServer upgrades every request to a websocket and registers it
// with h. /clog connections are registered without pumps, so nobody drains
// their send queue; everything else is served until the client disconnects.
func newTestHubServer(t *testing.T, h *Hub) (*httptest.Server, *clientRecorder) {
	t.Helper()
	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	rec := &clientRecorder{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		c := NewClient(h, conn)
		rec.add(c)
		h.Register(c)
		if r.URL.Path != "/clog" {
			c.Serve() // blocks until the connection dies
		}
	}))
	t.Cleanup(srv.Close)
	return srv, rec
}

// dialPath opens a client websocket connection to the given server path.
func dialPath(t *testing.T, srv *httptest.Server, path string) *websocket.Conn {
	t.Helper()
	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(srv.URL, "http")+path, nil)
	if err != nil {
		t.Fatalf("dial %s%s: %v", srv.URL, path, err)
	}
	t.Cleanup(func() { conn.Close() })
	return conn
}

// readWSMessage reads one message, returning any error for timeout/close
// frame assertions.
func readWSMessage(t *testing.T, conn *websocket.Conn, timeout time.Duration) ([]byte, error) {
	t.Helper()
	if err := conn.SetReadDeadline(time.Now().Add(timeout)); err != nil {
		t.Fatalf("set read deadline: %v", err)
	}
	_, data, err := conn.ReadMessage()
	return data, err
}

// readEvent reads one broadcast envelope from conn.
func readEvent(t *testing.T, conn *websocket.Conn) wsEvent {
	t.Helper()
	data, err := readWSMessage(t, conn, 2*time.Second)
	if err != nil {
		t.Fatalf("read event: %v", err)
	}
	var ev wsEvent
	if err := json.Unmarshal(data, &ev); err != nil {
		t.Fatalf("decode event %q: %v", data, err)
	}
	return ev
}

// waitClientCount polls until the hub holds exactly want clients; register
// and unregister are asynchronous relative to the dialer.
func waitClientCount(t *testing.T, h *Hub, want int) {
	t.Helper()
	for deadline := time.Now().Add(2 * time.Second); time.Now().Before(deadline); {
		h.mu.RLock()
		got := len(h.clients)
		h.mu.RUnlock()
		if got == want {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("hub client count never reached %d", want)
}

func TestBroadcastEventDeliversEnvelope(t *testing.T) {
	h := New(Options{})
	srv, _ := newTestHubServer(t, h)
	a := dialPath(t, srv, "/")
	waitClientCount(t, h, 1)
	b := dialPath(t, srv, "/")
	waitClientCount(t, h, 2)

	payload := map[string]string{"file": "app.json"}
	wantPayload, _ := json.Marshal(payload) // map[string]string never fails
	h.BroadcastEvent("config.updated", payload)

	for name, conn := range map[string]*websocket.Conn{"a": a, "b": b} {
		ev := readEvent(t, conn)
		if ev.Type != "config.updated" {
			t.Errorf("%s: type = %q, want %q", name, ev.Type, "config.updated")
		}
		if string(ev.Payload) != string(wantPayload) {
			t.Errorf("%s: payload = %s, want %s", name, ev.Payload, wantPayload)
		}
	}
}

func TestBroadcastDropsFullBufferedClient(t *testing.T) {
	h := New(Options{})
	srv, rec := newTestHubServer(t, h)
	a := dialPath(t, srv, "/")
	waitClientCount(t, h, 1)
	b := dialPath(t, srv, "/clog") // registered, but no write pump drains it
	waitClientCount(t, h, 2)
	bc := rec.get(t, 1)

	// Fill b's send queue; the next broadcast must drop it without blocking.
	for i := 0; i < sendBuffer; i++ {
		bc.send <- []byte("filler")
	}
	h.BroadcastEvent("flood", nil)
	waitClientCount(t, h, 1) // b was dropped

	h.BroadcastEvent("after-drop", nil)
	if ev := readEvent(t, a); ev.Type != "flood" {
		t.Fatalf("survivor first event = %q, want %q", ev.Type, "flood")
	}
	if ev := readEvent(t, a); ev.Type != "after-drop" {
		t.Fatalf("survivor second event = %q, want %q", ev.Type, "after-drop")
	}
	if _, err := readWSMessage(t, b, 300*time.Millisecond); err == nil {
		t.Fatal("dropped client received a broadcast")
	}
}

func TestUnregisterStopsDelivery(t *testing.T) {
	h := New(Options{})
	srv, rec := newTestHubServer(t, h)
	a := dialPath(t, srv, "/")
	waitClientCount(t, h, 1)
	b := dialPath(t, srv, "/")
	waitClientCount(t, h, 2)

	h.Unregister(rec.get(t, 1))
	waitClientCount(t, h, 1)

	h.BroadcastEvent("post-unregister", nil)
	if ev := readEvent(t, a); ev.Type != "post-unregister" {
		t.Fatalf("a event type = %q, want %q", ev.Type, "post-unregister")
	}
	// b sees no broadcast after Unregister — nothing at all, or the close
	// frame from its closed send queue. a's receipt proves the hub already
	// processed this broadcast.
	if _, err := readWSMessage(t, b, 300*time.Millisecond); err == nil {
		t.Fatal("b received a broadcast after Unregister")
	}
}

func TestBroadcastEventNilAndBadPayloadSafe(t *testing.T) {
	t.Run("nil hub drops the event", func(t *testing.T) {
		var h *Hub
		h.BroadcastEvent("noop", nil) // must not panic
	})
	t.Run("unmarshalable payload is dropped", func(t *testing.T) {
		h := New(Options{})
		h.BroadcastEvent("noop", make(chan int)) // json.Marshal fails
	})
}

func TestReadLimitDropsOversizedClient(t *testing.T) {
	h := New(Options{ReadLimit: 64})
	srv, _ := newTestHubServer(t, h)
	bad := dialPath(t, srv, "/")
	waitClientCount(t, h, 1)
	good := dialPath(t, srv, "/")
	waitClientCount(t, h, 2)

	if err := bad.WriteMessage(websocket.TextMessage, bytes.Repeat([]byte("x"), 65)); err != nil {
		t.Fatalf("send oversized message: %v", err)
	}
	waitClientCount(t, h, 1) // read pump hit the limit and dropped bad

	if _, err := readWSMessage(t, bad, 2*time.Second); err == nil {
		t.Fatal("oversized client connection is still readable")
	}

	h.BroadcastEvent("post-limit", nil)
	if ev := readEvent(t, good); ev.Type != "post-limit" {
		t.Fatalf("good event type = %q, want %q", ev.Type, "post-limit")
	}
}
