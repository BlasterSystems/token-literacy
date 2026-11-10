package ledger

import (
	"encoding/csv"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestWriteCSVAndJSONRoundtrip(t *testing.T) {
	db := openTest(t)
	ended := int64(1700000002000)
	dur := int64(2000)
	sub := "subagent"
	rows := []Call{
		{ID: "a", Harness: "codex", SessionID: "s1", RequestID: "a", Model: "gpt-5.2",
			InputUncached: 10, CacheRead: 5, Output: 2, Reasoning: 1,
			CostUSD: floatPtr(0.001), CostSource: "list_price",
			StartedAt: 1700000000000, EndedAt: &ended, DurationMs: &dur, TTFTMs: &dur,
			StopReason: sptr("stop"), Attempt: &dur, ToolName: nil,
			Source: "codex_rollout", HarnessVersion: sptr("0.166.0"), AgentType: &sub},
	}
	// Also through the real insert path for field coverage.
	if _, err := db.InsertCall(Call{ID: "b", Harness: "codex", SessionID: "s1", RequestID: "b",
		InputUncached: 1, Output: 1, Source: "codex_rollout", StartedAt: 1}); err != nil {
		t.Fatal(err)
	}
	rows2, _ := db.Rows(RowsFilter{Asc: true})
	rows = append(rows, rows2...)

	var csvBuf, jsonBuf strings.Builder
	if err := WriteCSV(&csvBuf, rows); err != nil {
		t.Fatal(err)
	}
	if err := WriteJSON(&jsonBuf, rows); err != nil {
		t.Fatal(err)
	}

	records, err := csv.NewReader(strings.NewReader(csvBuf.String())).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 3 { // header + 2 rows
		t.Fatalf("csv rows = %d", len(records))
	}
	header := records[0]
	if header[0] != "id" || header[15] != "cost_usd" {
		t.Fatalf("header = %v", header)
	}

	var parsed []Call
	if err := json.Unmarshal([]byte(jsonBuf.String()), &parsed); err != nil {
		t.Fatal(err)
	}
	if len(parsed) != 2 || parsed[0].ID != "a" {
		t.Fatalf("json = %+v", parsed)
	}
}

func TestExportFieldAllColumns(t *testing.T) {
	ended := int64(2)
	c := Call{
		ID: "id", Harness: "h", SessionID: "sess", RequestID: "req", Model: "m",
		BillingMode: "subscription", InputUncached: 1, CacheRead: 2, CacheWrite: 3,
		Output: 4, Reasoning: 1, PromptTotal: 6, TotalTokens: 10,
		CostUSD:  floatPtr(0.5), CostSource: "list_price",
		StartedAt: 1000, EndedAt: &ended, DurationMs: &ended, TTFTMs: &ended,
		StopReason: sptr("stop"), Attempt: &ended, ToolName: sptr("t"),
		Source: "src", HarnessVersion: sptr("v"), AgentType: sptr("subagent"),
	}
	for _, col := range ExportColumns {
		v := exportField(col, c)
		if v == "" && col != "tool_name" {
			t.Errorf("column %s empty", col)
		}
	}
	// Nil-optionals render empty.
	empty := Call{ID: "x", Harness: "h", Source: "s"}
	if v := exportField("ended_at", empty); v != "" {
		t.Errorf("nil ended_at = %q", v)
	}
	if v := exportField("agent_type", empty); v != "" {
		t.Errorf("nil agent_type = %q", v)
	}
}

func TestParseSinceBranches(t *testing.T) {
	if ParseSince("") != 0 {
		t.Error("empty must be 0")
	}
	if ParseSince("1700000000000") != 1700000000000 {
		t.Error("epoch-ms")
	}
	if ParseSince("2026-11-09T00:00:00Z") != 1794182400000 {
		t.Error("rfc3339")
	}
	if ParseSince("garbage") != 0 {
		t.Error("garbage must be 0")
	}
	if ParseSince("0d") != 0 {
		t.Error("0d must be 0")
	}
	now := time.Now().UnixMilli()
	if d := ParseSince("1d"); d < now-25*3600_000 || d > now-23*3600_000 {
		t.Errorf("1d = %d", d)
	}
	if d := ParseSince("90m"); d > now-89*60_000 || d < now-91*60_000 {
		t.Errorf("90m = %d", d)
	}
}

func TestCursorLag(t *testing.T) {
	db := openTest(t)
	if lag, _ := db.CursorLag(); lag != 0 {
		t.Fatalf("lag = %d", lag)
	}
	if err := db.SetCursor("f", 1, 1, ""); err != nil {
		t.Fatal(err)
	}
	if lag, _ := db.CursorLag(); lag == 0 {
		t.Fatal("lag should be set")
	}
}

func floatPtr(f float64) *float64 { return &f }

func sptr(s string) *string { return &s }
