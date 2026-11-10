package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/BlasterSystems/token-literacy/internal/ledger"
)

func TestRootAndNotFound(t *testing.T) {
	handler, _ := newTestHandler(t)
	rec := doRequest(t, handler, http.MethodGet, "/", nil, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("root = %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "<!doctype html>") || !strings.Contains(rec.Body.String(), "token-literacy") {
		t.Fatalf("root body = %s", rec.Body)
	}
	rec = doRequest(t, handler, http.MethodGet, "/missing", nil, nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("missing = %d", rec.Code)
	}
}

func TestNilDBReturns503(t *testing.T) {
	handler := New(Info{Version: "t", Started: time.Now()}, nil, nil)
	for _, path := range []string{"/api/summary", "/api/rows", "/api/sessions"} {
		rec := doRequest(t, handler, http.MethodGet, path, nil, nil)
		if rec.Code != http.StatusServiceUnavailable {
			t.Errorf("%s with nil db = %d", path, rec.Code)
		}
	}
	rec := doRequest(t, handler, http.MethodPost, "/v1/traces", []byte("{}"), nil)
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("ingest with nil db = %d", rec.Code)
	}
}

func TestIngestLogsAcceptedAndDiscarded(t *testing.T) {
	handler, db := newTestHandler(t)
	body := []byte(`{"resourceLogs":[{"scopeLogs":[{"logRecords":[{"body":{"stringValue":"PROMPT CONTENT SHOULD NOT PERSIST"}}]}]}]}`)
	rec := doRequest(t, handler, http.MethodPost, "/v1/logs", body, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("logs = %d", rec.Code)
	}
	rows, _ := db.Rows(ledger.RowsFilter{})
	if len(rows) != 0 {
		t.Fatalf("logs must not create rows")
	}
}

func TestCorruptMetricStateStartsFresh(t *testing.T) {
	handler, db := newTestHandler(t)
	if err := db.SetMeta("claude_metric_series_state", "{corrupt"); err != nil {
		t.Fatal(err)
	}
	rec := doRequest(t, handler, http.MethodPost, "/v1/metrics", readFixture(t, "token_usage_metrics.json"), nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("metrics with corrupt state = %d", rec.Code)
	}
	rows, _ := db.Rows(ledger.RowsFilter{})
	if len(rows) != 1 {
		t.Fatalf("rows = %d", len(rows))
	}
}

func TestParseSince(t *testing.T) {
	cases := []struct {
		in   string
		want int64
	}{
		{"", 0},
		{"1700000000000", 1700000000000},
		{"7d", time.Now().Add(-7 * 24 * time.Hour).UnixMilli()},
		{"12h", time.Now().Add(-12 * time.Hour).UnixMilli()},
		{"2026-11-09T00:00:00Z", 1794182400000},
		{"garbage", 0},
	}
	for _, tc := range cases {
		got := parseSince(tc.in)
		switch tc.in {
		case "7d", "12h":
			if abs(got-tc.want) > 5_000 {
				t.Errorf("parseSince(%s) = %d, want ~%d", tc.in, got, tc.want)
			}
		default:
			if got != tc.want {
				t.Errorf("parseSince(%s) = %d, want %d", tc.in, got, tc.want)
			}
		}
	}
}

func TestQueryFilter(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/rows?since=7d&harness=codex&session=s1&limit=5&offset=2&order=asc", nil)
	f := queryFilter(req)
	if f.Harness != "codex" || f.Session != "s1" || f.Limit != 5 || f.Offset != 2 || !f.Asc || f.SinceMS == 0 {
		t.Fatalf("filter = %+v", f)
	}
}

func TestHTTPErrorJSON(t *testing.T) {
	rec := httptest.NewRecorder()
	httpError(rec, errTest{})
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("code = %d", rec.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || body["error"] != "boom" {
		t.Fatalf("body = %s", rec.Body)
	}
}

type errTest struct{}

func (errTest) Error() string { return "boom" }

func abs(n int64) int64 {
	if n < 0 {
		return -n
	}
	return n
}
