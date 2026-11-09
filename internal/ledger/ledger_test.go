package ledger

import (
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func openTest(t *testing.T) *DB {
	t.Helper()
	db, err := Open(filepath.Join(t.TempDir(), "ledger.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func validCall() Call {
	return Call{
		ID: "codex:resp_1", Harness: "codex", SessionID: "s1", RequestID: "resp_1",
		Model: "gpt-5.2", InputUncached: 707, CacheRead: 28032, Output: 11,
		Source: "codex_rollout", StartedAt: time.Now().UnixMilli(),
	}
}

func TestInsertIdempotent(t *testing.T) {
	db := openTest(t)
	inserted, err := db.InsertCall(validCall())
	if err != nil || !inserted {
		t.Fatalf("first insert: inserted=%v err=%v", inserted, err)
	}
	inserted, err = db.InsertCall(validCall())
	if err != nil || inserted {
		t.Fatalf("replay: inserted=%v err=%v (want false/nil)", inserted, err)
	}
	rows, _ := db.Rows(RowsFilter{})
	if len(rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(rows))
	}
}

func TestInsertDerivedTotals(t *testing.T) {
	db := openTest(t)
	c := validCall()
	c.PromptTotal = 999 // caller error: ledger derives from additive fields
	c.TotalTokens = 999
	if _, err := db.InsertCall(c); err != nil {
		t.Fatalf("insert: %v", err)
	}
	rows, _ := db.Rows(RowsFilter{})
	if rows[0].PromptTotal != 707+28032 || rows[0].TotalTokens != 707+28032+11 {
		t.Fatalf("derived totals wrong: P=%d T=%d", rows[0].PromptTotal, rows[0].TotalTokens)
	}
}

func TestInvariantRejections(t *testing.T) {
	db := openTest(t)
	cases := []struct {
		name string
		mut  func(*Call)
		want string // substring of the error
	}{
		{"negative input", func(c *Call) { c.InputUncached = -1 }, "negative"},
		{"negative output", func(c *Call) { c.Output = -5 }, "negative"},
		{"zero total", func(c *Call) { c.Output = 0; c.CacheRead = 0; c.InputUncached = 0 }, "zero total"},
		{"reasoning exceeds output", func(c *Call) { c.Reasoning = 100 }, "reasoning"},
		{"empty id", func(c *Call) { c.ID = "" }, "empty id"},
		{"empty harness", func(c *Call) { c.Harness = "" }, "empty harness"},
	}
	for _, tc := range cases {
		c := validCall()
		c.ID = "x:" + tc.name
		tc.mut(&c)
		_, err := db.InsertCall(c)
		if err == nil {
			t.Errorf("%s: expected rejection", tc.name)
			continue
		}
		if !errors.Is(err, ErrInvalidCall) {
			t.Errorf("%s: err = %v, want ErrInvalidCall", tc.name, err)
		}
	}
}

func TestTurnIndexPerSession(t *testing.T) {
	db := openTest(t)
	for i := range 3 {
		c := validCall()
		c.ID = "codex:resp_" + string(rune('a'+i))
		c.RequestID = c.ID
		c.StartedAt = 1000 + int64(i)*1000
		if _, err := db.InsertCall(c); err != nil {
			t.Fatal(err)
		}
	}
	// Different session starts at 1.
	c := validCall()
	c.ID = "codex:other"
	c.SessionID = "s2"
	c.StartedAt = 9000
	if _, err := db.InsertCall(c); err != nil {
		t.Fatal(err)
	}
	rows, _ := db.Rows(RowsFilter{Asc: true})
	if rows[0].TurnIndex != 1 || rows[1].TurnIndex != 2 || rows[2].TurnIndex != 3 {
		t.Errorf("turn indexes = %d,%d,%d", rows[0].TurnIndex, rows[1].TurnIndex, rows[2].TurnIndex)
	}
	if rows[3].TurnIndex != 1 {
		t.Errorf("new session turn index = %d, want 1", rows[3].TurnIndex)
	}
}

func TestSummaryMetrics(t *testing.T) {
	// Formulas from README (M0): hit = R/(U+R+W), fresh = U/(U+R+W),
	// context tax = (U+R+W)/O.
	s := Summary{InputFresh: 707, CacheRead: 28032, CacheWrite: 0, Output: 11}
	if got := s.HitRate(); got != 28032.0/28739.0 {
		t.Errorf("hit rate = %v, want %v", got, 28032.0/28739.0)
	}
	if got := s.FreshShare(); got != 707.0/28739.0 {
		t.Errorf("fresh share = %v, want %v", got, 707.0/28739.0)
	}
	if got := s.ContextTax(); got != 28739.0/11.0 {
		t.Errorf("context tax = %v, want %v", got, 28739.0/11.0)
	}
	empty := Summary{}
	if empty.HitRate() != 0 || empty.FreshShare() != 0 || empty.ContextTax() != 0 {
		t.Error("empty summary must not divide by zero")
	}
}

func TestSessionRollup(t *testing.T) {
	db := openTest(t)
	if _, err := db.InsertCall(validCall()); err != nil {
		t.Fatal(err)
	}
	c := validCall()
	c.ID = "codex:resp_2"
	c.SessionID = "s1"
	if _, err := db.InsertCall(c); err != nil {
		t.Fatal(err)
	}
	sessions, err := db.Sessions(0, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 1 || sessions[0].Turns != 2 {
		t.Fatalf("sessions = %+v", sessions)
	}
}

func TestCursorsAndMeta(t *testing.T) {
	db := openTest(t)
	if _, _, ok, _ := db.GetCursor("x"); ok {
		t.Fatal("cursor should be absent")
	}
	if err := db.SetCursor("x", 42, 1024); err != nil {
		t.Fatal(err)
	}
	inode, offset, ok, err := db.GetCursor("x")
	if err != nil || !ok || inode != 42 || offset != 1024 {
		t.Fatalf("cursor = %d %d %v %v", inode, offset, ok, err)
	}
	if err := db.SetMeta("k", "v"); err != nil {
		t.Fatal(err)
	}
	if v, _ := db.GetMeta("k"); v != "v" {
		t.Fatalf("meta = %q", v)
	}
	if v, _ := db.GetMeta("missing"); v != "" {
		t.Fatalf("missing meta = %q", v)
	}
}

func TestSchemaVersionGuard(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ledger.db")
	db, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	db.Close()

	// Simulate a future schema.
	raw, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := raw.Exec("PRAGMA user_version=99"); err != nil {
		t.Fatal(err)
	}
	raw.Close()

	if _, err := Open(path); err == nil {
		t.Fatal("opening a newer schema must fail")
	}
}

func TestIntegrityAndCounts(t *testing.T) {
	db := openTest(t)
	if _, err := db.InsertCall(validCall()); err != nil {
		t.Fatal(err)
	}
	if result, err := db.IntegrityCheck(); err != nil || result != "ok" {
		t.Fatalf("integrity = %s, %v", result, err)
	}
	total, byHarness, err := db.Counts()
	if err != nil || total != 1 || byHarness["codex"] != 1 {
		t.Fatalf("counts = %d %v %v", total, byHarness, err)
	}
	if last, _ := db.LastSourceTime("codex"); last == 0 {
		t.Fatal("last source time should be set")
	}
}
