package codex

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "testdata", "codex", name))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	return data
}

// parse mirrors the strict newline contract of real rollouts: the tailer
// only consumes newline-terminated lines.
func parse(t *testing.T, body string) Result {
	t.Helper()
	if !strings.HasSuffix(body, "\n") {
		body += "\n"
	}
	return ParseResult([]byte(body), nil, 0)
}

func TestParseRecord(t *testing.T) {
	res := ParseResult(fixture(t, "record.jsonl"), nil, 0)
	if len(res.Warnings) != 0 {
		t.Fatalf("warnings = %v", res.Warnings)
	}
	if len(res.Calls) != 1 {
		t.Fatalf("calls = %d", len(res.Calls))
	}
	c := res.Calls[0]
	// U = input - cached = 707; R = 28032; W = 0; O = 11.
	if c.ID != "codex:resp_fixture0001" || c.RequestID != "resp_fixture0001" {
		t.Errorf("ids = %s / %s", c.ID, c.RequestID)
	}
	if c.InputUncached != 707 || c.CacheRead != 28032 || c.CacheWrite != 0 || c.Output != 11 {
		t.Errorf("split = %d/%d/%d/%d", c.InputUncached, c.CacheRead, c.CacheWrite, c.Output)
	}
	if c.PromptTotal != 28739 || c.TotalTokens != 28750 {
		t.Errorf("totals = %d/%d", c.PromptTotal, c.TotalTokens)
	}
	if c.Model != "gpt-5.2" {
		t.Errorf("model = %s (from turn_context)", c.Model)
	}
	if c.HarnessVersion == nil || *c.HarnessVersion != "0.166.0" {
		t.Errorf("harness version = %v", c.HarnessVersion)
	}
	wantStarted := time.Date(2026, 11, 7, 17, 44, 39, 275_000_000, time.UTC).UnixMilli()
	if c.StartedAt != wantStarted {
		t.Errorf("started_at = %d, want %d", c.StartedAt, wantStarted)
	}
	// gpt-5.2 list: in 1.25, read 0.125, out 10 per MTok.
	want := (707*1.25 + 28032*0.125 + 0 + 11*10) / 1e6
	if c.CostUSD == nil || *c.CostUSD != want {
		t.Errorf("cost = %v, want %v", c.CostUSD, want)
	}
}

func TestRecordWinsOverEvent(t *testing.T) {
	res := ParseResult(fixture(t, "record_and_event.jsonl"), nil, 0)
	if len(res.Calls) != 1 {
		t.Fatalf("calls = %d, want 1 (event_msg ignored when a record exists)", len(res.Calls))
	}
	if res.Calls[0].ID != "codex:resp_fixture0002" {
		t.Errorf("id = %s", res.Calls[0].ID)
	}
}

func TestEventOnlyFallback(t *testing.T) {
	res := ParseResult(fixture(t, "event_only.jsonl"), nil, 0)
	if len(res.Calls) != 1 {
		t.Fatalf("calls = %d", len(res.Calls))
	}
	c := res.Calls[0]
	if c.ID != "codex:e:01a1a72a-0d35-7111-bd84-3b9ad1107244:0:2" {
		t.Errorf("id = %s", c.ID)
	}
	if c.InputUncached != 29765-12000 || c.CacheRead != 12000 || c.Output != 153 {
		t.Errorf("split = %d/%d/%d", c.InputUncached, c.CacheRead, c.Output)
	}
	if c.Reasoning != 90 {
		t.Errorf("reasoning = %d, want 90 (stored, never added)", c.Reasoning)
	}
}

func TestSubagentAttribution(t *testing.T) {
	res := ParseResult(fixture(t, "subagent.jsonl"), nil, 0)
	if len(res.Calls) != 1 {
		t.Fatalf("calls = %d", len(res.Calls))
	}
	c := res.Calls[0]
	if c.AgentType == nil || *c.AgentType != "subagent" {
		t.Errorf("agent_type = %v", c.AgentType)
	}
	if c.AgentID == nil || *c.AgentID != "01parent-session" {
		t.Errorf("agent_id = %v", c.AgentID)
	}
	if c.SessionID != "01parent-session" {
		t.Errorf("session = %s", c.SessionID)
	}
}

