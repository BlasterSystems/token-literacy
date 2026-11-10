// Package server implements the loopback-only HTTP surface: OTLP ingest,
// health, query APIs, and (from M3) the dashboard.
package server

import (
	"encoding/json"
	"errors"
	"io/fs"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/BlasterSystems/token-literacy/internal/ingest/otlp"
	"github.com/BlasterSystems/token-literacy/internal/ledger"
	"github.com/BlasterSystems/token-literacy/internal/pricing"
)

// Info is the immutable runtime metadata exposed by the server.
type Info struct {
	Version string
	Started time.Time
	DBPath  string
	// CodexDir is the watched sessions directory; CodexDirExists reports
	// whether it was present at startup (dashboard empty states).
	CodexDir       string
	CodexDirExists bool
	// WSL marks a Windows-Subsystem-for-Linux host (loopback caveat banner).
	WSL bool
}

// Server serves the collector over loopback HTTP.
type Server struct {
	info     Info
	db       *ledger.DB
	override pricing.Override

	// metricMu serializes the cumulative-metric read-modify-write:
	// loadMetricState → MetricCalls → saveMetricState must be atomic or two
	// overlapping requests double-count or lose deltas.
	metricMu sync.Mutex
}

// effectiveOverride layers DB-cached model rates (OpenRouter snapshots and
// manual edits) under the pricing.json override. Queried per ingest request:
// the rates table is tiny and rates changes apply without a restart. On a
// DB error the config override alone is used.
func (s *Server) effectiveOverride() pricing.Override {
	if s.db == nil {
		return s.override
	}
	rates, err := s.db.ModelRates()
	if err != nil || len(rates) == 0 {
		return s.override
	}
	return pricing.Merge(RatesOverride(rates), s.override)
}

// RatesOverride converts cached DB rates to a pricing override keyed by the
// stored model prefix. Manual rows are applied after OpenRouter rows so an
// edited rate wins on the same key.
func RatesOverride(rates []ledger.ModelRate) pricing.Override {
	ov := pricing.Override{}
	for _, pass := range []string{"openrouter", "manual"} {
		for _, r := range rates {
			if r.Source != pass || r.Model == "" {
				continue
			}
			ov[r.Model] = pricing.Price{Input: r.Input, CacheRead: r.CacheRead, CacheWrite: r.CacheWrite, Output: r.Output}
		}
	}
	return ov
}

// New builds the HTTP handler. db may be nil only in tests that exercise
// read-only metadata routes; ingest requires a db.
func New(info Info, db *ledger.DB, override pricing.Override) http.Handler {
	s := &Server{info: info, db: db, override: override}
	mux := http.NewServeMux()

	mux.HandleFunc("POST /v1/traces", s.handleOTLP(otlp.KindTraces))
	mux.HandleFunc("POST /v1/metrics", s.handleOTLP(otlp.KindMetrics))
	mux.HandleFunc("POST /v1/logs", s.handleOTLP(otlp.KindLogs))
	mux.HandleFunc("GET /health", s.handleHealth)
	mux.HandleFunc("GET /api/summary", s.guardDB(s.handleSummary))
	mux.HandleFunc("GET /api/rows", s.guardDB(s.handleRows))
	mux.HandleFunc("GET /api/sessions", s.guardDB(s.handleSessions))
	mux.HandleFunc("GET /api/rates", s.guardDB(s.handleRatesGet))
	mux.HandleFunc("PUT /api/rates", s.guardDB(s.handleRatesPut))
	mux.HandleFunc("DELETE /api/rates", s.guardDB(s.handleRatesDelete))
	mux.HandleFunc("GET /export.csv", s.guardDB(s.handleExportCSV))
	mux.HandleFunc("GET /export.json", s.guardDB(s.handleExportJSON))
	// Embedded DS fonts — the only static assets; nothing is fetched remotely.
	fontsRoot, _ := fs.Sub(fontsFS, "assets")
	mux.Handle("GET /assets/", http.StripPrefix("/assets/", http.FileServerFS(fontsRoot)))
	mux.HandleFunc("GET /", s.guardDB(s.handleDashboard))

	return loopbackGuard(mux)
}

