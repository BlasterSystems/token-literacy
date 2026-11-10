package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestApplyCodexOtelCreatesAndAppends(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	res, err := ApplyCodexOtelSetup(path, "http://127.0.0.1:4318", false)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Changed {
		t.Fatal("expected change")
	}
	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), "[otel.metrics_exporter.otlp-http]") ||
		!strings.Contains(string(data), `endpoint = "http://127.0.0.1:4318"`) {
		t.Fatalf("block missing: %s", data)
	}

	// Idempotent.
	res, err = ApplyCodexOtelSetup(path, "http://127.0.0.1:4318", false)
	if err != nil {
		t.Fatal(err)
	}
	if res.Changed {
		t.Fatalf("second apply changed: %+v", res)
	}

	// Existing content preserved.
	if err := os.WriteFile(path, []byte("model = \"gpt-5.2\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ApplyCodexOtelSetup(path, "http://127.0.0.1:4318", false); err != nil {
		t.Fatal(err)
	}
	data, _ = os.ReadFile(path)
	if !strings.HasPrefix(string(data), "model = \"gpt-5.2\"") {
		t.Fatalf("existing content lost: %s", data)
	}
}

func TestApplyCodexOtelReplacesOldBlock(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if _, err := ApplyCodexOtelSetup(path, "http://127.0.0.1:9999", false); err != nil {
		t.Fatal(err)
	}
	res, err := ApplyCodexOtelSetup(path, "http://127.0.0.1:4318", false)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Changed {
		t.Fatal("endpoint change should rewrite block")
	}
	data, _ := os.ReadFile(path)
	if strings.Contains(string(data), "9999") {
		t.Fatalf("stale endpoint survived: %s", data)
	}
	if strings.Count(string(data), "BEGIN tokenledger") != 1 {
		t.Fatalf("duplicate blocks: %s", data)
	}
}

func TestUninstallCodexOtel(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte("model = \"gpt-5.2\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ApplyCodexOtelSetup(path, "http://127.0.0.1:4318", false); err != nil {
		t.Fatal(err)
	}
	changed, err := UninstallCodexOtelSetup(path)
	if err != nil || !changed {
		t.Fatalf("uninstall: %v %v", changed, err)
	}
	data, _ := os.ReadFile(path)
	if strings.Contains(string(data), "tokenledger") || strings.Contains(string(data), "otel") {
		t.Fatalf("block survived: %s", data)
	}
	if !strings.HasPrefix(string(data), "model = \"gpt-5.2\"") {
		t.Fatalf("content lost: %s", data)
	}
}

func TestCodexConfigPathEnv(t *testing.T) {
	t.Setenv("CODEX_HOME", "")
	if got := CodexConfigPath("/home/u", ""); got != filepath.Join("/home/u", ".codex", "config.toml") {
		t.Errorf("path = %s", got)
	}
	t.Setenv("CODEX_HOME", "/ch")
	if got := CodexConfigPath("/home/u", ""); got != filepath.Join("/ch", "config.toml") {
		t.Errorf("path = %s", got)
	}
}
