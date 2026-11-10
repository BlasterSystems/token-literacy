package server

import (
	"encoding/csv"
	"encoding/json"
	"math"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/BlasterSystems/token-literacy/internal/ledger"
)

// seedTwenty inserts a deterministic 20-turn session.
func seedTwenty(t *testing.T, db *ledger.DB) {
	t.Helper()
	now := time.Now().UnixMilli()
	for i := range 20 {
		c := ledger.Call{
			ID: "codex:turn" + strconv.Itoa(i), Harness: "codex", SessionID: "s-twenty",
			RequestID: "turn" + strconv.Itoa(i), Model: "gpt-5.2",
			InputUncached: int64(500 + i*10), CacheRead: int64(20000 - i*100),
			Output: int64(100 + i), Source: "codex_rollout", StartedAt: now - int64((19-i))*60_000,
		}
		if _, err := db.InsertCall(c); err != nil {
			t.Fatal(err)
		}
	}
}

func TestDashboardRendersChartsAndTable(t *testing.T) {
	handler, db := newTestHandler(t)
	seedTwenty(t, db)

	rec := doRequest(t, handler, http.MethodGet, "/", nil, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("dashboard = %d", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{
		"Tokens", "Cache hit rate", "Fresh share", "Context tax",
		"API-equiv. cost", "Cost / output token", "Turns",
		"TokL - Token Literacy", "A BlasterSystems product",
		`id="bars"`, `id="rate"`, `"u":`, "s-twenty",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("dashboard missing %q", want)
		}
	}
	if strings.Contains(body, "http://") || strings.Contains(body, "https://") {
		// src/href to external hosts would violate the no-CDN rule; only the
		// export links reference the local server.
		if strings.Contains(body, `<script src=`) || strings.Contains(body, `<link`) {
			t.Error("external asset reference found")
		}
	}
}

func TestDashboardEmptyState(t *testing.T) {
	handler, _ := newTestHandler(t)
	rec := doRequest(t, handler, http.MethodGet, "/", nil, nil)
	body := rec.Body.String()
	if !strings.Contains(body, "No turns recorded yet") {
		t.Fatalf("empty state missing: %s", body)
	}
}

func TestDashboardAggregateOnlyBanner(t *testing.T) {
	handler, db := newTestHandler(t)
	sub := "aggregate"
	if _, err := db.InsertCall(ledger.Call{
		ID: "claude-code:agg:1", Harness: "claude-code", SessionID: "aggregate", RequestID: "agg:1",
		InputUncached: 100, Output: 10, Source: "claude_otel", AgentType: &sub,
		StartedAt: time.Now().UnixMilli(),
	}); err != nil {
		t.Fatal(err)
	}
	rec := doRequest(t, handler, http.MethodGet, "/", nil, nil)
	if !strings.Contains(rec.Body.String(), "aggregate-only") {
		t.Fatalf("aggregate banner missing: %s", rec.Body)
	}
}

func TestDashboardCodexDirBanner(t *testing.T) {
	db, err := ledger.Open(filepath.Join(t.TempDir(), "ledger.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	handler := New(Info{Version: "t", Started: time.Now(), CodexDir: "/missing/codex", CodexDirExists: false}, db, nil)
	rec := doRequest(t, handler, http.MethodGet, "/", nil, nil)
	if !strings.Contains(rec.Body.String(), "Codex directory not found") {
		t.Fatalf("codex banner missing: %s", rec.Body)
	}
}

func TestExportCSVMatchesRows(t *testing.T) {
	handler, db := newTestHandler(t)
	seedTwenty(t, db)

	rec := doRequest(t, handler, http.MethodGet, "/export.csv?harness=codex", nil, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("csv = %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/csv") {
		t.Fatalf("content-type = %s", ct)
	}
	cr := csv.NewReader(rec.Body)
	records, err := cr.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 21 {
		t.Fatalf("csv rows = %d, want 21 (header + 20)", len(records))
	}
	if records[0][0] != "id" || records[0][8] != "input_uncached" {
		t.Fatalf("header = %v", records[0])
	}

	// Every CSV number must equal /api/rows for the same filter.
	rec2 := doRequest(t, handler, http.MethodGet, "/api/rows?harness=codex&order=asc&limit=0", nil, nil)
	var rows []map[string]any
	if err := json.Unmarshal(rec2.Body.Bytes(), &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 20 {
		t.Fatalf("rows = %d", len(rows))
	}
	for i, row := range rows {
		csvID := records[i+1][0]
		if csvID != row["id"] {
			t.Fatalf("row %d: csv id %s != api id %v", i, csvID, row["id"])
		}
		csvU, _ := strconv.ParseInt(records[i+1][8], 10, 64)
		if csvU != int64(row["input_uncached"].(float64)) {
			t.Fatalf("row %d: csv U %d != api U %v", i, csvU, row["input_uncached"])
		}
		csvT, _ := strconv.ParseInt(records[i+1][14], 10, 64)
		if csvT != int64(row["total_tokens"].(float64)) {
			t.Fatalf("row %d: csv total %d != api total %v", i, csvT, row["total_tokens"])
		}
	}
}

func TestExportJSON(t *testing.T) {
	handler, db := newTestHandler(t)
	seedTwenty(t, db)
	rec := doRequest(t, handler, http.MethodGet, "/export.json", nil, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("json = %d", rec.Code)
	}
	var rows []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 20 {
		t.Fatalf("json rows = %d", len(rows))
	}
	if rows[0]["input_uncached"].(float64)+rows[0]["cache_read"].(float64) != rows[0]["prompt_total"].(float64) {
		t.Fatal("export must preserve the additive invariant")
	}
}

func TestChartRowsAscending(t *testing.T) {
	rows := []ledger.Call{
		{ID: "b", InputUncached: 30, CacheRead: 40, CacheWrite: 10, Output: 15, StartedAt: 200},
		{ID: "a", InputUncached: 10, CacheRead: 20, CacheWrite: 0, Output: 5, StartedAt: 100},
	}
	chart := chartRows(rows)
	if len(chart) != 2 {
		t.Fatalf("chart rows = %d", len(chart))
	}
	if chart[0].T != 100 || chart[1].T != 200 {
		t.Fatalf("not ascending: %+v", chart)
	}
	if chart[1].U != 30 || chart[1].R != 40 || chart[1].W != 10 || chart[1].O != 15 {
		t.Fatalf("token split wrong: %+v", chart[1])
	}
	// 40/(30+40+10) = 50%
	if chart[1].Hit < 49.9 || chart[1].Hit > 50.1 {
		t.Errorf("hit = %v, want 50", chart[1].Hit)
	}
	if got := chartRows(nil); len(got) != 0 {
		t.Errorf("empty rows should yield no chart points")
	}
}

func TestDashboardEstimatesUnpricedTurns(t *testing.T) {
	handler, db := newTestHandler(t)
	// A row stored before default pricing existed: no cost, cost_source=none.
	if _, err := db.InsertCall(ledger.Call{
		ID: "u1", Harness: "codex", SessionID: "s1", Model: "mystery-model",
		InputUncached: 50, CacheRead: 50, CacheWrite: 0, Output: 5,
		PromptTotal: 100, TotalTokens: 105, StartedAt: 1000,
		Source: "codex_rollout", CostSource: "none",
	}); err != nil {
		t.Fatal(err)
	}

	rec := doRequest(t, handler, http.MethodGet, "/", nil, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("dashboard = %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "≈") {
		t.Error("estimate marker missing from cost card")
	}
	if !strings.Contains(body, "turns unpriced") {
		t.Error("estimate breakdown missing")
	}

	rec = doRequest(t, handler, http.MethodGet, "/api/summary", nil, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("summary = %d", rec.Code)
	}
	var sum struct {
		CostEstimateUSD float64 `json:"cost_estimate_usd"`
		UnpricedCalls   int64   `json:"unpriced_calls"`
		UsedDefault     bool    `json:"unpriced_used_default"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &sum); err != nil {
		t.Fatal(err)
	}
	if sum.UnpricedCalls != 1 || !sum.UsedDefault || sum.CostEstimateUSD <= 0 {
		t.Fatalf("estimate = %+v", sum)
	}
	// (50*4 + 50*0.2 + 5*20)/1e6 = 0.00031 at the opus-5.5 default.
	if math.Abs(sum.CostEstimateUSD-0.00031) > 1e-12 {
		t.Errorf("estimate = %v, want 0.00031", sum.CostEstimateUSD)
	}
}

func TestRatesOverrideManualWins(t *testing.T) {
	rates := []ledger.ModelRate{
		{Model: "gpt-6", Input: 1, Source: "openrouter"},
		{Model: "gpt-6", Input: 9, Source: "manual"},
		{Model: "gpt-6-luna", Input: 2, Source: "openrouter"},
	}
	ov := RatesOverride(rates)
	if ov["gpt-6"].Input != 9 {
		t.Fatalf("manual row lost: %+v", ov)
	}
	if ov["gpt-6-luna"].Input != 2 {
		t.Fatalf("openrouter row lost: %+v", ov)
	}
}
