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
