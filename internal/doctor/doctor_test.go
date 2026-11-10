package doctor

import (
	"bytes"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/BlasterSystems/token-literacy/internal/ledger"
)

func TestRunAllFailWithoutSetup(t *testing.T) {
	home := t.TempDir()
	r := Run(Inputs{
		DBPath:         filepath.Join(home, "missing.db"),
		ClaudeSettings: filepath.Join(home, ".claude", "settings.json"),
		ExpectedClaude: map[string]string{},
		CollectorURL:   "http://127.0.0.1:1/health", // nothing listens
		OverridePath:   filepath.Join(home, "no-pricing.json"),
		HTTPClient:     &http.Client{Timeout: time.Second},
	})
	if r.Failures() == 0 {
		t.Fatalf("expected failures, got %+v", r.Checks)
	}
}

func TestRunHealthy(t *testing.T) {
	dir := t.TempDir()
	db, err := ledger.Open(filepath.Join(dir, "ledger.db"))
	if err != nil {
		t.Fatal(err)
	}
	c := ledger.Call{
		ID: "codex:r1", Harness: "codex", SessionID: "s", RequestID: "r1",
		InputUncached: 10, CacheRead: 5, Output: 2, Source: "codex_rollout",
		StartedAt: time.Now().UnixMilli(),
	}
	if _, err := db.InsertCall(c); err != nil {
		t.Fatal(err)
	}
	db.Close()

	// Fake collector serving the real handler shape.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"status": "ok", "version": "test"})
	}))
	defer ts.Close()

	// Valid claude settings.
	settings := filepath.Join(dir, "settings.json")
	os.WriteFile(settings, []byte(`{"env":{"CLAUDE_CODE_ENABLE_TELEMETRY":"1"}}`), 0o600)

	r := Run(Inputs{
		DBPath:         filepath.Join(dir, "ledger.db"),
		ClaudeSettings: settings,
		ExpectedClaude: map[string]string{"CLAUDE_CODE_ENABLE_TELEMETRY": "1"},
		CollectorURL:   ts.URL + "/health",
		OverridePath:   filepath.Join(dir, "pricing.json"),
		HTTPClient:     ts.Client(),
	})
	for _, c := range r.Checks {
		if c.Status == "fail" {
			t.Errorf("unexpected failure: %s: %s", c.Name, c.Detail)
		}
	}
}

func TestClaudeConfigMismatchFails(t *testing.T) {
	dir := t.TempDir()
	settings := filepath.Join(dir, "settings.json")
	os.WriteFile(settings, []byte(`{"env":{"CLAUDE_CODE_ENABLE_TELEMETRY":"0"}}`), 0o600)
	r := Run(Inputs{
		ClaudeSettings: settings,
		ExpectedClaude: map[string]string{"CLAUDE_CODE_ENABLE_TELEMETRY": "1"},
	})
	var found bool
	for _, c := range r.Checks {
		if c.Name == "claude-config" && c.Status == "fail" {
			found = true
		}
	}
	if !found {
		t.Fatal("mismatched config should fail")
	}
}

func TestUnparsablePricingOverrideFails(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pricing.json")
	os.WriteFile(path, []byte("{bad"), 0o600)
	r := Run(Inputs{OverridePath: path})
	var found bool
	for _, c := range r.Checks {
		if c.Name == "pricing-override" && c.Status == "fail" {
			found = true
		}
	}
	if !found {
		t.Fatal("unparsable override should fail")
	}
}

func TestRender(t *testing.T) {
	r := Report{Checks: []Check{
		{Name: "ledger", Status: "ok", Detail: "fine"},
		{Name: "collector", Status: "fail", Detail: "down"},
	}}
	var buf bytes.Buffer
	r.Render(&buf)
	out := buf.String()
	if !strings.Contains(out, "ok") || !strings.Contains(out, "fail") || !strings.Contains(out, "ledger") {
		t.Fatalf("render = %q", out)
	}
}

func TestCodexDirChecks(t *testing.T) {
	if c := checkCodexDir(t.TempDir()); c.Status != "ok" {
		t.Fatalf("existing dir: %+v", c)
	}
	missing := filepath.Join(t.TempDir(), "nope")
	if c := checkCodexDir(missing); c.Status != "fail" {
		t.Fatalf("missing dir: %+v", c)
	}
	file := filepath.Join(t.TempDir(), "f")
	os.WriteFile(file, []byte("x"), 0o600)
	if c := checkCodexDir(file); c.Status != "fail" {
		t.Fatalf("file dir: %+v", c)
	}
}

func TestInstanceCheck(t *testing.T) {
	dir := t.TempDir()
	// No lock file: ok.
	if c := checkInstance(filepath.Join(dir, "lock")); c.Status != "ok" {
		t.Fatalf("no lock: %+v", c)
	}
	// Own live pid: warn.
	lock := filepath.Join(dir, "lock")
	os.WriteFile(lock, []byte(strconv.Itoa(os.Getpid())), 0o600)
	if c := checkInstance(lock); c.Status != "warn" {
		t.Fatalf("live lock: %+v", c)
	}
	// Stale pid: ok.
	os.WriteFile(lock, []byte("999999"), 0o600)
	if c := checkInstance(lock); c.Status != "ok" {
		t.Fatalf("stale lock: %+v", c)
	}
}

func TestPortCheck(t *testing.T) {
	// Nothing listens on a high port: warn.
	if c := checkPort(45999); c.Status != "warn" {
		t.Fatalf("closed port: %+v", c)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	port := ln.Addr().(*net.TCPAddr).Port
	if c := checkPort(port); c.Status != "ok" {
		t.Fatalf("open port: %+v", c)
	}
}

func TestCheckHarnessVersionMissing(t *testing.T) {
	if c := CheckHarnessVersion("definitely-not-a-binary-xyz"); c.Status != "warn" {
		t.Fatalf("missing binary: %+v", c)
	}
}

func TestRunIncludesWSLCheck(t *testing.T) {
	r := Run(Inputs{})
	var found bool
	for _, c := range r.Checks {
		if c.Name == "wsl" {
			found = true
		}
	}
	if !found {
		t.Fatal("wsl check missing")
	}
}
