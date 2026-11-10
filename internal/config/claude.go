// Package config edits harness configuration files safely: idempotent
// merges, timestamped-free backups, diffs, and marker-key cleanup.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// ClaudeEnvBlock is the env block tokenledger writes into Claude Code
// user-level settings (docs/PLAN.md M1).
func ClaudeEnvBlock(endpoint string) map[string]string {
	return map[string]string{
		"CLAUDE_CODE_ENABLE_TELEMETRY":        "1",
		"OTEL_METRICS_EXPORTER":               "otlp",
		"OTEL_LOGS_EXPORTER":                  "none",
		"OTEL_EXPORTER_OTLP_PROTOCOL":         "http/json",
		"OTEL_EXPORTER_OTLP_ENDPOINT":         endpoint,
		"OTEL_METRIC_EXPORT_INTERVAL":         "5000",
		"CLAUDE_CODE_ENHANCED_TELEMETRY_BETA": "1",
		"OTEL_TRACES_EXPORTER":                "otlp",
		"OTEL_TRACES_EXPORT_INTERVAL":         "2000",
	}
}

// markerPrefixes identify env keys tokenledger owns; uninstall removes them.
var markerPrefixes = []string{
	"OTEL_EXPORTER_OTLP",
	"OTEL_METRICS_EXPORTER",
	"OTEL_LOGS_EXPORTER",
	"OTEL_TRACES_EXPORTER",
	"OTEL_METRIC_EXPORT_INTERVAL",
	"OTEL_TRACES_EXPORT_INTERVAL",
	"CLAUDE_CODE_ENABLE_TELEMETRY",
	"CLAUDE_CODE_ENHANCED_TELEMETRY_BETA",
}

// IsMarkerKey reports whether an env key was written by tokenledger.
func IsMarkerKey(key string) bool {
	for _, p := range markerPrefixes {
		if len(key) >= len(p) && key[:len(p)] == p {
			return true
		}
	}
	return false
}

// ClaudeSettingsPath resolves the Claude Code user settings path, honoring
// CLAUDE_CONFIG_DIR. home is the user home directory; configDir may be "".
func ClaudeSettingsPath(home, configDir string) string {
	if configDir == "" {
		configDir = os.Getenv("CLAUDE_CONFIG_DIR")
	}
	if configDir != "" {
		return filepath.Join(configDir, "settings.json")
	}
	return filepath.Join(home, ".claude", "settings.json")
}

// Options controls ApplyClaudeSetup.
type Options struct {
	DryRun bool
	Force  bool // rewrite even when the merge changes nothing
}

// Result reports what ApplyClaudeSetup did or would do.
type Result struct {
	Path    string
	Changed bool
	Diff    []string // "+key" added, "~key: old -> new" changed, "-key" removed
	Backup  string   // backup file written ("" when none)
	DryRun  bool
}

// ApplyClaudeSetup merges env into the settings file's top-level "env" object.
// Existing non-env content is preserved. A backup is written next to the file
// before any change. Unparsable files are refused, never overwritten.
func ApplyClaudeSetup(path string, env map[string]string, opts Options) (Result, error) {
	res := Result{Path: path, DryRun: opts.DryRun}

	data, err := os.ReadFile(path)
	switch {
	case err == nil:
	case os.IsNotExist(err):
		data = nil
	default:
		return res, fmt.Errorf("read %s: %w", path, err)
	}

	var root map[string]any
	if data != nil {
		if err := json.Unmarshal(data, &root); err != nil {
			return res, fmt.Errorf("refusing to edit unparsable settings file %s: %w", path, err)
		}
	}
	if root == nil {
		root = map[string]any{}
	}

	envObj, _ := root["env"].(map[string]any)
	if envObj == nil {
		envObj = map[string]any{}
	}

	// Compute the merged env.
	merged := map[string]any{}
	for k, v := range envObj {
		merged[k] = v
	}
	for k, v := range env {
		merged[k] = v
	}
	if opts.Force {
		for k := range merged {
			if IsMarkerKey(k) {
				if _, ours := env[k]; !ours {
					delete(merged, k)
				}
			}
		}
	}

	// Diff.
	for _, k := range sortedKeys(env) {
		newV := env[k]
		old, existed := envObj[k]
		switch {
		case !existed:
			res.Diff = append(res.Diff, "+"+k+": "+fmt.Sprint(newV))
		case fmt.Sprint(old) != fmt.Sprint(newV):
			res.Diff = append(res.Diff, "~"+k+": "+fmt.Sprint(old)+" -> "+fmt.Sprint(newV))
		}
	}
	for _, k := range sortedKeys(envObj) {
		if _, ours := env[k]; ours {
			continue
		}
		if opts.Force && IsMarkerKey(k) {
			res.Diff = append(res.Diff, "-"+k)
		}
	}

	newEnvJSON, err := json.MarshalIndent(merged, "", "  ")
	if err != nil {
		return res, err
	}
	root["env"] = json.RawMessage(newEnvJSON)

	out, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return res, err
	}
	out = append(out, '\n')

	res.Changed = data == nil || string(out) != string(data)
	if !res.Changed && !opts.Force {
		return res, nil
	}
	if opts.DryRun {
		return res, nil
	}

	if data != nil {
		backup := path + ".bak-tokenledger"
		if err := os.WriteFile(backup, data, 0o600); err != nil {
			return res, fmt.Errorf("write backup: %w", err)
		}
		res.Backup = backup
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return res, fmt.Errorf("create settings directory: %w", err)
	}
	if err := os.WriteFile(path, out, 0o600); err != nil {
		return res, fmt.Errorf("write settings: %w", err)
	}
	return res, nil
}

// UninstallClaudeSetup removes tokenledger-owned env keys from the settings
// file. It reports whether the file changed and refuses unparsable files.
func UninstallClaudeSetup(path string) (bool, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	var root map[string]any
	if err := json.Unmarshal(data, &root); err != nil {
		return false, fmt.Errorf("refusing to edit unparsable settings file %s: %w", path, err)
	}
	envObj, _ := root["env"].(map[string]any)
	if envObj == nil {
		return false, nil
	}
	removed := false
	for k := range envObj {
		if IsMarkerKey(k) {
			delete(envObj, k)
			removed = true
		}
	}
	if !removed {
		return false, nil
	}
	root["env"] = envObj
	out, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return false, err
	}
	out = append(out, '\n')
	if err := os.WriteFile(path+".bak-tokenledger", data, 0o600); err != nil {
		return false, fmt.Errorf("write backup: %w", err)
	}
	return true, os.WriteFile(path, out, 0o600)
}

// ClaudeConfigPresent reports whether the settings file carries the
// tokenledger env block and which keys are missing/differing.
func ClaudeConfigPresent(path string, env map[string]string) (present bool, problems []string, err error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return false, []string{"settings file does not exist: " + path}, nil
	}
	if err != nil {
		return false, nil, err
	}
	var root map[string]any
	if err := json.Unmarshal(data, &root); err != nil {
		return false, []string{"settings file is not valid JSON: " + path}, nil
	}
	envObj, _ := root["env"].(map[string]any)
	if envObj == nil {
		return false, []string{"no env block in " + path}, nil
	}
	for k, want := range env {
		got, ok := envObj[k]
		if !ok {
			problems = append(problems, "missing "+k)
		} else if fmt.Sprint(got) != want {
			problems = append(problems, fmt.Sprintf("%s is %v, want %s", k, got, want))
		}
	}
	return len(problems) == 0, problems, nil
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
