package simulate

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/BlasterSystems/token-literacy/internal/ledger"
)

func copyFixtures(t *testing.T, dest string, files map[string]string) {
	t.Helper()
	for name, src := range files {
		data, err := os.ReadFile(src)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dest, name), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func TestRunLoadsProductionFixtures(t *testing.T) {
	db, err := ledger.Open(filepath.Join(t.TempDir(), "ledger.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	dir := t.TempDir()
	copyFixtures(t, dir, map[string]string{
		"span.json":          filepath.Join("..", "..", "testdata", "otlp", "llm_request_span.json"),
		"metrics.json":       filepath.Join("..", "..", "testdata", "otlp", "token_usage_metrics.json"),
		"codex_record.jsonl": filepath.Join("..", "..", "testdata", "codex", "record.jsonl"),
	})

	inserted, err := Run(db, dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	// 1 span + 1 aggregate metric window + 1 codex record.
	if inserted != 3 {
		t.Fatalf("inserted = %d, want 3", inserted)
	}
	rows, _ := db.Rows(ledger.RowsFilter{Asc: true})
	if len(rows) != 3 {
		t.Fatalf("rows = %d", len(rows))
	}
	if rows[0].Harness != "claude-code" || rows[2].Harness != "codex" {
		t.Fatalf("harnesses = %s/%s/%s", rows[0].Harness, rows[1].Harness, rows[2].Harness)
	}
}

func TestRunSkipsNonOTLPJSON(t *testing.T) {
	db, err := ledger.Open(filepath.Join(t.TempDir(), "ledger.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "other.json"), []byte(`{"unrelated":"payload"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	inserted, err := Run(db, dir, nil)
	if err != nil || inserted != 0 {
		t.Fatalf("inserted = %d, err = %v", inserted, err)
	}
}

func TestRunEmptyDir(t *testing.T) {
	db, err := ledger.Open(filepath.Join(t.TempDir(), "ledger.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	inserted, err := Run(db, t.TempDir(), nil)
	if err != nil || inserted != 0 {
		t.Fatalf("inserted = %d, err = %v", inserted, err)
	}
}
