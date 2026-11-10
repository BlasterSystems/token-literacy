package config

import "testing"

func TestClaudeEnvBlockKeys(t *testing.T) {
	env := ClaudeEnvBlock("http://127.0.0.1:4318")
	want := map[string]string{
		"CLAUDE_CODE_ENABLE_TELEMETRY":        "1",
		"OTEL_METRICS_EXPORTER":               "otlp",
		"OTEL_LOGS_EXPORTER":                  "none",
		"OTEL_EXPORTER_OTLP_PROTOCOL":         "http/json",
		"OTEL_EXPORTER_OTLP_ENDPOINT":         "http://127.0.0.1:4318",
		"OTEL_METRIC_EXPORT_INTERVAL":         "5000",
		"CLAUDE_CODE_ENHANCED_TELEMETRY_BETA": "1",
		"OTEL_TRACES_EXPORTER":                "otlp",
		"OTEL_TRACES_EXPORT_INTERVAL":         "2000",
	}
	if len(env) != len(want) {
		t.Fatalf("env block keys = %v", env)
	}
	for k, v := range want {
		if env[k] != v {
			t.Errorf("%s = %s, want %s", k, env[k], v)
		}
	}
}
