package server

import (
	"encoding/csv"
	"encoding/json"
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
		"Total tokens", "Cache hit rate", "Fresh share", "Context tax",
		"API-equiv. cost", "Cost / output token",
		"<svg viewBox=", "<rect", "<polyline", "s-twenty",
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

func TestStackedBarsSVGShape(t *testing.T) {
	rows := []ledger.Call{
		{ID: "a", InputUncached: 10, CacheRead: 20, CacheWrite: 0, Output: 5, StartedAt: 100},
		{ID: "b", InputUncached: 30, CacheRead: 40, CacheWrite: 10, Output: 15, StartedAt: 200},
	}
	svg := stackedBarsSVG(rows)
	if !strings.HasPrefix(svg, `<svg viewBox="0 0 `) {
		t.Fatalf("svg = %s", svg[:60])
	}
	if strings.Count(svg, "<rect") != 7 { // 4+3 nonzero segments
		t.Errorf("rects = %d, want 7", strings.Count(svg, "<rect"))
	}
	if !strings.Contains(svg, "var(--c2)") {
		t.Error("cache-read color missing")
	}
	empty := stackedBarsSVG(nil)
	if !strings.Contains(empty, "no turns in range") {
		t.Error("empty svg missing message")
	}
}

func TestRateLineSVGShape(t *testing.T) {
	rows := []ledger.Call{
		{ID: "a", InputUncached: 10, CacheRead: 30, Output: 5, StartedAt: 100},
		{ID: "b", InputUncached: 30, CacheRead: 10, Output: 15, StartedAt: 200},
	}
	svg := rateLineSVG(rows)
	if !strings.Contains(svg, "<polyline") {
		t.Fatal("polyline missing")
	}
	if !strings.Contains(svg, "25%") || !strings.Contains(svg, "100%") {
		t.Error("gridline labels missing")
	}
}
