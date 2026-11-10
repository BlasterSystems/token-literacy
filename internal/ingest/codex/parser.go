// Package codex ingests OpenAI Codex CLI rollout JSONL files into the
// additive ledger. It reads usage metadata only; message and tool content
// are skipped without being interpreted.
package codex

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/BlasterSystems/token-literacy/internal/ledger"
	"github.com/BlasterSystems/token-literacy/internal/pricing"
)

// line is one rollout JSONL record.
type line struct {
	Timestamp string          `json:"timestamp"`
	Ordinal   int64           `json:"ordinal"`
	Type      string          `json:"type"`
	Payload   json.RawMessage `json:"payload"`
}

type sessionMetaPayload struct {
	SessionID      string  `json:"session_id"`
	ID             string  `json:"id"`
	CLIVersion     string  `json:"cli_version"`
	ParentThreadID *string `json:"parent_thread_id"`
	ThreadSource   string  `json:"thread_source"`
}

type turnContextPayload struct {
	Model string `json:"model"`
}

// tokenUsage mirrors the codex usage struct.
type tokenUsage struct {
	InputTokens           int64 `json:"input_tokens"`
	CachedInputTokens     int64 `json:"cached_input_tokens"`
	CacheWriteInputTokens int64 `json:"cache_write_input_tokens"`
	OutputTokens          int64 `json:"output_tokens"`
	ReasoningOutputTokens int64 `json:"reasoning_output_tokens"`
	TotalTokens           int64 `json:"total_tokens"`
}

type tokenUsageRecordPayload struct {
	ResponseID string      `json:"response_id"`
	SessionID  string      `json:"session_id"`
	ThreadID   string      `json:"thread_id"`
	TurnID     string      `json:"turn_id"`
	Usage      *tokenUsage `json:"usage"`
}

type tokenCountInfo struct {
	LastTokenUsage *tokenUsage `json:"last_token_usage"`
}

type tokenCountPayload struct {
	Type string          `json:"type"`
	Info *tokenCountInfo `json:"info"`
}

// Result is the parsed output for one file.
type Result struct {
	Calls    []ledger.Call
	Warnings []string
}

// parser carries per-file state.
type parser struct {
	override   pricing.Override
	sessionID  string
	cliVersion string
	parent     *string
	model      string

	records bool
	events  []ledger.Call
	res     Result
}

// finalize returns the parsed output, applying the file-level rule:
// event_msg token_count rows are used only when the file has no
// token_usage_record entries.
func (p *parser) finalize() Result {
	if !p.records {
		p.res.Calls = append(p.res.Calls, p.events...)
	}
	return p.res
}

func (p *parser) warnf(format string, args ...any) {
	p.res.Warnings = append(p.res.Warnings, fmt.Sprintf(format, args...))
}

func (p *parser) handleLine(chunk []byte) {
	ln := strings.TrimSpace(string(chunk))
	if ln == "" {
		return
	}
	var l line
	if err := json.Unmarshal([]byte(ln), &l); err != nil {
		p.warnf("skipping unparsable line: %v", err)
		return
	}
	switch l.Type {
	case "session_meta":
		var meta sessionMetaPayload
		if json.Unmarshal(l.Payload, &meta) == nil {
			if meta.SessionID != "" {
				p.sessionID = meta.SessionID
			} else if meta.ID != "" {
				p.sessionID = meta.ID
			}
			p.cliVersion = meta.CLIVersion
			p.parent = meta.ParentThreadID
		}
	case "turn_context":
		var tc turnContextPayload
		if json.Unmarshal(l.Payload, &tc) == nil && tc.Model != "" {
			p.model = tc.Model
		}
	case "token_usage_record":
		p.records = true
		var rec tokenUsageRecordPayload
		if err := json.Unmarshal(l.Payload, &rec); err != nil {
			p.warnf("token_usage_record unparsable: %v", err)
			return
		}
		if rec.Usage == nil {
			p.warnf("token_usage_record without usage (response_id=%s)", rec.ResponseID)
			return
		}
		call, ok := p.toCall(rec.Usage, "codex:"+rec.ResponseID, rec.ResponseID, l, rec.ThreadID)
		if !ok {
			return
		}
		p.res.Calls = append(p.res.Calls, call)
	case "event_msg":
		var ev tokenCountPayload
		if err := json.Unmarshal(l.Payload, &ev); err != nil || ev.Type != "token_count" {
			return
		}
		if ev.Info == nil || ev.Info.LastTokenUsage == nil {
			return
		}
		id := fmt.Sprintf("codex:e:%s:%d", p.sessionID, l.Ordinal)
		call, ok := p.toCall(ev.Info.LastTokenUsage, id, "", l, "")
		if !ok {
			return
		}
		p.events = append(p.events, call)
	}
}

