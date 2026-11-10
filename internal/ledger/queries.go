package ledger

import (
	"database/sql"
	"os"
	"strconv"
)

// RowsFilter selects call rows for the dashboard and export.
type RowsFilter struct {
	SinceMS int64  // 0 = no lower bound
	Harness string // "" = all
	Session string // "" = all
	Limit   int    // 0 = no limit
	Offset  int
	Asc     bool // export wants ascending; dashboard wants descending
}

// Summary is the aggregate over the selected rows.
type Summary struct {
	Calls       int64
	InputFresh  int64
	CacheRead   int64
	CacheWrite  int64
	Output      int64
	Reasoning   int64
	PromptTotal int64
	TotalTokens int64
	CostUSD     float64
	CostedCalls int64 // rows with a computable cost
}

// HitRate is the share of prompt tokens served from prompt cache: R/(U+R+W).
func (s Summary) HitRate() float64 {
	denom := s.InputFresh + s.CacheRead + s.CacheWrite
	if denom == 0 {
		return 0
	}
	return float64(s.CacheRead) / float64(denom)
}

// FreshShare is the share of prompt tokens that were not cached: U/(U+R+W).
func (s Summary) FreshShare() float64 {
	denom := s.InputFresh + s.CacheRead + s.CacheWrite
	if denom == 0 {
		return 0
	}
	return float64(s.InputFresh) / float64(denom)
}

// ContextTax is the prompt-to-output ratio: (U+R+W)/O. How much context the
// agent pays for each token of work.
func (s Summary) ContextTax() float64 {
	if s.Output == 0 {
		return 0
	}
	return float64(s.InputFresh+s.CacheRead+s.CacheWrite) / float64(s.Output)
}

// CostPerOutputToken is the API-equivalent cost per completion token.
func (s Summary) CostPerOutputToken() float64 {
	if s.Output == 0 {
		return 0
	}
	return s.CostUSD / float64(s.Output)
}

const callColumns = `id, harness, session_id, agent_id, turn_index, request_id, model, billing_mode,
	input_uncached, cache_read, cache_write, output, reasoning, prompt_total, total_tokens,
	cost_usd, cost_source, started_at, ended_at, duration_ms, ttft_ms, stop_reason,
	attempt, tool_name, source, harness_version, raw, agent_type`

func scanCall(row interface{ Scan(...any) error }) (Call, error) {
	var c Call
	err := row.Scan(&c.ID, &c.Harness, &c.SessionID, &c.AgentID, &c.TurnIndex, &c.RequestID,
		&c.Model, &c.BillingMode, &c.InputUncached, &c.CacheRead, &c.CacheWrite, &c.Output,
		&c.Reasoning, &c.PromptTotal, &c.TotalTokens, &c.CostUSD, &c.CostSource, &c.StartedAt,
		&c.EndedAt, &c.DurationMs, &c.TTFTMs, &c.StopReason, &c.Attempt, &c.ToolName,
		&c.Source, &c.HarnessVersion, &c.Raw, &c.AgentType)
	return c, err
}

