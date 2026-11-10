package ledger

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
)

// ExportColumns is the stable CSV/JSON export column order (data contract).
var ExportColumns = []string{
	"id", "harness", "session_id", "agent_type", "turn_index", "request_id", "model", "billing_mode",
	"input_uncached", "cache_read", "cache_write", "output", "reasoning", "prompt_total", "total_tokens",
	"cost_usd", "cost_source", "started_at", "ended_at", "duration_ms", "ttft_ms", "stop_reason",
	"attempt", "source", "harness_version",
}

// WriteCSV writes rows in export column order. The caller sets headers.
func WriteCSV(w io.Writer, rows []Call) error {
	cw := csv.NewWriter(w)
	if err := cw.Write(ExportColumns); err != nil {
		return err
	}
	for _, c := range rows {
		record := make([]string, len(ExportColumns))
		for i, col := range ExportColumns {
			record[i] = exportField(col, c)
		}
		if err := cw.Write(record); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}

// WriteJSON writes rows as pretty-printed JSON.
func WriteJSON(w io.Writer, rows []Call) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(rows)
}

func exportField(col string, c Call) string {
	optInt := func(v *int64) string {
		if v == nil {
			return ""
		}
		return strconv.FormatInt(*v, 10)
	}
	optStr := func(v *string) string {
		if v == nil {
			return ""
		}
		return *v
	}
	switch col {
	case "id":
		return c.ID
	case "harness":
		return c.Harness
	case "session_id":
		return c.SessionID
	case "agent_type":
		return optStr(c.AgentType)
	case "turn_index":
		return strconv.FormatInt(c.TurnIndex, 10)
	case "request_id":
		return c.RequestID
	case "model":
		return c.Model
	case "billing_mode":
		return c.BillingMode
	case "input_uncached":
		return strconv.FormatInt(c.InputUncached, 10)
	case "cache_read":
		return strconv.FormatInt(c.CacheRead, 10)
	case "cache_write":
		return strconv.FormatInt(c.CacheWrite, 10)
	case "output":
		return strconv.FormatInt(c.Output, 10)
	case "reasoning":
		return strconv.FormatInt(c.Reasoning, 10)
	case "prompt_total":
		return strconv.FormatInt(c.PromptTotal, 10)
	case "total_tokens":
		return strconv.FormatInt(c.TotalTokens, 10)
	case "cost_usd":
		if c.CostUSD == nil {
			return ""
		}
		return fmt.Sprintf("%.6f", *c.CostUSD)
	case "cost_source":
		return c.CostSource
	case "started_at":
		return strconv.FormatInt(c.StartedAt, 10)
	case "ended_at":
		return optInt(c.EndedAt)
	case "duration_ms":
		return optInt(c.DurationMs)
	case "ttft_ms":
		return optInt(c.TTFTMs)
	case "stop_reason":
		return optStr(c.StopReason)
	case "attempt":
		return optInt(c.Attempt)
	case "source":
		return c.Source
	case "harness_version":
		return optStr(c.HarnessVersion)
	default:
		return ""
	}
}
