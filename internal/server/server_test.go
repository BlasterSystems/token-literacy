package server

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/BlasterSystems/token-literacy/internal/ledger"
)

func newTestHandler(t *testing.T) (http.Handler, *ledger.DB) {
	t.Helper()
	db, err := ledger.Open(filepath.Join(t.TempDir(), "ledger.db"))
	if err != nil {
		t.Fatalf("open ledger: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	handler := New(Info{Version: "test", Started: time.Now(), DBPath: "test.db"}, db, nil)
	return handler, db
}

func doRequest(t *testing.T, handler http.Handler, method, target string, body []byte, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, target, bytes.NewReader(body))
	req.Host = "127.0.0.1:4318"
	req.RemoteAddr = "127.0.0.1:54321"
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "otlp", name))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	return data
}

func TestHealth(t *testing.T) {
	handler, _ := newTestHandler(t)
	rec := doRequest(t, handler, http.MethodGet, "/health", nil, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["status"] != "ok" {
		t.Errorf("status = %v", body["status"])
	}
	if body["calls_total"] != float64(0) {
		t.Errorf("calls_total = %v", body["calls_total"])
	}
}

func TestIngestTraces(t *testing.T) {
	handler, db := newTestHandler(t)
	rec := doRequest(t, handler, http.MethodPost, "/v1/traces", readFixture(t, "llm_request_span.json"), map[string]string{"Content-Type": "application/json"})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body)
	}
	rows, err := db.Rows(ledger.RowsFilter{Asc: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(rows))
	}
	c := rows[0]
	// Row must equal the span: U + R + W == prompt_total, total == P + O.
	if c.InputUncached != 707 || c.CacheRead != 28032 || c.CacheWrite != 0 || c.Output != 11 {
		t.Errorf("token split = %d/%d/%d/%d", c.InputUncached, c.CacheRead, c.CacheWrite, c.Output)
	}
	if c.PromptTotal != 707+28032+0 || c.TotalTokens != c.PromptTotal+11 {
		t.Errorf("totals: P=%d total=%d", c.PromptTotal, c.TotalTokens)
	}
	if c.ID != "claude-code:req_01HTZ0TEST" || c.SessionID != "sess-abc123" {
		t.Errorf("ids: %s session=%s", c.ID, c.SessionID)
	}
	if c.HarnessVersion == nil || *c.HarnessVersion != "2.1.268" {
		t.Errorf("harness_version = %v", c.HarnessVersion)
	}
	if c.CostSource != "list_price" || c.CostUSD == nil {
		t.Errorf("cost = %v source=%s", c.CostUSD, c.CostSource)
	}
	if c.Raw == nil || strings.Contains(*c.Raw, "content") {
		t.Errorf("raw must be trimmed metadata: %v", c.Raw)
	}
}

func TestIngestTracesIdempotent(t *testing.T) {
	handler, db := newTestHandler(t)
	fixture := readFixture(t, "llm_request_span.json")
	for range 2 {
		rec := doRequest(t, handler, http.MethodPost, "/v1/traces", fixture, nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("post: status = %d", rec.Code)
		}
	}
	rows, _ := db.Rows(ledger.RowsFilter{})
	if len(rows) != 1 {
		t.Fatalf("rows after replay = %d, want 1 (restart idempotency)", len(rows))
	}
}

func TestIngestTracesGzip(t *testing.T) {
	handler, db := newTestHandler(t)
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	zw.Write(readFixture(t, "llm_request_span.json"))
	zw.Close()
	rec := doRequest(t, handler, http.MethodPost, "/v1/traces", buf.Bytes(), map[string]string{"Content-Encoding": "gzip"})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	rows, _ := db.Rows(ledger.RowsFilter{})
	if len(rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(rows))
	}
}

