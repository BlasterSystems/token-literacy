package otlp

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"

	"github.com/BlasterSystems/token-literacy/internal/ledger"
	"github.com/BlasterSystems/token-literacy/internal/pricing"
)

const (
	// spanLLMRequest is the per-request span carrying the additive token split.
	spanLLMRequest = "claude_code.llm_request"
	// metricTokenUsage is the aggregate fallback metric.
	metricTokenUsage = "claude_code.token.usage"
)

// Skipped describes a payload element that was seen but not stored. These are
// logged server-side and never persisted.
type Skipped struct {
	Kind   string // span | point
	Reason string
}

// SpanCalls extracts one ledger.Call per claude_code.llm_request span.
func SpanCalls(body []byte, override pricing.Override) ([]ledger.Call, []Skipped, error) {
	var req traceRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, nil, fmt.Errorf("decode trace request: %w", err)
	}
	var calls []ledger.Call
	var skipped []Skipped
	seen := map[string]bool{}
	for _, rs := range req.ResourceSpans {
		for _, ss := range rs.ScopeSpans {
			harnessVersion := ""
			if ss.Scope != nil {
				harnessVersion = ss.Scope.Version
			}
			for _, sp := range ss.Spans {
				if sp.Name != spanLLMRequest {
					continue
				}
				call, reason, ok := claudeSpanToCall(sp, harnessVersion, override)
				if !ok {
					skipped = append(skipped, Skipped{Kind: "span", Reason: reason})
					continue
				}
				if seen[call.ID] {
					skipped = append(skipped, Skipped{Kind: "span", Reason: "duplicate request_id " + call.ID})
					continue
				}
				seen[call.ID] = true
				calls = append(calls, call)
			}
		}
	}
	return calls, skipped, nil
}

// rawSpanMetadata builds the trimmed raw payload: allow-listed attributes
// only, never message or tool content.
func rawSpanMetadata(name string, m map[string]any) string {
	allow := []string{
		"input_tokens", "output_tokens", "cache_read_tokens", "cache_creation_tokens",
		"model", "request_id", "session.id", "query_source", "duration_ms", "ttft_ms",
		"stop_reason", "attempt", "success", "terminal", "service_tier",
	}
	raw := map[string]any{"span": name}
	attrs := map[string]any{}
	for _, k := range allow {
		if v, ok := m[k]; ok {
			attrs[k] = v
		}
	}
	raw["attributes"] = attrs
	b, err := json.Marshal(raw)
	if err != nil {
		return "{}"
	}
	return string(b)
}

func claudeSpanToCall(sp span, harnessVersion string, override pricing.Override) (ledger.Call, string, bool) {
	m := attrMap(sp.Attributes)

	requestID, ok := asString(m, "request_id")
	if !ok {
		return ledger.Call{}, "missing request_id", false
	}
	input, hasInput := asInt(m, "input_tokens")
	output, hasOutput := asInt(m, "output_tokens")
	cacheRead, _ := asInt(m, "cache_read_tokens")
	cacheWrite, _ := asInt(m, "cache_creation_tokens")
	if !hasInput && !hasOutput {
		return ledger.Call{}, "no token attributes", false
	}
	startedAt, ok := parseNanoMs(sp.StartTimeUnixNano)
	if !ok {
		return ledger.Call{}, "missing start time", false
	}

	sessionID, _ := asString(m, "session.id")
	model, _ := asString(m, "model")

	call := ledger.Call{
		ID:             "claude-code:" + requestID,
		Harness:        "claude-code",
		SessionID:      sessionID,
		RequestID:      requestID,
		Model:          model,
		BillingMode:    "subscription",
		InputUncached:  input,
		CacheRead:      cacheRead,
		CacheWrite:     cacheWrite,
		Output:         output,
		Source:         "claude_otel",
		StartedAt:      startedAt,
		HarnessVersion: strPtr(harnessVersion),
	}
	if end, ok := parseNanoMs(sp.EndTimeUnixNano); ok && end >= startedAt {
		call.EndedAt = &end
	}
	if d, ok := asInt(m, "duration_ms"); ok {
		call.DurationMs = &d
	}
	if t, ok := asInt(m, "ttft_ms"); ok {
		call.TTFTMs = &t
	}
	if s, ok := asString(m, "stop_reason"); ok {
		call.StopReason = &s
	}
	if a, ok := asInt(m, "attempt"); ok {
		call.Attempt = &a
	}
	raw := rawSpanMetadata(sp.Name, m)
	call.Raw = &raw

	if cost, ok := pricing.Cost(model, override, call.InputUncached, call.CacheRead, call.CacheWrite, call.Output); ok {
		call.CostUSD = &cost
		call.CostSource = "list_price"
	} else {
		call.CostSource = "none"
	}
	return call, "", true
}

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// MetricState carries cumulative metric series values between requests so
// the fallback can compute per-window deltas. Persisted by the server.
type MetricState map[string]int64

