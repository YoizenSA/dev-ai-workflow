package control

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func TestGlanceDoneIsOneLine(t *testing.T) {
	title, body, err := Glance(WristNotice{Kind: "done", Agent: "dev", Label: "fix login"})
	if err != nil {
		t.Fatal(err)
	}
	if title != "Agent done" {
		t.Errorf("title = %q", title)
	}
	if body != "dev · fix login" {
		t.Errorf("body = %q", body)
	}
}

func TestGlanceFailedKeepsTheReason(t *testing.T) {
	title, body, err := Glance(WristNotice{
		Kind: "failed", Agent: "qa", Label: "review", Detail: "model key missing",
	})
	if err != nil {
		t.Fatal(err)
	}
	if title != "Agent failed" || body != "qa · review\nmodel key missing" {
		t.Errorf("got %q / %q", title, body)
	}
}

func TestGlanceDoneNamesTheWork(t *testing.T) {
	title, body, err := Glance(WristNotice{
		Kind: "done", Agent: "dev", Label: "Fix login", Detail: "Add the password check on the form",
	})
	if err != nil {
		t.Fatal(err)
	}
	if title != "Agent done" || body != "dev · Fix login\nAdd the password check on the form" {
		t.Errorf("got %q / %q", title, body)
	}
}

func TestGlanceWaitingNamesTheAsk(t *testing.T) {
	title, body, err := Glance(WristNotice{Kind: "waiting", Agent: "dev", Label: "Allow bash?"})
	if err != nil {
		t.Fatal(err)
	}
	if title != "Agent waiting" || body != "dev · Allow bash?" {
		t.Errorf("got %q / %q", title, body)
	}
}

func TestGlanceCountsSiblingsStillRunning(t *testing.T) {
	_, body, err := Glance(WristNotice{Kind: "done", Agent: "dev", Label: "fix login", Remaining: 2})
	if err != nil {
		t.Fatal(err)
	}
	if body != "dev · fix login · 2 still running" {
		t.Errorf("body = %q", body)
	}
}

func TestGlanceRejectsNoise(t *testing.T) {
	if _, _, err := Glance(WristNotice{Kind: "idle", Agent: "dev"}); err == nil {
		t.Fatal("idle must not become a wrist notice")
	}
	if _, _, err := Glance(WristNotice{Kind: "done", Agent: "  \n"}); err == nil {
		t.Fatal("blank agent must not become a wrist notice")
	}
}

func TestGlanceCollapsesToOneLine(t *testing.T) {
	_, body, err := Glance(WristNotice{Kind: "done", Agent: "dev", Label: "fix\nlogin\tnow"})
	if err != nil {
		t.Fatal(err)
	}
	if body != "dev · fix login now" {
		t.Errorf("body = %q", body)
	}
}

type recordedPush struct {
	title, body string
	calls       int
}

func (r *recordedPush) Send(title, body string) error {
	r.calls++
	r.title, r.body = title, body
	return nil
}

func (r *recordedPush) PublicKey() string { return "" }

func TestWristSettingsKeepURLAndSwitch(t *testing.T) {
	path := filepath.Join(t.TempDir(), "wrist.json")
	gate := loadWristGate(path)
	if gate.Enabled() {
		t.Fatal("missing file must stay off")
	}
	if gate.URL() != "http://127.0.0.1:5768" {
		t.Fatalf("default url = %q", gate.URL())
	}
	if err := gate.Set(true, "http://127.0.0.1:9999/"); err != nil {
		t.Fatal(err)
	}
	again := loadWristGate(path)
	if !again.Enabled() || again.URL() != "http://127.0.0.1:9999" {
		t.Fatalf("reloaded enabled=%v url=%q", again.Enabled(), again.URL())
	}
	if err := gate.Set(false, "ftp://files.example"); err == nil {
		t.Fatal("a non-http url must be rejected")
	}
}

func TestNotifyStaysQuietUntilEnabled(t *testing.T) {
	rec := &recordedPush{}
	api := &PushAPI{
		sender: rec,
		store:  &PushStore{subs: []PushSubscription{{Endpoint: "https://push.example/sub"}}},
	}
	req := httptest.NewRequest(http.MethodPost, "/api/push/notify", strings.NewReader(
		`{"kind":"done","agent":"dev","label":"fix login"}`,
	))
	req.RemoteAddr = "127.0.0.1:4321"
	rr := httptest.NewRecorder()
	api.handleNotify(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body %s", rr.Code, rr.Body.String())
	}
	if rec.calls != 0 {
		t.Fatal("a glance must not send until the watch switch is on")
	}
	if !strings.Contains(rr.Body.String(), `"disabled"`) {
		t.Fatalf("body = %s", rr.Body.String())
	}
}

func TestNotifyFromLoopbackDeliversTheGlance(t *testing.T) {
	rec := &recordedPush{}
	api := &PushAPI{
		sender: rec,
		store:  &PushStore{subs: []PushSubscription{{Endpoint: "https://push.example/sub"}}},
		wrist:  &wristGate{enabled: true},
	}
	req := httptest.NewRequest(http.MethodPost, "/api/push/notify", strings.NewReader(
		`{"kind":"done","agent":"dev","label":"fix login"}`,
	))
	req.RemoteAddr = "127.0.0.1:4321"
	rr := httptest.NewRecorder()
	api.handleNotify(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body %s", rr.Code, rr.Body.String())
	}
	if rec.calls != 1 || rec.title != "Agent done" || rec.body != "dev · fix login" {
		t.Fatalf("sent %+v", rec)
	}
}

func TestNotifyFromTheNetworkDoesNotBuzz(t *testing.T) {
	rec := &recordedPush{}
	api := &PushAPI{
		sender: rec,
		store:  &PushStore{subs: []PushSubscription{{Endpoint: "https://push.example/sub"}}},
	}
	req := httptest.NewRequest(http.MethodPost, "/api/push/notify", strings.NewReader(
		`{"kind":"done","agent":"dev","label":"fix login"}`,
	))
	req.RemoteAddr = "192.168.1.20:4321"
	rr := httptest.NewRecorder()
	api.handleNotify(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("status = %d", rr.Code)
	}
	if rec.calls != 0 {
		t.Fatal("a non-loopback request must not send")
	}
}
