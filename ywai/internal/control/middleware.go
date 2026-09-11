package control

import (
	"log"
	"net/http"
)

// chain wraps the control server's mux in the middleware every route shares.
// It used to live in the tools API and cover only the proxied tool routes;
// now that all routes are registered on one mux, the whole API gets the same
// treatment — including the routes that previously had no panic recovery.
//
// Nothing wraps the ResponseWriter on purpose: the real http.ResponseWriter
// reaches every handler, so Flush, Hijack and Unwrap work natively. (An
// earlier 405-rewriting wrapper made the chat proxy's `w.(http.Flusher)`
// assertion fail unless Flush was hand-forwarded here.)
func chain(next http.Handler) http.Handler {
	return recoveryMiddleware(next)
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
