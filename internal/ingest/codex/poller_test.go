package codex

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/BlasterSystems/token-literacy/internal/ledger"
)

func openDB(t *testing.T) *ledger.DB {
	t.Helper()
	db, err := ledger.Open(filepath.Join(t.TempDir(), "ledger.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func newPoller(db *ledger.DB, dir string) *Poller {
	return &Poller{DB: db, Dir: dir, Logf: func(string, ...any) {}, Warnf: func(string, ...any) {}}
}

func TestScanOnceBackfillsAllFiles(t *testing.T) {
	db := openDB(t)
	dir := filepath.Join("..", "..", "..", "testdata", "codex")
	p := newPoller(db, dir)
	inserted, warnings, err := p.ScanOnce(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) != 0 {
		t.Fatalf("warnings = %v", warnings)
	}
	// record(1) + record_and_event(1) + event_only(1) + subagent(1) + resumed(2)
	if inserted != 6 {
		t.Fatalf("inserted = %d, want 6", inserted)
	}
	rows, _ := db.Rows(ledger.RowsFilter{Asc: true})
	if len(rows) != 6 {
		t.Fatalf("rows = %d", len(rows))
	}
	// Second scan is a no-op (cursors).
	inserted, _, err = p.ScanOnce(context.Background())
	if err != nil || inserted != 0 {
		t.Fatalf("second scan: inserted=%d err=%v (cursor idempotency)", inserted, err)
	}
}

func TestMissingDirectoryIsNotAnError(t *testing.T) {
	db := openDB(t)
	p := newPoller(db, filepath.Join(t.TempDir(), "does-not-exist"))
	inserted, _, err := p.ScanOnce(context.Background())
	if err != nil || inserted != 0 {
		t.Fatalf("missing dir: inserted=%d err=%v", inserted, err)
	}
}

func TestIncrementalAppendAndPartialLine(t *testing.T) {
	db := openDB(t)
	dir := t.TempDir()
	file := filepath.Join(dir, "rollout-live.jsonl")
	write(t, file, fixture(t, "record.jsonl"))

	p := newPoller(db, dir)
	inserted, _, err := p.ScanOnce(context.Background())
	if err != nil || inserted != 1 {
		t.Fatalf("first scan: inserted=%d err=%v", inserted, err)
	}

	// Append a partial line: must not be consumed yet.
	f, _ := os.OpenFile(file, os.O_APPEND|os.O_WRONLY, 0o600)
	f.WriteString(`{"timestamp":"2026-11-07T18:00:00.000Z","ordinal":3,"type":"token_usage_`)
	f.Close()
	inserted, _, err = p.ScanOnce(context.Background())
	if err != nil || inserted != 0 {
		t.Fatalf("partial line: inserted=%d err=%v", inserted, err)
	}

	// Complete the line: now consumed exactly once.
	f, _ = os.OpenFile(file, os.O_APPEND|os.O_WRONLY, 0o600)
	f.WriteString(`record","payload":{"thread_id":"01a1b71c-6c14-7161-9c84-6d8b9baa9701","turn_id":"t1","session_id":"01a1b71c-6c14-7161-9c84-6d8b9baa9701","response_id":"resp_fixture0004","usage":{"input_tokens":3000,"cached_input_tokens":1000,"cache_write_input_tokens":0,"output_tokens":20,"reasoning_output_tokens":5,"total_tokens":3020}}}` + "\n")
	f.Close()
	inserted, _, err = p.ScanOnce(context.Background())
	if err != nil || inserted != 1 {
		t.Fatalf("completed line: inserted=%d err=%v", inserted, err)
	}
	rows, _ := db.Rows(ledger.RowsFilter{})
	if len(rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(rows))
	}
}

func TestResumedReplayDedupesGlobally(t *testing.T) {
	db := openDB(t)
	dir := filepath.Join("..", "..", "..", "testdata", "codex", "resumed")
	p := newPoller(db, dir)
	if _, _, err := p.ScanOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	rows, _ := db.Rows(ledger.RowsFilter{})
	ids := map[string]bool{}
	for _, r := range rows {
		ids[r.ID] = true
	}
	if len(rows) != 2 || len(ids) != 2 {
		t.Fatalf("rows = %v, want exactly resp_fixture0009 + resp_fixture0010", ids)
	}
}

// Review finding (blocker): resumed polls must restore parser state, or
// every post-first-poll row loses its session/model/cost.
func TestResumedPollRestoresState(t *testing.T) {
	db := openDB(t)
	dir := t.TempDir()
	file := filepath.Join(dir, "rollout-live.jsonl")
	write(t, file, fixture(t, "record.jsonl")) // session_meta + turn_context + record

	p := newPoller(db, dir)
	if _, _, err := p.ScanOnce(context.Background()); err != nil {
		t.Fatal(err)
	}

	// Append a second record in a new "poll window" — no session_meta again.
	f, _ := os.OpenFile(file, os.O_APPEND|os.O_WRONLY, 0o600)
	f.WriteString(`{"timestamp":"2026-11-07T17:45:39.275Z","ordinal":3,"type":"token_usage_record","payload":{"thread_id":"01a1b71c-6c14-7161-9c84-6d8b9baa9701","turn_id":"01a1b71c-847","session_id":"01a1b71c-6c14-7161-9c84-6d8b9baa9701","response_id":"resp_fixture0005","usage":{"input_tokens":3000,"cached_input_tokens":1000,"cache_write_input_tokens":0,"output_tokens":20,"reasoning_output_tokens":5,"total_tokens":3020}}}` + "\n")
	f.Close()
	if _, _, err := p.ScanOnce(context.Background()); err != nil {
		t.Fatal(err)
	}

	rows, _ := db.Rows(ledger.RowsFilter{Asc: true})
	if len(rows) != 2 {
		t.Fatalf("rows = %d", len(rows))
	}
	second := rows[1]
	if second.ID != "codex:resp_fixture0005" {
		t.Fatalf("second id = %s", second.ID)
	}
	if second.SessionID != "01a1b71c-6c14-7161-9c84-6d8b9baa9701" {
		t.Errorf("resumed row session = %q (state not restored?)", second.SessionID)
	}
	if second.Model != "gpt-5.2" {
		t.Errorf("resumed row model = %q", second.Model)
	}
	if second.CostSource != "list_price" || second.CostUSD == nil {
		t.Errorf("resumed row cost = %v / %s (empty model breaks pricing)", second.CostUSD, second.CostSource)
	}
}

// Review finding (major): the records-win rule is per FILE, not per poll
// window — an event written right after a record must not double-count.
func TestRecordThenEventAcrossPolls(t *testing.T) {
	db := openDB(t)
	dir := t.TempDir()
	file := filepath.Join(dir, "rollout-live.jsonl")

	record := `{"timestamp":"2026-11-07T17:44:39.275Z","ordinal":2,"type":"token_usage_record","payload":{"session_id":"s-cross","response_id":"resp_cross_1","usage":{"input_tokens":100,"cached_input_tokens":0,"cache_write_input_tokens":0,"output_tokens":5,"reasoning_output_tokens":0,"total_tokens":105}}}` + "\n"
	event := `{"timestamp":"2026-11-07T17:44:39.277Z","ordinal":3,"type":"event_msg","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":100,"cached_input_tokens":0,"cache_write_input_tokens":0,"output_tokens":5,"reasoning_output_tokens":0,"total_tokens":105}}}}` + "\n"

	p := newPoller(db, dir)
	write(t, file, []byte(record))
	if _, _, err := p.ScanOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	f, _ := os.OpenFile(file, os.O_APPEND|os.O_WRONLY, 0o600)
	f.WriteString(event)
	f.Close()
	if _, _, err := p.ScanOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	rows, _ := db.Rows(ledger.RowsFilter{})
	if len(rows) != 1 || rows[0].ID != "codex:resp_cross_1" {
		t.Fatalf("rows = %+v, want exactly the record row", rows)
	}
}

// Review finding (major): event fallback IDs must not collide across files
// of one resumed session.
func TestEventIDsSaltedPerFile(t *testing.T) {
	db := openDB(t)
	dir := t.TempDir()
	event := `{"ordinal":2,"type":"event_msg","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":10,"cached_input_tokens":0,"cache_write_input_tokens":0,"output_tokens":1,"reasoning_output_tokens":0,"total_tokens":11}}}}` + "\n"
	for _, name := range []string{"rollout-a.jsonl", "rollout-b.jsonl"} {
		write(t, filepath.Join(dir, name), []byte(event))
	}
	p := newPoller(db, dir)
	if _, _, err := p.ScanOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	rows, _ := db.Rows(ledger.RowsFilter{})
	if len(rows) != 2 || rows[0].ID == rows[1].ID {
		t.Fatalf("rows = %+v, want 2 distinct salted IDs", rows)
	}
}

// Review finding (major): invariants the ledger rejects must be pre-filtered
// so a bad record cannot stall the file cursor forever.
func TestInvariantPrefilterPreventsStall(t *testing.T) {
	db := openDB(t)
	dir := t.TempDir()
	file := filepath.Join(dir, "rollout-stall.jsonl")
	// reasoning_output_tokens (9) > output_tokens (3): ledger would reject.
	write(t, file, []byte(`{"ordinal":0,"type":"session_meta","payload":{"session_id":"s1","cli_version":"0.166.0"}}
{"ordinal":1,"type":"token_usage_record","payload":{"session_id":"s1","response_id":"r_bad","usage":{"input_tokens":100,"cached_input_tokens":0,"cache_write_input_tokens":0,"output_tokens":3,"reasoning_output_tokens":9,"total_tokens":103}}}` + "\n"))
	p := newPoller(db, dir)
	if _, _, err := p.ScanOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	// The cursor advanced past the bad record.
	_, offset, _, ok, _ := db.GetCursor("codex:" + file)
	if !ok || offset == 0 {
		t.Fatalf("cursor not advanced past rejected record: offset=%d ok=%v", offset, ok)
	}
	// A good record after the bad one is still ingested.
	f, _ := os.OpenFile(file, os.O_APPEND|os.O_WRONLY, 0o600)
	f.WriteString(`{"ordinal":2,"type":"token_usage_record","payload":{"session_id":"s1","response_id":"r_good","usage":{"input_tokens":100,"cached_input_tokens":0,"cache_write_input_tokens":0,"output_tokens":3,"reasoning_output_tokens":2,"total_tokens":103}}}` + "\n")
	f.Close()
	inserted, _, err := p.ScanOnce(context.Background())
	if err != nil || inserted != 1 {
		t.Fatalf("post-stall insert = %d, %v", inserted, err)
	}
}

func TestRunStopsOnContextCancel(t *testing.T) {
	db := openDB(t)
	p := newPoller(db, t.TempDir())
	p.Interval = 10 * time.Millisecond
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		p.Run(ctx)
		close(done)
	}()
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not stop on cancel")
	}
}

func write(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}
