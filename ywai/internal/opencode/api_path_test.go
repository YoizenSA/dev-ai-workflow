package opencode

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/agent"
)

// v2 serves its web UI from the bare routes, so /agent answers 200 with the
// SPA's HTML shell. Hitting it looks healthy and then fails to decode, which is
// why the path — not just the parsing — has to follow the flavor.
func TestServerClient_AgentPathFollowsFlavor(t *testing.T) {
	for _, tc := range []struct{ flavor, want string }{
		{"v1", "/agent"},
		{"v2", "/api/agent"},
	} {
		t.Run(tc.flavor, func(t *testing.T) {
			t.Setenv(agent.OpenCodeOverrideEnv, tc.flavor)

			var got string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				got = r.URL.Path
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`[]`))
			}))
			defer srv.Close()

			c := NewServerClient(srv.URL)
			c.useCLI = false
			if _, err := c.ListAgents(context.Background()); err != nil {
				t.Fatalf("ListAgents: %v", err)
			}
			if got != tc.want {
				t.Errorf("requested %q, want %q", got, tc.want)
			}
		})
	}
}

func TestServerClient_ProviderPathFollowsFlavor(t *testing.T) {
	for _, tc := range []struct{ flavor, want string }{
		{"v1", "/provider"},
		{"v2", "/api/provider"},
	} {
		t.Run(tc.flavor, func(t *testing.T) {
			t.Setenv(agent.OpenCodeOverrideEnv, tc.flavor)

			var got string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				got = r.URL.Path
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{}`))
			}))
			defer srv.Close()

			c := NewServerClient(srv.URL)
			c.useCLI = false
			_ = c.getConnectedProviders(context.Background())
			if got != tc.want {
				t.Errorf("requested %q, want %q", got, tc.want)
			}
		})
	}
}
