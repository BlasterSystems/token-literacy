package doctor

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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
