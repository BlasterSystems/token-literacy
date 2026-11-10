package main

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BlasterSystems/token-literacy/internal/ledger"
)

func TestCmdExportCSVToFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	dbPath := filepath.Join(dir, "ledger.db")
	db, err := ledger.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.InsertCall(ledger.Call{
		ID: "codex:r1", Harness: "codex", SessionID: "s", RequestID: "r1",
		InputUncached: 10, CacheRead: 5, Output: 2, Source: "codex_rollout",
	}); err != nil {
		t.Fatal(err)
	}
	db.Close()

	out := filepath.Join(dir, "export.csv")
	if err := cmdExport([]string{"--csv", "--out", out, "--db", dbPath}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	records, err := csv.NewReader(strings.NewReader(string(data))).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 2 || records[0][0] != "id" || records[1][0] != "codex:r1" {
		t.Fatalf("csv = %v", records)
	}
}

func TestCmdExportJSONRequiresChoice(t *testing.T) {
	if err := cmdExport([]string{}); err == nil {
		t.Fatal("no format should error")
	}
	if err := cmdExport([]string{"--csv", "--json"}); err == nil {
		t.Fatal("both formats should error")
	}
}

func TestCmdExportMissingDB(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	if err := cmdExport([]string{"--csv", "--db", filepath.Join(dir, "nope.db")}); err == nil {
		t.Fatal("missing db should error")
	}
}
