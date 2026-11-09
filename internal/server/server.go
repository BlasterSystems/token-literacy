package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Info is the immutable runtime metadata exposed by the walking skeleton.
type Info struct {
	Version string
	Started time.Time
	DBPath  string
}

// NewHandler returns the loopback-only HTTP handler. Ingest and dashboard
// routes land in M1 and M3; /health and / exist now so the process is
// smoke-testable across platforms.
func NewHandler(info Info) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status":         "ok",
			"version":        info.Version,
			"uptime_seconds": int64(time.Since(info.Started).Seconds()),
			"db":             info.DBPath,
		})
	})

	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		fmt.Fprintln(w, "token-literacy collector")
		fmt.Fprintln(w)
		fmt.Fprintln(w, "health: /health")
		fmt.Fprintln(w, "dashboard: planned in M3; see docs/PLAN.md")
	})

	return mux
}