// toCall normalizes one usage record: U = input - cached, R = cached,
// W = cache_write (0 for OpenAI list prices), O = output. The additive
// invariant U + R + W == input_tokens is enforced here; failing records are
// reported as warnings, never guessed.
func (p *parser) toCall(u *tokenUsage, id, requestID string, l line, agentID string) (ledger.Call, bool) {
	if u.InputTokens < 0 || u.CachedInputTokens < 0 || u.OutputTokens < 0 {
		p.warnf("negative token counts in %s", id)
		return ledger.Call{}, false
	}
	// Codex semantics: input_tokens includes cached_input_tokens; the cache
	// write figure (0 on OpenAI list prices) is additive on top of input.
	if u.CachedInputTokens > u.InputTokens {
		p.warnf("additive invariant failed for %s: cached=%d exceeds input=%d",
			id, u.CachedInputTokens, u.InputTokens)
		return ledger.Call{}, false
	}
	if u.TotalTokens != 0 && u.TotalTokens != u.InputTokens+u.OutputTokens {
		p.warnf("total mismatch for %s: total=%d input+output=%d",
			id, u.TotalTokens, u.InputTokens+u.OutputTokens)
		return ledger.Call{}, false
	}

	sessionID := p.sessionID
	if sessionID == "" {
		sessionID = "unknown"
	}
	startedAt := parseTimestamp(l.Timestamp)
	raw := rawMetadata(id, requestID, l, u)

	call := ledger.Call{
		ID:             id,
		Harness:        "codex",
		SessionID:      sessionID,
		RequestID:      requestID,
		Model:          p.model,
		BillingMode:    "subscription",
		InputUncached:  u.InputTokens - u.CachedInputTokens,
		CacheRead:      u.CachedInputTokens,
		CacheWrite:     u.CacheWriteInputTokens,
		Output:         u.OutputTokens,
		Reasoning:      u.ReasoningOutputTokens,
		PromptTotal:    u.InputTokens + u.CacheWriteInputTokens,
		TotalTokens:    u.InputTokens + u.CacheWriteInputTokens + u.OutputTokens,
		Source:         "codex_rollout",
		StartedAt:      startedAt,
		HarnessVersion: strPtr(p.cliVersion),
		Raw:            &raw,
	}
	if p.parent != nil && *p.parent != "" {
		sub := "subagent"
		call.AgentType = &sub
		call.AgentID = p.parent
	} else if agentID != "" && agentID != p.sessionID {
		call.AgentID = &agentID
	}
	if cost, ok := pricing.Cost(p.model, p.override, call.InputUncached, call.CacheRead, call.CacheWrite, call.Output); ok {
		call.CostUSD = &cost
		call.CostSource = "list_price"
	} else {
		call.CostSource = "none"
	}
	return call, true
}

// rawMetadata trims the record to usage metadata; never prompts or content.
func rawMetadata(id, requestID string, l line, u *tokenUsage) string {
	raw := map[string]any{
		"type":        "token_usage_record",
		"ordinal":     l.Ordinal,
		"input":       u.InputTokens,
		"cached":      u.CachedInputTokens,
		"cache_write": u.CacheWriteInputTokens,
		"output":      u.OutputTokens,
		"reasoning":   u.ReasoningOutputTokens,
		"total":       u.TotalTokens,
	}
	if requestID != "" {
		raw["response_id"] = requestID
	}
	b, err := json.Marshal(raw)
	if err != nil {
		return "{}"
	}
	return string(b)
}

func parseTimestamp(s string) int64 {
	if s == "" {
		return 0
	}
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return t.UnixMilli()
	}
	return 0
}

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
