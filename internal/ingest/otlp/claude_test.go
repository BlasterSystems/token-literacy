package otlp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/BlasterSystems/token-literacy/internal/ledger"
)

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "testdata", "otlp", name))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	return data
}

func TestSpanCalls(t *testing.T) {
	calls, skipped, err := SpanCalls(fixture(t, "llm_request_span.json"), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(skipped) != 0 {
		t.Fatalf("skipped = %v", skipped)
	}
	if len(calls) != 1 {
		t.Fatalf("calls = %d", len(calls))
	}
	c := calls[0]
	if c.ID != "claude-code:req_01HTZ0TEST" {
		t.Errorf("id = %s", c.ID)
	}
	if c.InputUncached != 707 || c.CacheRead != 28032 || c.CacheWrite != 0 || c.Output != 11 {
		t.Errorf("split = %d/%d/%d/%d", c.InputUncached, c.CacheRead, c.CacheWrite, c.Output)
	}
	if c.SessionID != "sess-abc123" || c.Model != "claude-sonnet-4-5-20250929" {
		t.Errorf("session/model = %s/%s", c.SessionID, c.Model)
	}
	// Sonnet 4.5 list: in 3, read 0.3, write 3.75, out 15 per MTok.
	want := (707*3 + 28032*0.3 + 0 + 11*15) / 1e6
	if c.CostUSD == nil || *c.CostUSD != want || c.CostSource != "list_price" {
		t.Errorf("cost = %v, want %v", c.CostUSD, want)
	}
	if c.StartedAt != 1700000000000 || c.EndedAt == nil || *c.EndedAt != 1700000002000 {
		t.Errorf("times = %d %v", c.StartedAt, c.EndedAt)
	}
	if c.DurationMs == nil || *c.DurationMs != 2000 || c.TTFTMs == nil || *c.TTFTMs != 350 {
		t.Errorf("durations = %v %v", c.DurationMs, c.TTFTMs)
	}
	if c.HarnessVersion == nil || *c.HarnessVersion != "2.1.268" {
		t.Errorf("harness version = %v", c.HarnessVersion)
	}
}

func TestSpanSkips(t *testing.T) {
	body := []byte(`{"resourceSpans":[{"scopeSpans":[{"spans":[
		{"name":"claude_code.llm_request","startTimeUnixNano":"1000","attributes":[{"key":"model","value":{"stringValue":"m"}}]},
		{"name":"other.span","startTimeUnixNano":"1000"},
		{"name":"claude_code.llm_request","startTimeUnixNano":"1000","attributes":[{"key":"output_tokens","value":{"intValue":"5"}}]}
	]}]}]}`)
	calls, skipped, err := SpanCalls(body, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(calls) != 0 {
		t.Errorf("calls = %d, want 0", len(calls))
	}
	if len(skipped) != 2 {
		t.Fatalf("skipped = %v, want 2", skipped)
	}
}

func TestSpanIntAsNumber(t *testing.T) {
	// Some exporters emit intValue as a JSON number.
	body := []byte(`{"resourceSpans":[{"scopeSpans":[{"spans":[
		{"name":"claude_code.llm_request","startTimeUnixNano":"1000",
		 "attributes":[{"key":"request_id","value":{"stringValue":"r"}},{"key":"input_tokens","value":{"intValue":42}}]}
	]}]}]}`)
	calls, _, err := SpanCalls(body, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(calls) != 1 || calls[0].InputUncached != 42 {
		t.Fatalf("calls = %+v", calls)
	}
}

func TestMetricCallsDelta(t *testing.T) {
	calls, skipped, err := MetricCalls(fixture(t, "token_usage_metrics.json"), MetricState{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(skipped) != 0 || len(calls) != 1 {
		t.Fatalf("calls=%d skipped=%v", len(calls), skipped)
	}
	c := calls[0]
	if c.AgentType == nil || *c.AgentType != "aggregate" {
		t.Errorf("agent_type = %v", c.AgentType)
	}
	if c.InputUncached != 707 || c.CacheRead != 28032 || c.Output != 11 {
		t.Errorf("split = %d/%d/%d", c.InputUncached, c.CacheRead, c.Output)
	}
	if c.SessionID != "aggregate" {
		t.Errorf("session = %s", c.SessionID)
	}
}

func TestMetricCallsCumulative(t *testing.T) {
	state := MetricState{}
	mk := func(input, output int64) []byte {
		dp := func(t string, v int64) map[string]any {
			return map[string]any{
				"timeUnixNano": "100",
				"asInt":        v,
				"attributes": []map[string]any{
					{"key": "type", "value": map[string]any{"stringValue": t}},
					{"key": "model", "value": map[string]any{"stringValue": "m"}},
				},
			}
		}
		body := map[string]any{
			"resourceMetrics": []map[string]any{{
				"scopeMetrics": []map[string]any{{
					"metrics": []map[string]any{{
						"name": "claude_code.token.usage",
						"sum": map[string]any{
							"aggregationTemporality": 1,
							"dataPoints":             []map[string]any{dp("input", input), dp("output", output)},
						},
					}},
				}},
			}},
		}
		b, _ := json.Marshal(body)
		return b
	}
	calls, _, err := MetricCalls(mk(100, 200), state, nil)
	if err != nil || len(calls) != 1 {
		t.Fatalf("first: calls=%d err=%v", len(calls), err)
	}
	if calls[0].InputUncached != 100 {
		t.Errorf("first window input = %d", calls[0].InputUncached)
	}
	calls, _, err = MetricCalls(mk(150, 300), state, nil)
	if err != nil || len(calls) != 1 {
		t.Fatalf("second: calls=%d err=%v", len(calls), err)
	}
	if calls[0].InputUncached != 50 {
		t.Errorf("second window input = %d, want 50 (cumulative delta)", calls[0].InputUncached)
	}
	// Counter reset (exporter restart) yields the raw value.
	calls, _, err = MetricCalls(mk(10, 20), state, nil)
	if err != nil || len(calls) != 1 || calls[0].InputUncached != 10 {
		t.Fatalf("reset: calls=%+v err=%v", calls, err)
	}
}

// Review finding (major): two models in one export window must produce two
// rows, not one merged row with the wrong model.
func TestMetricCallsGroupsByModel(t *testing.T) {
	body := map[string]any{
		"resourceMetrics": []map[string]any{{
			"scopeMetrics": []map[string]any{{
				"metrics": []map[string]any{{
					"name": "claude_code.token.usage",
					"sum": map[string]any{
						"aggregationTemporality": 2,
						"dataPoints": []map[string]any{
							dp("input", 10, "claude-sonnet-4-5"),
							dp("output", 5, "claude-sonnet-4-5"),
							dp("input", 7, "claude-haiku-4-5"),
							dp("output", 2, "claude-haiku-4-5"),
						},
					},
				}},
			}},
		}},
	}
	b, _ := json.Marshal(body)
	calls, _, err := MetricCalls(b, MetricState{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(calls) != 2 {
		t.Fatalf("calls = %d, want 2 (one per model)", len(calls))
	}
	byModel := map[string]ledger.Call{}
	for _, c := range calls {
		byModel[c.Model] = c
	}
	if byModel["claude-sonnet-4-5"].InputUncached != 10 || byModel["claude-haiku-4-5"].InputUncached != 7 {
		t.Fatalf("per-model input wrong: %+v", calls)
	}
}

func dp(typ string, v int64, model string) map[string]any {
	return map[string]any{
		"timeUnixNano": "100",
		"asInt":        v,
		"attributes": []map[string]any{
			{"key": "type", "value": map[string]any{"stringValue": typ}},
			{"key": "model", "value": map[string]any{"stringValue": model}},
		},
	}
}

func TestZeroTotalMetricSkipped(t *testing.T) {
	body := map[string]any{
		"resourceMetrics": []map[string]any{{
			"scopeMetrics": []map[string]any{{
				"metrics": []map[string]any{{
					"name": "claude_code.token.usage",
					"sum": map[string]any{
						"aggregationTemporality": 2,
						"dataPoints": []map[string]any{{
							"timeUnixNano": "1",
							"asInt":        0,
							"attributes": []map[string]any{
								{"key": "type", "value": map[string]any{"stringValue": "input"}},
							},
						}},
					},
				}},
			}},
		}},
	}
	b, _ := json.Marshal(body)
	calls, _, err := MetricCalls(b, MetricState{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(calls) != 0 {
		t.Fatalf("zero-value windows must not become rows: %+v", calls)
	}
}

func TestUnknownModelNoCost(t *testing.T) {
	body := []byte(`{"resourceSpans":[{"scopeSpans":[{"spans":[
		{"name":"claude_code.llm_request","startTimeUnixNano":"1000",
		 "attributes":[{"key":"request_id","value":{"stringValue":"r"}},{"key":"input_tokens","value":{"intValue":"5"}},{"key":"output_tokens","value":{"intValue":"2"}},{"key":"model","value":{"stringValue":"mystery-model"}}]}
	]}]}]}`)
	calls, _, err := SpanCalls(body, nil)
	if err != nil {
		t.Fatal(err)
	}
	if calls[0].CostUSD != nil || calls[0].CostSource != "none" {
		t.Errorf("unknown model cost = %v / %s", calls[0].CostUSD, calls[0].CostSource)
	}
}

func itoa(v int64) string {
	b, _ := json.Marshal(v)
	return string(b)
}

// Compile-time: MetricState satisfies the persisted-state contract shape.
var _ = ledger.Call{}
