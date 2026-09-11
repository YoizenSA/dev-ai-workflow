package control

import (
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/envprofile"
)

// chain wraps the control server's mux in the middleware every route shares.
// It used to live in the tools API and cover only the proxied tool routes;
// now that all routes are registered on one mux, the whole API gets the same
// treatment — including the routes that previously had no panic recovery.
//
// Nothing wraps the ResponseWriter on purpose: the real http.ResponseWriter
// reaches every handler, so Flush, Hijack and Unwrap work natively. (An
// earlier 405-rewriting wrapper made handlers' `w.(http.Flusher)`
// assertions fail unless Flush was hand-forwarded here.)
func chain(next http.Handler) http.Handler {
	return recoveryMiddleware(profileScopeMiddleware(next))
}

// profileScopeMiddleware runs scoped settings requests inside one profile's
// env sandbox, so the Settings UI edits either the global config or exactly
// one environment. Only the Settings surface is scopable: /api/config/* and
// /api/agents-md. Everything else (workflows, evals, envs, version, tools,
// MCP store...) passes through untouched, as does the /api/events websocket,
// whose connection outlives the handler return that restores the env.
// Scoped requests serialize on the sandbox mutex; long applies must shell
// out instead (see envApplyRunner). Unknown profiles are 404,
// reserved/invalid names are 400.
func profileScopeMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !isScopableSettingsPath(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}
		name := strings.TrimSpace(r.URL.Query().Get("profile"))
		if name == "" {
			next.ServeHTTP(w, r)
			return
		}
		p, err := envprofile.Get(name)
		if err != nil {
			if strings.Contains(err.Error(), "reserved by a ywai command") ||
				strings.Contains(err.Error(), "invalid profile name") {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			} else {
				writeJSON(w, http.StatusNotFound, map[string]string{"error": "unknown profile " + strconv.Quote(name)})
			}
			return
		}
		if err := envprofile.WithProfileEnv(p, func() error {
			next.ServeHTTP(w, r)
			return nil
		}); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
	})
}

// isScopableSettingsPath reports whether a path belongs to the Settings UI
// surface (the whole /api/config/* tree plus AGENTS.md editing) or the
// Workflow Studio surface (/api/workflows*), which also renders per profile:
// the store is shared (D3) but the exporter + skills/MCP catalogs resolve
// inside the environment via OPENCODE_CONFIG_DIR.
func isScopableSettingsPath(path string) bool {
	return strings.HasPrefix(path, "/api/config/") || path == "/api/agents-md" ||
		strings.HasPrefix(path, "/api/workflows")
}

// recoveryMiddleware turns a panicking handler into a 500 JSON response
// instead of a dropped connection.
func recoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				log.Printf("PANIC recovered: %v", rec)
				writeJSON(w, http.StatusInternalServerError, map[string]string{
					"error": "internal server error",
				})
			}
		}()
		next.ServeHTTP(w, r)
	})
}
