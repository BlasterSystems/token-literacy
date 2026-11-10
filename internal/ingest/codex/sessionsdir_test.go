package codex

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BlasterSystems/token-literacy/internal/ledger"
)

func TestSessionsDirFromEnv(t *testing.T) {
	t.Setenv("CODEX_HOME", "/custom/codex")
	dir, ok := SessionsDir()
	if !ok || !strings.HasSuffix(dir, filepath.Join("custom", "codex", "sessions")) {
		t.Fatalf("dir = %s", dir)
	}
}

func TestNilLoggersDoNotPanic(t *testing.T) {
	db, err := ledger.Open(filepath.Join(t.TempDir(), "ledger.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	// Unparsable file triggers warnf with nil Warnf; must not panic.
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "rollout-bad.jsonl"), []byte("not json\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	p := &Poller{DB: db, Dir: dir} // no Logf/Warnf
	if _, _, err := p.ScanOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestScanSkipsDirectoriesAndOtherSuffixes(t *testing.T) {
	db, err := ledger.Open(filepath.Join(t.TempDir(), "ledger.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "sub", "rollout.jsonl"), []byte("also not json\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	p := &Poller{DB: db, Dir: dir, Warnf: func(string, ...any) {}}
	if _, _, err := p.ScanOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
}
