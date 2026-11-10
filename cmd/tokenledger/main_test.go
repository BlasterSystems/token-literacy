package main

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fakeHome(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	return dir
}

func TestSetupDryRunDoesNotWrite(t *testing.T) {
	home := fakeHome(t)
	if err := cmdSetup([]string{"--dry-run"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(home, ".claude", "settings.json")); !os.IsNotExist(err) {
		t.Fatal("dry run wrote settings")
	}
}

func TestSetupWritesAndIdempotent(t *testing.T) {
	home := fakeHome(t)
	if err := cmdSetup([]string{}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(home, ".claude", "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "CLAUDE_CODE_ENABLE_TELEMETRY") {
		t.Fatalf("env block missing: %s", data)
	}
	if err := cmdSetup([]string{}); err != nil {
		t.Fatal(err)
	}
}

func TestSetupRespectsConfigDir(t *testing.T) {
	home := fakeHome(t)
	t.Setenv("CLAUDE_CONFIG_DIR", filepath.Join(home, "custom"))
	if err := cmdSetup([]string{}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(home, "custom", "settings.json")); err != nil {
		t.Fatal(err)
	}
}

func TestRunDoctorWithFreshState(t *testing.T) {
	home := fakeHome(t)
	report, err := runDoctor([]string{"--db", filepath.Join(home, "nope.db")})
	if err != nil {
		t.Fatal(err)
	}
	if report.Failures() == 0 {
		t.Fatalf("fresh state must report failures: %+v", report.Checks)
	}
}

func TestParsePort(t *testing.T) {
	if p, err := parsePort("127.0.0.1:4318"); err != nil || p != 4318 {
		t.Fatalf("parsePort = %d, %v", p, err)
	}
	if _, err := parsePort("localhost:1"); err == nil {
		t.Fatal("bad addr accepted")
	}
}

func TestDefaultDBPathFromEnv(t *testing.T) {
	t.Setenv("TOKENLEDGER_DB", "/tmp/custom-ledger.db")
	if got := defaultDBPath(); got != "/tmp/custom-ledger.db" {
		t.Fatalf("defaultDBPath = %s", got)
	}
}

func TestCmdSimulateFixture(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	dbPath := filepath.Join(dir, "ledger.db")
	fixtureDir := t.TempDir()
	copyPath := func(dst, src string) {
		data, err := os.ReadFile(src)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(dst, data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	copyPath(filepath.Join(fixtureDir, "span.json"), filepath.Join("..", "..", "testdata", "otlp", "llm_request_span.json"))
	copyPath(filepath.Join(fixtureDir, "record.jsonl"), filepath.Join("..", "..", "testdata", "codex", "record.jsonl"))

	if err := cmdSimulate([]string{"--fixture", fixtureDir, "--db", dbPath}); err != nil {
		t.Fatal(err)
	}
	if err := cmdSimulate([]string{"--db", dbPath}); err == nil {
		t.Fatal("missing --fixture should error")
	}
}

func TestCmdInstallUninstallFakeHome(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	// install would run launchctl load and fail outside a real session;
	// uninstall must be safe and idempotent regardless.
	if err := cmdUninstall([]string{}); err != nil {
		t.Fatal(err)
	}
	if err := cmdUninstall([]string{"--purge"}); err != nil {
		t.Fatal(err)
	}
}

func TestStartServeIngestAndQuery(t *testing.T) {
	home := fakeHome(t)
	dbPath := filepath.Join(home, "ledger.db")
	r, err := startServe(0, dbPath, "", true) // no codex poller in the test
	if err != nil {
		t.Fatal(err)
	}
	defer r.stop()

	base := "http://" + r.Addr
	resp, err := http.Get(base + "/health")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var health map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
		t.Fatal(err)
	}
	if health["status"] != "ok" {
		t.Fatalf("health = %v", health)
	}

	fixture, err := os.ReadFile(filepath.Join("..", "..", "testdata", "otlp", "llm_request_span.json"))
	if err != nil {
		t.Fatal(err)
	}
	resp2, err := http.Post(base+"/v1/traces", "application/json", strings.NewReader(string(fixture)))
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()
	body, _ := io.ReadAll(resp2.Body)
	if resp2.StatusCode != 200 || string(body) != "{}" {
		t.Fatalf("ingest = %d %s", resp2.StatusCode, body)
	}

	resp3, err := http.Get(base + "/api/summary")
	if err != nil {
		t.Fatal(err)
	}
	defer resp3.Body.Close()
	var summary map[string]any
	json.NewDecoder(resp3.Body).Decode(&summary)
	if summary["total_tokens"] != float64(28750) {
		t.Fatalf("total_tokens = %v", summary["total_tokens"])
	}

	if err := r.stop(); err != nil {
		t.Fatalf("stop: %v", err)
	}
	// Server must be down after stop.
	if _, err := http.Get(base + "/health"); err == nil {
		t.Fatal("server still up after stop")
	}
}
