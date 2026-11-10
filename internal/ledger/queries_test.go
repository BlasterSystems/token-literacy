package ledger

import (
	"testing"
	"time"
)

func insertAggregate(t *testing.T, db *DB, id string) {
	t.Helper()
	sub := "aggregate"
	if _, err := db.InsertCall(Call{
		ID: id, Harness: "claude-code", SessionID: "aggregate", RequestID: id,
		InputUncached: 100, Output: 10, Source: "claude_otel", AgentType: &sub,
		StartedAt: time.Now().UnixMilli(),
	}); err != nil {
		t.Fatal(err)
	}
}

func TestSummaryAndFilters(t *testing.T) {
	db := openTest(t)
	now := time.Now().UnixMilli()
	rows := []Call{
		{ID: "a", Harness: "claude-code", SessionID: "s1", RequestID: "a", Model: "claude-sonnet-4-5", InputUncached: 100, CacheRead: 300, Output: 10, Source: "claude_otel", StartedAt: now - 5000},
		{ID: "b", Harness: "claude-code", SessionID: "s2", RequestID: "b", InputUncached: 50, Output: 5, Source: "claude_otel", StartedAt: now - 1000},
		{ID: "c", Harness: "codex", SessionID: "s1", RequestID: "c", InputUncached: 10, Output: 2, Source: "codex_rollout", StartedAt: now},
	}
	for _, c := range rows {
		if _, err := db.InsertCall(c); err != nil {
			t.Fatalf("%s: %v", c.ID, err)
		}
	}

	sum, err := db.Summary(RowsFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if sum.Calls != 3 || sum.TotalTokens != 477 {
		t.Fatalf("summary = %+v", sum)
	}
	if sum.CostPerOutputToken() != 0 {
		t.Errorf("unpriced rows must cost 0")
	}

	// Harness filter.
	codexSum, _ := db.Summary(RowsFilter{Harness: "codex"})
	if codexSum.Calls != 1 || codexSum.Output != 2 {
		t.Fatalf("codex summary = %+v", codexSum)
	}
	// Session filter.
	s1, _ := db.Summary(RowsFilter{Session: "s1", Harness: "claude-code"})
	if s1.Calls != 1 {
		t.Fatalf("s1 summary = %+v", s1)
	}
	// Since filter.
	recent, _ := db.Summary(RowsFilter{SinceMS: now - 2000})
	if recent.Calls != 2 {
		t.Fatalf("recent summary = %+v", recent)
	}

	// Rows: limit/offset/order.
	first, _ := db.Rows(RowsFilter{Limit: 1, Asc: true})
	if first[0].ID != "a" {
		t.Errorf("asc first = %s", first[0].ID)
	}
	second, _ := db.Rows(RowsFilter{Limit: 1, Offset: 1, Asc: true})
	if second[0].ID != "b" {
		t.Errorf("offset = %s", second[0].ID)
	}
	desc, _ := db.Rows(RowsFilter{Limit: 1})
	if desc[0].ID != "c" {
		t.Errorf("desc first = %s", desc[0].ID)
	}
}

func TestAggregateCount(t *testing.T) {
	db := openTest(t)
	insertAggregate(t, db, "agg:1")
	n, err := db.AggregateCount("claude-code")
	if err != nil || n != 1 {
		t.Fatalf("aggregate count = %d, %v", n, err)
	}
	if n, _ := db.AggregateCount("codex"); n != 0 {
		t.Fatalf("codex aggregate count = %d", n)
	}
}

func TestDBSizeAndFormatMs(t *testing.T) {
	db := openTest(t)
	size, err := db.DBSize()
	if err != nil || size == 0 {
		t.Fatalf("size = %d, %v", size, err)
	}
	if got := FormatMs(0); got != "never" {
		t.Errorf("FormatMs(0) = %s", got)
	}
	if got := FormatMs(123); got != "123" {
		t.Errorf("FormatMs = %s", got)
	}
}

func TestInsertEmptySourceRejected(t *testing.T) {
	db := openTest(t)
	c := validCall()
	c.Source = ""
	if _, err := db.InsertCall(c); err == nil {
		t.Fatal("empty source must be rejected")
	}
}

func TestCursorOverwrite(t *testing.T) {
	db := openTest(t)
	if err := db.SetCursor("f", 1, 10, ""); err != nil {
		t.Fatal(err)
	}
	if err := db.SetCursor("f", 1, 20, "{\"session_id\":\"s\"}"); err != nil {
		t.Fatal(err)
	}
	_, offset, state, ok, _ := db.GetCursor("f")
	if !ok || offset != 20 || state != "{\"session_id\":\"s\"}" {
		t.Fatalf("cursor = %d %q %v", offset, state, ok)
	}
}
