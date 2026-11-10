package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// CodexOtelBlock is the TOML section written for --with-codex-otel. It makes
// Codex export codex.turn.* metrics over OTLP/HTTP; token counts still come
// from rollouts.
func CodexOtelBlock(endpoint string) string {
	return fmt.Sprintf("[otel.metrics_exporter.otlp-http]\nendpoint = %q\nprotocol = \"http/json\"\n", endpoint)
}

const codexMarkerBegin = "# BEGIN tokenledger"
const codexMarkerEnd = "# END tokenledger"

// CodexConfigPath resolves ~/.codex/config.toml honoring CODEX_HOME.
func CodexConfigPath(home, codexHome string) string {
	if codexHome == "" {
		codexHome = os.Getenv("CODEX_HOME")
	}
	if codexHome != "" {
		return filepath.Join(codexHome, "config.toml")
	}
	return filepath.Join(home, ".codex", "config.toml")
}

// ApplyCodexOtelSetup merges the otel metrics block into config.toml.
// Existing content is preserved; a previous tokenledger block is replaced.
// Unparsable is not detectable without a TOML parser, so the merge is
// line-based and only touches the marked block.
func ApplyCodexOtelSetup(path, endpoint string, dryRun bool) (Result, error) {
	res := Result{Path: path, DryRun: dryRun}
	data, err := os.ReadFile(path)
	switch {
	case err == nil:
	case os.IsNotExist(err):
		data = nil
	default:
		return res, err
	}

	block := codexMarkerBegin + "\n" + CodexOtelBlock(endpoint) + codexMarkerEnd + "\n"
	lines := strings.Split(string(data), "\n")
	// An unmarked section with the same header would duplicate the TOML
	// table and make the file unparsable for Codex; refuse instead. Lines
	// inside our own marked block are skipped (idempotent re-runs).
	inOwnBlock := false
	for _, ln := range lines {
		trimmed := strings.TrimSpace(ln)
		switch trimmed {
		case codexMarkerBegin:
			inOwnBlock = true
			continue
		case codexMarkerEnd:
			inOwnBlock = false
			continue
		}
		if !inOwnBlock && trimmed == "[otel.metrics_exporter.otlp-http]" {
			return res, fmt.Errorf(
				"%s already defines [otel.metrics_exporter.otlp-http] outside the tokenledger block; merge it manually or delete the section, then rerun setup",
				path)
		}
	}
	var out []string
	inBlock := false
	replaced := false
	for _, ln := range lines {
		switch {
		case strings.TrimSpace(ln) == codexMarkerBegin:
			inBlock = true
			if !replaced {
				out = append(out, strings.TrimSuffix(block, "\n"))
				replaced = true
			}
		case strings.TrimSpace(ln) == codexMarkerEnd:
			inBlock = false
		default:
			if !inBlock {
				out = append(out, ln)
			}
		}
	}
	if !replaced {
		out = append(out, strings.TrimSuffix(block, "\n"))
	}
	// Normalize trailing newlines.
	merged := strings.TrimRight(strings.Join(out, "\n"), "\n") + "\n"

	res.Changed = string(data) != merged
	if !res.Changed {
		return res, nil
	}
	if strings.Contains(string(data), codexMarkerBegin) && !replaced {
		// unreachable (replaced would be true), kept for clarity
	}
	res.Diff = append(res.Diff, "+[otel.metrics_exporter.otlp-http] (endpoint "+endpoint+")")
	if dryRun {
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
		return res, err
	}
	return res, os.WriteFile(path, []byte(merged), 0o600)
}

// UninstallCodexOtelSetup removes the tokenledger block from config.toml.
func UninstallCodexOtelSetup(path string) (bool, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	lines := strings.Split(string(data), "\n")
	var out []string
	inBlock := false
	removed := false
	for _, ln := range lines {
		switch {
		case strings.TrimSpace(ln) == codexMarkerBegin:
			inBlock = true
			removed = true
		case strings.TrimSpace(ln) == codexMarkerEnd:
			inBlock = false
		default:
			if !inBlock {
				out = append(out, ln)
			}
		}
	}
	if !removed {
		return false, nil
	}
	merged := strings.TrimRight(strings.Join(out, "\n"), "\n") + "\n"
	if merged == string(data) {
		return false, nil
	}
	if err := os.WriteFile(path+".bak-tokenledger", data, 0o600); err != nil {
		return false, err
	}
	return true, os.WriteFile(path, []byte(merged), 0o600)
}
