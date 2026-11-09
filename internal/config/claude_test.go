package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func envBlock() map[string]string {
	return map[string]string{
		"CLAUDE_CODE_ENABLE_TELEMETRY": "1",
		"OTEL_METRICS_EXPORTER":        "otlp",
		"OTEL_EXPORTER_OTLP_ENDPOINT":  "http://127.0.0.1:4318",
	}
}

func TestApplyCreatesFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	res, err := ApplyClaudeSetup(path, envBlock(), Options{})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Changed || res.Backup != "" {
		t.Fatalf("res = %+v", res)
	}
	var root map[string]any
	data, _ := os.ReadFile(path)
	if err := json.Unmarshal(data, &root); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if root["env"] == nil {
		t.Fatal("env block missing")
	}
}

func TestApplyMergePreservesContent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	existing := `{"env":{"MY_KEY":"keep","OTEL_METRICS_EXPORTER":"none"},"model":"opus"}`
	if err := os.WriteFile(path, []byte(existing), 0o600); err != nil {
		t.Fatal(err)
	}
	res, err := ApplyClaudeSetup(path, envBlock(), Options{})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Changed {
		t.Fatal("should change")
	}
	var root map[string]any
	data, _ := os.ReadFile(path)
	json.Unmarshal(data, &root)
	env := root["env"].(map[string]any)
	if env["MY_KEY"] != "keep" {
		t.Errorf("unrelated env key lost: %v", env)
	}
	if env["OTEL_METRICS_EXPORTER"] != "otlp" {
		t.Errorf("existing key not updated: %v", env)
	}
	if root["model"] != "opus" {
		t.Errorf("top-level content lost: %v", root)
	}
	if res.Backup == "" {
		t.Fatal("no backup written")
	}
	if _, err := os.Stat(res.Backup); err != nil {
		t.Fatalf("backup missing: %v", err)
	}
}

func TestApplyIdempotentNoChange(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	if _, err := ApplyClaudeSetup(path, envBlock(), Options{}); err != nil {
		t.Fatal(err)
	}
	res, err := ApplyClaudeSetup(path, envBlock(), Options{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Changed {
		t.Fatalf("second apply changed the file: %v", res.Diff)
	}
}

func TestApplyDryRun(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	res, err := ApplyClaudeSetup(path, envBlock(), Options{DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Changed {
		t.Fatal("dry run should report would-change")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("dry run must not write")
	}
	if len(res.Diff) == 0 {
		t.Fatal("dry run diff empty")
	}
}

func TestApplyRefusesUnparsable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(path, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ApplyClaudeSetup(path, envBlock(), Options{}); err == nil {
		t.Fatal("unparsable file must be refused")
	}
	if data, _ := os.ReadFile(path); string(data) != "{not json" {
		t.Fatal("unparsable file was modified")
	}
}

func TestUninstall(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	if _, err := ApplyClaudeSetup(path, envBlock(), Options{}); err != nil {
		t.Fatal(err)
	}
	changed, err := UninstallClaudeSetup(path)
	if err != nil || !changed {
		t.Fatalf("uninstall: %v %v", changed, err)
	}
	var root map[string]any
	data, _ := os.ReadFile(path)
	json.Unmarshal(data, &root)
	env := root["env"].(map[string]any)
	for k := range env {
		if IsMarkerKey(k) {
			t.Errorf("marker key survived: %s", k)
		}
	}
}

func TestClaudeSettingsPathConfigDir(t *testing.T) {
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	if got := ClaudeSettingsPath("/home/u", ""); got != filepath.Join("/home/u", ".claude", "settings.json") {
		t.Errorf("path = %s", got)
	}
	if got := ClaudeSettingsPath("/home/u", "/custom"); got != filepath.Join("/custom", "settings.json") {
		t.Errorf("path = %s", got)
	}
	t.Setenv("CLAUDE_CONFIG_DIR", "/from-env")
	if got := ClaudeSettingsPath("/home/u", ""); got != filepath.Join("/from-env", "settings.json") {
		t.Errorf("path = %s", got)
	}
}

func TestClaudeConfigPresent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	present, _, err := ClaudeConfigPresent(path, envBlock())
	if err != nil || present {
		t.Fatalf("missing file: present=%v err=%v", present, err)
	}
	if _, err := ApplyClaudeSetup(path, envBlock(), Options{}); err != nil {
		t.Fatal(err)
	}
	present, problems, err := ClaudeConfigPresent(path, envBlock())
	if err != nil || !present || len(problems) != 0 {
		t.Fatalf("present=%v problems=%v err=%v", present, problems, err)
	}
}

func TestMarkerPrefixes(t *testing.T) {
	for _, k := range []string{"OTEL_EXPORTER_OTLP_ENDPOINT", "OTEL_METRIC_EXPORT_INTERVAL", "CLAUDE_CODE_ENABLE_TELEMETRY"} {
		if !IsMarkerKey(k) {
			t.Errorf("%s should be a marker", k)
		}
	}
	if IsMarkerKey("OTEL_OTHER") || IsMarkerKey("CLAUDE_CODE_SUBAGENT_MODEL") {
		t.Error("non-marker key matched")
	}
	if !strings.HasPrefix("x", "") {
		t.Fatal("sanity")
	}
}