func TestIngestMalformedDoesNotKillServer(t *testing.T) {
	handler, db := newTestHandler(t)
	rec := doRequest(t, handler, http.MethodPost, "/v1/traces", []byte("this is not otel"), nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("malformed status = %d, want 400", rec.Code)
	}
	// Zero-total span payload: decodes but violates the invariant.
	rec = doRequest(t, handler, http.MethodPost, "/v1/traces", []byte(`{"resourceSpans":[{"scopeSpans":[{"spans":[{"name":"claude_code.llm_request","startTimeUnixNano":"1","attributes":[{"key":"input_tokens","value":{"intValue":"0"}},{"key":"request_id","value":{"stringValue":"z"}},{"key":"session.id","value":{"stringValue":"s"}}]}]}]}]}`), nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("zero-total should be skipped, not fail: %d", rec.Code)
	}
	if rows, _ := db.Rows(ledger.RowsFilter{}); len(rows) != 0 {
		t.Fatalf("zero-total row was inserted")
	}
	// Server is still healthy afterwards.
	rec = doRequest(t, handler, http.MethodGet, "/health", nil, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("health after malformed = %d", rec.Code)
	}
}

func TestIngestMetricsFallback(t *testing.T) {
	handler, db := newTestHandler(t)
	rec := doRequest(t, handler, http.MethodPost, "/v1/metrics", readFixture(t, "token_usage_metrics.json"), nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	rows, _ := db.Rows(ledger.RowsFilter{Asc: true})
	if len(rows) != 1 {
		t.Fatalf("rows = %d, want 1 aggregate row", len(rows))
	}
	c := rows[0]
	if c.AgentType == nil || *c.AgentType != "aggregate" {
		t.Errorf("agent_type = %v, want aggregate", c.AgentType)
	}
	if c.InputUncached != 707 || c.CacheRead != 28032 || c.Output != 11 {
		t.Errorf("aggregate split = %d/%d/%d", c.InputUncached, c.CacheRead, c.Output)
	}
	// Replay of the same window must not duplicate.
	rec = doRequest(t, handler, http.MethodPost, "/v1/metrics", readFixture(t, "token_usage_metrics.json"), nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("replay status = %d", rec.Code)
	}
	rows, _ = db.Rows(ledger.RowsFilter{})
	if len(rows) != 1 {
		t.Fatalf("rows after replay = %d, want 1", len(rows))
	}
}

func TestLoopbackGuard(t *testing.T) {
	handler, _ := newTestHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req.Host = "evil.example.com:80"
	req.RemoteAddr = "127.0.0.1:1234"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("evil Host status = %d, want 403", rec.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/health", nil)
	req.Host = "127.0.0.1:4318"
	req.RemoteAddr = "10.1.2.3:9999"
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("non-loopback RemoteAddr status = %d, want 403", rec.Code)
	}
}

func TestRowsAndSummary(t *testing.T) {
	handler, _ := newTestHandler(t)
	doRequest(t, handler, http.MethodPost, "/v1/traces", readFixture(t, "llm_request_span.json"), nil)

	rec := doRequest(t, handler, http.MethodGet, "/api/rows", nil, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("rows status = %d", rec.Code)
	}
	var rows []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &rows); err != nil {
		t.Fatalf("decode rows: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("rows = %d", len(rows))
	}

	rec = doRequest(t, handler, http.MethodGet, "/api/summary", nil, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("summary status = %d", rec.Code)
	}
	var sum map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &sum); err != nil {
		t.Fatalf("decode summary: %v", err)
	}
	if sum["total_tokens"] != float64(707+28032+0+11) {
		t.Errorf("total_tokens = %v", sum["total_tokens"])
	}
	// Cache hit rate must derive from the additive row: R/(U+R).
	want := float64(28032) / float64(28032+707)
	if got := sum["cache_hit_rate"].(float64); got*want == 0 || got <= 0.9 || got >= 1.0 {
		t.Errorf("cache_hit_rate = %v, want ~%v", got, want)
	}
	rec = doRequest(t, handler, http.MethodGet, "/api/sessions", nil, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("sessions status = %d", rec.Code)
	}
	var sessions []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &sessions); err != nil {
		t.Fatalf("decode sessions: %v", err)
	}
	if len(sessions) != 1 || sessions[0]["session_id"] != "sess-abc123" {
		t.Errorf("sessions = %v", sessions)
	}
}