// loopbackGuard rejects requests whose source address or Host header is not
// loopback (DNS-rebinding defense; ADR-0003).
func loopbackGuard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !isLoopbackHost(r.Host) || !isLoopbackAddr(r.RemoteAddr) {
			http.Error(w, "loopback only", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func isLoopbackAddr(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		host = addr
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func isLoopbackHost(hostport string) bool {
	host := hostport
	if h, _, err := net.SplitHostPort(hostport); err == nil {
		host = h
	}
	host = strings.TrimSuffix(host, ".")
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// handleOTLP accepts OTLP/HTTP JSON for one signal. Malformed payloads are
// rejected with 400 and logged; the server keeps running. Rows that fail
// their harness invariant are logged and skipped; the request still succeeds
// for the remaining rows.
func (s *Server) handleOTLP(kind otlp.Kind) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.db == nil {
			http.Error(w, "ledger unavailable", http.StatusServiceUnavailable)
			return
		}
		body, err := otlp.ReadBody(r.Body, r.Header.Get("Content-Encoding"))
		if err != nil {
			log.Printf("otlp %s: %v", kind, err)
			http.Error(w, "unreadable body", http.StatusBadRequest)
			return
		}

		var calls []ledger.Call
		var skipped []otlp.Skipped
		var pendingState *otlp.MetricState
		// The cumulative-metric state is shared: serialize the whole
		// load→convert→insert→save sequence for metrics requests.
		if kind == otlp.KindMetrics {
			s.metricMu.Lock()
			defer s.metricMu.Unlock()
		}
		switch kind {
		case otlp.KindTraces:
			calls, skipped, err = otlp.SpanCalls(body, s.effectiveOverride())
		case otlp.KindMetrics:
			state, loadErr := s.loadMetricState()
			if loadErr != nil {
				log.Printf("otlp metrics: load state: %v", loadErr)
			}
			calls, skipped, err = otlp.MetricCalls(body, state, s.effectiveOverride())
			pendingState = &state
		case otlp.KindLogs:
			// Accepted and discarded: logs never touch the ledger.
			writeOTLPOK(w)
			return
		}
		if err != nil {
			log.Printf("otlp %s: malformed payload rejected: %v", kind, err)
			http.Error(w, "malformed payload", http.StatusBadRequest)
			return
		}
		for _, sk := range skipped {
			log.Printf("otlp %s: skipped %s: %s", kind, sk.Kind, sk.Reason)
		}
		inserted, rejected := 0, 0
		for _, c := range calls {
			ok, err := s.db.InsertCall(c)
			if err != nil {
				rejected++
				if errors.Is(err, ledger.ErrInvalidCall) {
					log.Printf("ingest: invariant rejection id=%s: %v", c.ID, err)
				} else {
					log.Printf("ingest: insert failed id=%s: %v", c.ID, err)
				}
				continue
			}
			if ok {
				inserted++
			}
		}
		// Persist the cumulative state only after the rows it produced are
		// stored: state-then-crash would otherwise lose the delta.
		if pendingState != nil {
			if saveErr := s.saveMetricState(*pendingState); saveErr != nil {
				log.Printf("otlp metrics: persist state: %v", saveErr)
			}
		}
		if inserted > 0 || rejected > 0 {
			log.Printf("ingest %s: inserted=%d rejected=%d skipped=%d", kind, inserted, rejected, len(skipped))
		}
		writeOTLPOK(w)
	}
}

func writeOTLPOK(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte("{}"))
}

const metricStateKey = "claude_metric_series_state"

func (s *Server) loadMetricState() (otlp.MetricState, error) {
	raw, err := s.db.GetMeta(metricStateKey)
	if err != nil {
		return otlp.MetricState{}, err
	}
	if raw == "" {
		return otlp.MetricState{}, nil
	}
	var st otlp.MetricState
	if err := json.Unmarshal([]byte(raw), &st); err != nil {
		return otlp.MetricState{}, nil // start fresh on corrupt state
	}
	return st, nil
}

func (s *Server) saveMetricState(st otlp.MetricState) error {
	if len(st) == 0 {
		return nil
	}
	b, err := json.Marshal(st)
	if err != nil {
		return err
	}
	return s.db.SetMeta(metricStateKey, string(b))
}