// Rows returns call rows matching the filter.
func (d *DB) Rows(f RowsFilter) ([]Call, error) {
	q := "SELECT " + callColumns + " FROM calls WHERE (?=0 OR started_at>=?) AND (?='' OR harness=?) AND (?='' OR session_id=?)"
	args := []any{f.SinceMS, f.SinceMS, f.Harness, f.Harness, f.Session, f.Session}
	if f.Asc {
		q += " ORDER BY started_at ASC, id ASC"
	} else {
		q += " ORDER BY started_at DESC, id DESC"
	}
	if f.Limit > 0 {
		q += " LIMIT ? OFFSET ?"
		args = append(args, f.Limit, f.Offset)
	}
	rows, err := d.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Call
	for rows.Next() {
		c, err := scanCall(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// Summary aggregates rows for the filter.
func (d *DB) Summary(f RowsFilter) (Summary, error) {
	q := `SELECT COUNT(*), COALESCE(SUM(input_uncached),0), COALESCE(SUM(cache_read),0),
		COALESCE(SUM(cache_write),0), COALESCE(SUM(output),0), COALESCE(SUM(reasoning),0),
		COALESCE(SUM(prompt_total),0), COALESCE(SUM(total_tokens),0),
		COALESCE(SUM(cost_usd),0), COUNT(cost_usd)
		FROM calls WHERE (?=0 OR started_at>=?) AND (?='' OR harness=?) AND (?='' OR session_id=?)`
	var s Summary
	err := d.db.QueryRow(q, f.SinceMS, f.SinceMS, f.Harness, f.Harness, f.Session, f.Session).Scan(
		&s.Calls, &s.InputFresh, &s.CacheRead, &s.CacheWrite, &s.Output, &s.Reasoning,
		&s.PromptTotal, &s.TotalTokens, &s.CostUSD, &s.CostedCalls)
	return s, err
}

// SessionRow is one session in the session list.
type SessionRow struct {
	Harness     string `json:"harness"`
	SessionID   string `json:"session_id"`
	FirstSeen   int64  `json:"first_seen"`
	LastSeen    int64  `json:"last_seen"`
	Turns       int64  `json:"turns"`
	TotalTokens int64  `json:"total_tokens"`
}

// Sessions lists sessions with at least one row since the given time.
func (d *DB) Sessions(sinceMS int64, harness string) ([]SessionRow, error) {
	q := `SELECT harness, session_id, MIN(started_at), MAX(started_at), COUNT(*), COALESCE(SUM(total_tokens),0)
		FROM calls
		WHERE (?=0 OR started_at>=?) AND (?='' OR harness=?)
		GROUP BY harness, session_id ORDER BY MAX(started_at) DESC`
	rows, err := d.db.Query(q, sinceMS, sinceMS, harness, harness)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SessionRow
	for rows.Next() {
		var s SessionRow
		if err := rows.Scan(&s.Harness, &s.SessionID, &s.FirstSeen, &s.LastSeen, &s.Turns, &s.TotalTokens); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// LastSourceTime returns the newest started_at for a harness (0 if none).
func (d *DB) LastSourceTime(harness string) (int64, error) {
	var last sql.NullInt64
	err := d.db.QueryRow(`SELECT MAX(started_at) FROM calls WHERE harness=?`, harness).Scan(&last)
	if err != nil {
		return 0, err
	}
	return last.Int64, nil
}

// Counts returns total rows and per-harness counts for /health.
func (d *DB) Counts() (total int64, byHarness map[string]int64, err error) {
	byHarness = map[string]int64{}
	if err := d.db.QueryRow(`SELECT COUNT(*) FROM calls`).Scan(&total); err != nil {
		return 0, nil, err
	}
	rows, err := d.db.Query(`SELECT harness, COUNT(*) FROM calls GROUP BY harness`)
	if err != nil {
		return 0, nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var h string
		var n int64
		if err := rows.Scan(&h, &n); err != nil {
			return 0, nil, err
		}
		byHarness[h] = n
	}
	return total, byHarness, rows.Err()
}

// AggregateCount returns the number of claude-code rows that came from the
// metric fallback (agent_type='aggregate') rather than per-turn spans.
func (d *DB) AggregateCount(harness string) (int64, error) {
	var n int64
	err := d.db.QueryRow(`SELECT COUNT(*) FROM calls WHERE harness=? AND agent_type='aggregate'`, harness).Scan(&n)
	return n, err
}

// PerTurnCount returns rows for a harness excluding aggregate fallback rows.
func (d *DB) PerTurnCount(harness string) (int64, error) {
	var n int64
	err := d.db.QueryRow(`SELECT COUNT(*) FROM calls WHERE harness=? AND (agent_type IS NULL OR agent_type!='aggregate')`, harness).Scan(&n)
	return n, err
}

// GetCursor returns the stored (inode, offset) cursor and parser state for
// a source. state is "" when none was persisted.
func (d *DB) GetCursor(source string) (inode int64, offset int64, state string, ok bool, err error) {
	var i, o sql.NullInt64
	var st string
	err = d.db.QueryRow(`SELECT inode, offset, state FROM ingest_cursor WHERE source=?`, source).Scan(&i, &o, &st)
	if err == sql.ErrNoRows {
		return 0, 0, "", false, nil
	}
	if err != nil {
		return 0, 0, "", false, err
	}
	return i.Int64, o.Int64, st, true, nil
}

// SetCursor stores the (inode, offset) cursor and parser state for a source.
func (d *DB) SetCursor(source string, inode int64, offset int64, state string) error {
	_, err := d.db.Exec(`INSERT INTO ingest_cursor (source, inode, offset, state, updated_at)
		VALUES (?,?,?,?,strftime('%s','now') * 1000)
		ON CONFLICT(source) DO UPDATE SET inode=excluded.inode, offset=excluded.offset, state=excluded.state, updated_at=excluded.updated_at`,
		source, inode, offset, state)
	return err
}

// GetMeta returns a metadata value ("" when absent).
func (d *DB) GetMeta(key string) (string, error) {
	var v string
	err := d.db.QueryRow(`SELECT value FROM meta WHERE key=?`, key).Scan(&v)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return v, err
}

// SetMeta stores a metadata value.
func (d *DB) SetMeta(key, value string) error {
	_, err := d.db.Exec(`INSERT INTO meta (key, value) VALUES (?,?)
		ON CONFLICT(key) DO UPDATE SET value=excluded.value`, key, value)
	return err
}

// IntegrityCheck runs PRAGMA integrity_check and returns its result string.
func (d *DB) IntegrityCheck() (string, error) {
	var result string
	if err := d.db.QueryRow(`PRAGMA integrity_check`).Scan(&result); err != nil {
		return "", err
	}
	return result, nil
}

// DBSize returns the on-disk size of the ledger database in bytes.
func (d *DB) DBSize() (int64, error) {
	var path string
	if err := d.db.QueryRow(`PRAGMA database_list`).Scan(new(int), new(string), &path); err != nil {
		return 0, err
	}
	fi, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	return fi.Size(), nil
}

// FormatMs renders an epoch-ms timestamp as a string (helper for CLI output).
func FormatMs(ms int64) string {
	if ms == 0 {
		return "never"
	}
	return strconv.FormatInt(ms, 10)
}