// MetricCalls converts claude_code.token.usage points into aggregate rows.
// Delta-temporality points and gauges are used as-is; cumulative sums are
// differenced against state (including exporter-restart resets).
func MetricCalls(body []byte, state MetricState, override pricing.Override) ([]ledger.Call, []Skipped, error) {
	var req metricRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, nil, fmt.Errorf("decode metric request: %w", err)
	}
	if state == nil {
		state = MetricState{}
	}

	type groupKey struct {
		tNano int64
		model string
	}
	type group struct {
		u, r int64
		w, o int64
		seen bool
	}
	groups := map[groupKey]*group{}
	var order []groupKey
	var skipped []Skipped

	addValue := func(model, typ string, tNano int64, value int64) {
		key := groupKey{tNano: tNano, model: model}
		g := groups[key]
		if g == nil {
			g = &group{}
			groups[key] = g
			order = append(order, key)
		}
		g.seen = true
		switch typ {
		case "input":
			g.u = value
		case "output":
			g.o = value
		case "cacheRead", "cache_read":
			g.r = value
		case "cacheCreation", "cache_creation":
			g.w = value
		}
	}

	for _, rm := range req.ResourceMetrics {
		for _, sm := range rm.ScopeMetrics {
			for _, mt := range sm.Metrics {
				if mt.Name != metricTokenUsage {
					continue
				}
				type point struct {
					dp    dataPoint
					delta bool
				}
				var points []point
				if mt.Sum != nil {
					isDelta := mt.Sum.AggregationTemporality == temporalityDelta
					for _, dp := range mt.Sum.DataPoints {
						points = append(points, point{dp: dp, delta: isDelta})
					}
				}
				if mt.Gauge != nil {
					for _, dp := range mt.Gauge.DataPoints {
						points = append(points, point{dp: dp, delta: true})
					}
				}
				for _, p := range points {
					m := attrMap(p.dp.Attributes)
					typ, ok := asString(m, "type")
					if !ok {
						skipped = append(skipped, Skipped{Kind: "point", Reason: "missing type"})
						continue
					}
					model, _ := asString(m, "model")
					value, ok := p.dp.value()
					if !ok {
						skipped = append(skipped, Skipped{Kind: "point", Reason: "no value"})
						continue
					}
					tNano, err := strconv.ParseInt(p.dp.TimeUnixNano, 10, 64)
					if err != nil {
						// Parse time before touching the cumulative state:
						// a dropped point must not consume its delta.
						skipped = append(skipped, Skipped{Kind: "point", Reason: "missing time"})
						continue
					}
					if !p.delta {
						// Cumulative: difference against the stored series value.
						seriesKey := model + "\x00" + typ
						cum := value
						if last, seen := state[seriesKey]; seen {
							if value >= last {
								value = value - last
							}
							// else: exporter restart; take the current value as the new delta.
						}
						state[seriesKey] = cum
					}
					if value == 0 {
						continue
					}
					addValue(model, typ, tNano, value)
				}
			}
		}
	}

	sort.Slice(order, func(i, j int) bool {
		if order[i].tNano != order[j].tNano {
			return order[i].tNano < order[j].tNano
		}
		return order[i].model < order[j].model
	})
	var calls []ledger.Call
	for _, key := range order {
		g := groups[key]
		if !g.seen {
			continue
		}
		id := fmt.Sprintf("claude-code:agg:%d:%s", key.tNano, key.model)
		startedAt := key.tNano / 1_000_000
		raw := map[string]any{"metric": metricTokenUsage, "type": "aggregate", "model": key.model}
		rawJSON, _ := json.Marshal(raw)
		rawStr := string(rawJSON)
		call := ledger.Call{
			ID:            id,
			Harness:       "claude-code",
			SessionID:     "aggregate",
			RequestID:     id,
			Model:         key.model,
			BillingMode:   "subscription",
			InputUncached: g.u,
			CacheRead:     g.r,
			CacheWrite:    g.w,
			Output:        g.o,
			Source:        "claude_otel",
			StartedAt:     startedAt,
			AgentType:     strPtr("aggregate"),
			Raw:           &rawStr,
		}
		if cost, ok := pricing.Cost(key.model, override, g.u, g.r, g.w, g.o); ok {
			call.CostUSD = &cost
			call.CostSource = "list_price"
		} else {
			call.CostSource = "none"
		}
		calls = append(calls, call)
	}
	return calls, skipped, nil
}
