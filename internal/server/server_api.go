package server

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/BlasterSystems/token-literacy/internal/ledger"
	"github.com/BlasterSystems/token-literacy/internal/pricing"
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
	estUSD, unpriced, usedDefault, err := s.costEstimate(queryFilter(r))
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
		"cost_estimate_usd":     estUSD,
		"unpriced_calls":        unpriced,
		"unpriced_used_default": usedDefault,
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

// handleRatesGet lists cached model rates plus the default price info.
func (s *Server) handleRatesGet(w http.ResponseWriter, r *http.Request) {
	rates, err := s.db.ModelRates()
	if err != nil {
		httpError(w, err)
		return
	}
	if rates == nil {
		rates = []ledger.ModelRate{}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"rates":   rates,
		"default": map[string]any{"model": pricing.DefaultModel, "price": pricing.DefaultPrice},
	})
}

// handleRatesPut upserts one rate. The body is a ledger.ModelRate JSON;
// model and non-negative rates are required. An empty source defaults to
// "manual" (a user edit); "openrouter" is written by the dashboard when
// persisting a catalog pick.
func (s *Server) handleRatesPut(w http.ResponseWriter, r *http.Request) {
	var rate ledger.ModelRate
	if err := json.NewDecoder(r.Body).Decode(&rate); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}
	if rate.Model == "" {
		http.Error(w, "model is required", http.StatusBadRequest)
		return
	}
	if rate.Input < 0 || rate.CacheRead < 0 || rate.CacheWrite < 0 || rate.Output < 0 {
		http.Error(w, "rates must be non-negative", http.StatusBadRequest)
		return
	}
	if rate.Source != "openrouter" && rate.Source != "manual" {
		rate.Source = "manual"
	}
	rate.UpdatedAtMS = time.Now().UnixMilli()
	if err := s.db.UpsertModelRate(rate); err != nil {
		httpError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(rate)
}

// handleRatesDelete removes one cached rate: /api/rates?model=<key>.
func (s *Server) handleRatesDelete(w http.ResponseWriter, r *http.Request) {
	model := r.URL.Query().Get("model")
	if model == "" {
		http.Error(w, "model is required", http.StatusBadRequest)
		return
	}
	if err := s.db.DeleteModelRate(model); err != nil {
		httpError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]bool{"deleted": true})
}
