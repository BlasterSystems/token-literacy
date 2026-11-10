package server

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/BlasterSystems/token-literacy/internal/ledger"
)

var exportColumns = []string{
	"id", "harness", "session_id", "agent_type", "turn_index", "request_id", "model", "billing_mode",
	"input_uncached", "cache_read", "cache_write", "output", "reasoning", "prompt_total", "total_tokens",
	"cost_usd", "cost_source", "started_at", "ended_at", "duration_ms", "ttft_ms", "stop_reason",
	"attempt", "source", "harness_version",
}

// exportRows is the shared row query for CSV and JSON export. Export is
// always ascending and unpaginated so files are reproducible.
func (s *Server) exportRows(r *http.Request) ([]ledger.Call, error) {
	f := queryFilter(r)
	f.Asc = true
	f.Limit = 0
	f.Offset = 0
	return s.db.Rows(f)
}

func (s *Server) handleExportCSV(w http.ResponseWriter, r *http.Request) {
	rows, err := s.exportRows(r)
	if err != nil {
		httpError(w, err)
		return
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="tokenledger-export.csv"`)
	cw := csv.NewWriter(w)
	_ = cw.Write(exportColumns)
	for _, c := range rows {
		record := make([]string, len(exportColumns))
		for i, col := range exportColumns {
			record[i] = csvField(col, c)
		}
		if err := cw.Write(record); err != nil {
			return
		}
	}
	cw.Flush()
}

func (s *Server) handleExportJSON(w http.ResponseWriter, r *http.Request) {
	rows, err := s.exportRows(r)
	if err != nil {
		httpError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", `attachment; filename="tokenledger-export.json"`)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(rows)
}

func csvField(col string, c ledger.Call) string {
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

// formatTime renders epoch ms for the dashboard table.
func formatTime(ms int64) string {
	if ms == 0 {
		return "—"
	}
	return time.UnixMilli(ms).Local().Format("01-02 15:04:05")
}
