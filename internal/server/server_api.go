package server

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/BlasterSystems/token-literacy/internal/ledger"
)

// guardDB returns 503 when the ledger is not open.
func (s *Server) guardDB(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.db == nil {
			http.Error(w, "ledger unavailable", http.StatusServiceUnavailable)
			return
		}
		next(w, r)
	}
}

// handleHealth reports process, ledger, and cursor state.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	resp := map[string]any{
		"status":         "ok",
		"version":        s.info.Version,
		"uptime_seconds": int64(time.Since(s.info.Started).Seconds()),
		"db":             s.info.DBPath,
	}
	if s.db != nil {
		total, byHarness, err := s.db.Counts()
		if err == nil {
			resp["calls_total"] = total
			resp["calls_by_harness"] = byHarness
			for harness, label := range map[string]string{
				"claude-code": "last_claude_span_ms",
				"codex":       "last_codex_row_ms",
			} {
				if last, err := s.db.LastSourceTime(harness); err == nil {
					resp[label] = last
					if last > 0 {
						resp[label+"_age_seconds"] = int64(time.Since(time.UnixMilli(last)).Seconds())
					}
				}
			}
			if agg, err := s.db.AggregateCount("claude-code"); err == nil {
				resp["claude_aggregate_only"] = agg > 0
			}
		}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// queryFilter parses since/harness/session/limit/offset from the request.
func queryFilter(r *http.Request) ledger.RowsFilter {
	q := r.URL.Query()
	return ledger.RowsFilter{
		SinceMS: ledger.ParseSince(q.Get("since")),
		Harness: q.Get("harness"),
		Session: q.Get("session"),
		Limit:   atoiDefault(q.Get("limit"), 0),
		Offset:  atoiDefault(q.Get("offset"), 0),
		Asc:     q.Get("order") == "asc",
	}
}

func atoiDefault(s string, def int) int {
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return n
}

func (s *Server) handleSummary(w http.ResponseWriter, r *http.Request) {
	sum, err := s.db.Summary(queryFilter(r))
	if err != nil {
		httpError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"calls":                 sum.Calls,
		"input_fresh":           sum.InputFresh,
		"cache_read":            sum.CacheRead,
		"cache_write":           sum.CacheWrite,
		"output":                sum.Output,
		"reasoning":             sum.Reasoning,
		"prompt_total":          sum.PromptTotal,
		"total_tokens":          sum.TotalTokens,
		"cost_usd":              sum.CostUSD,
		"costed_calls":          sum.CostedCalls,
		"cache_hit_rate":        sum.HitRate(),
		"fresh_share":           sum.FreshShare(),
		"context_tax":           sum.ContextTax(),
		"cost_per_output_token": sum.CostPerOutputToken(),
	})
}

func (s *Server) handleRows(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.Rows(queryFilter(r))
	if err != nil {
		httpError(w, err)
		return
	}
	if rows == nil {
		rows = []ledger.Call{}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(rows)
}

func (s *Server) handleSessions(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	sessions, err := s.db.Sessions(ledger.ParseSince(q.Get("since")), q.Get("harness"))
	if err != nil {
		httpError(w, err)
		return
	}
	if sessions == nil {
		sessions = []ledger.SessionRow{}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(sessions)
}

func httpError(w http.ResponseWriter, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusInternalServerError)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
}