func TestAdditiveInvariantWarns(t *testing.T) {
	// cached_input_tokens (900) exceeds input_tokens (1000): the additive
	// model cannot hold, so the record is refused rather than guessed.
	body := []byte(`{"timestamp":"2026-11-07T17:44:15.000Z","ordinal":0,"type":"session_meta","payload":{"session_id":"s1","cli_version":"0.166.0"}}
{"ordinal":1,"type":"turn_context","payload":{"model":"gpt-5.2"}}
{"ordinal":2,"type":"token_usage_record","payload":{"session_id":"s1","response_id":"r1","usage":{"input_tokens":1000,"cached_input_tokens":9000,"cache_write_input_tokens":0,"output_tokens":5,"reasoning_output_tokens":0,"total_tokens":1005}}}`)
	res := parse(t, string(body))
	if len(res.Calls) != 0 {
		t.Fatalf("failing record must not become a row: %+v", res.Calls)
	}
	if len(res.Warnings) != 1 || !contains(res.Warnings[0], "additive invariant") {
		t.Fatalf("warnings = %v", res.Warnings)
	}
}

func TestTotalMismatchWarns(t *testing.T) {
	body := []byte(`{"ordinal":0,"type":"session_meta","payload":{"session_id":"s1","cli_version":"0.166.0"}}
{"ordinal":1,"type":"token_usage_record","payload":{"session_id":"s1","response_id":"r1","usage":{"input_tokens":100,"cached_input_tokens":0,"cache_write_input_tokens":0,"output_tokens":5,"reasoning_output_tokens":0,"total_tokens":999}}}`)
	res := parse(t, string(body))
	if len(res.Calls) != 0 || len(res.Warnings) != 1 {
		t.Fatalf("calls=%d warnings=%v", len(res.Calls), res.Warnings)
	}
}

func TestUnknownModelDefaultPrice(t *testing.T) {
	body := []byte(`{"ordinal":0,"type":"session_meta","payload":{"session_id":"s1","cli_version":"0.166.0"}}
{"ordinal":1,"type":"turn_context","payload":{"model":"mystery-model"}}
{"ordinal":2,"type":"token_usage_record","payload":{"session_id":"s1","response_id":"r1","usage":{"input_tokens":100,"cached_input_tokens":50,"cache_write_input_tokens":0,"output_tokens":5,"reasoning_output_tokens":0,"total_tokens":105}}}`)
	res := parse(t, string(body))
	if len(res.Calls) != 1 {
		t.Fatalf("calls = %d", len(res.Calls))
	}
	// Unknown models are priced at the opus-5.5 default: U=50 R=50 O=5 →
	// (50*4 + 50*0.2 + 5*20)/1e6 = 0.00031.
	if res.Calls[0].CostUSD == nil || res.Calls[0].CostSource != "default_price" {
		t.Fatalf("cost = %v / %s, want default_price", res.Calls[0].CostUSD, res.Calls[0].CostSource)
	}
	if *res.Calls[0].CostUSD != 0.00031 {
		t.Errorf("default cost = %v, want 0.00031", *res.Calls[0].CostUSD)
	}
}

func TestNoPromptContentInRaw(t *testing.T) {
	// A message payload with prompt-looking content must never reach raw.
	body := []byte(`{"ordinal":0,"type":"session_meta","payload":{"session_id":"s1","cli_version":"0.166.0"}}
{"ordinal":1,"type":"response_item","payload":{"type":"message","content":[{"text":"SECRET PROMPT CONTENT abcdef"}]}}
{"ordinal":2,"type":"token_usage_record","payload":{"session_id":"s1","response_id":"r1","usage":{"input_tokens":100,"cached_input_tokens":0,"cache_write_input_tokens":0,"output_tokens":5,"reasoning_output_tokens":0,"total_tokens":105}}}`)
	res := parse(t, string(body))
	if len(res.Calls) != 1 {
		t.Fatal("no call")
	}
	raw := *res.Calls[0].Raw
	if contains(raw, "SECRET PROMPT CONTENT") {
		t.Fatalf("prompt content leaked into raw: %s", raw)
	}
	if contains(raw, "base_instructions") {
		t.Fatalf("instructions leaked into raw: %s", raw)
	}
}

func contains(haystack, needle string) bool {
	return strings.Contains(haystack, needle)
}
