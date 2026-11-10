// Package ledger implements the SQLite-backed token ledger: schema,
// migrations, idempotent inserts, and the query surface used by the
// dashboard, export, and doctor commands.
package ledger

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite" // pure-Go SQLite driver (CGO_ENABLED=0)
)

// schemaVersion is the current PRAGMA user_version of the schema.
const schemaVersion = 3

// Call is one normalized per-turn token row (docs/PLAN.md data contract).
type Call struct {
	ID             string   `json:"id"` // harness:request_id
	Harness        string   `json:"harness"`
	SessionID      string   `json:"session_id"`
	AgentID        *string  `json:"agent_id"`
	TurnIndex      int64    `json:"turn_index"`
	RequestID      string   `json:"request_id"`
	Model          string   `json:"model"`
	BillingMode    string   `json:"billing_mode"` // subscription, api_key, credits
	InputUncached  int64    `json:"input_uncached"`
	CacheRead      int64    `json:"cache_read"`
	CacheWrite     int64    `json:"cache_write"`
	Output         int64    `json:"output"`
	Reasoning      int64    `json:"reasoning"` // subset of Output (Codex); 0 for Claude
	PromptTotal    int64    `json:"prompt_total"`
	TotalTokens    int64    `json:"total_tokens"`
	CostUSD        *float64 `json:"cost_usd"`
	CostSource     string   `json:"cost_source"` // list_price, default_price, provider_reported, none
	StartedAt      int64    `json:"started_at"`  // epoch ms
	EndedAt        *int64   `json:"ended_at"`
	DurationMs     *int64   `json:"duration_ms"`
	TTFTMs         *int64   `json:"ttft_ms"`
	StopReason     *string  `json:"stop_reason"`
	Attempt        *int64   `json:"attempt"`
	ToolName       *string  `json:"tool_name"`
	Source         string   `json:"source"` // claude_otel, codex_rollout, codex_otel
	HarnessVersion *string  `json:"harness_version"`
	Raw            *string  `json:"raw"` // trimmed source metadata; never prompts or content
	AgentType      *string  `json:"agent_type"`
}

// DB wraps the SQLite ledger database.
type DB struct {
	db *sql.DB
}

// Open opens (creating if needed) the ledger database at path and applies
// migrations. The parent directory is created if missing.
func Open(path string) (*DB, error) {
	if dir := filepath.Dir(path); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("create ledger directory: %w", err)
		}
	}
	dsn := "file:" + path
	sqlDB, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open ledger: %w", err)
	}
	// Single-developer volume: one connection serializes writes cleanly and
	// avoids SQLITE_BUSY entirely. Reads are fast enough at this scale.
	sqlDB.SetMaxOpenConns(1)
	d := &DB{db: sqlDB}
	if err := d.configure(); err != nil {
		sqlDB.Close()
		return nil, err
	}
	if err := d.migrate(); err != nil {
		sqlDB.Close()
		return nil, err
	}
	return d, nil
}

func (d *DB) configure() error {
	for _, pragma := range []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA busy_timeout=5000",
		"PRAGMA synchronous=NORMAL",
	} {
		if _, err := d.db.Exec(pragma); err != nil {
			return fmt.Errorf("configure sqlite (%s): %w", pragma, err)
		}
	}
	return nil
}

var schemaV1 = `
CREATE TABLE IF NOT EXISTS calls (
	id              TEXT PRIMARY KEY,
	harness         TEXT NOT NULL,
	session_id      TEXT NOT NULL DEFAULT '',
	agent_id        TEXT,
	turn_index      INTEGER NOT NULL DEFAULT 0,
	request_id      TEXT NOT NULL DEFAULT '',
	model           TEXT NOT NULL DEFAULT '',
	billing_mode    TEXT NOT NULL DEFAULT 'subscription',
	input_uncached  INTEGER NOT NULL,
	cache_read      INTEGER NOT NULL,
	cache_write     INTEGER NOT NULL,
	output          INTEGER NOT NULL,
	reasoning       INTEGER NOT NULL DEFAULT 0,
	prompt_total    INTEGER NOT NULL,
	total_tokens    INTEGER NOT NULL,
	cost_usd        REAL,
	cost_source     TEXT NOT NULL DEFAULT 'none',
	started_at      INTEGER NOT NULL,
	ended_at        INTEGER,
	duration_ms     INTEGER,
	ttft_ms         INTEGER,
	stop_reason     TEXT,
	attempt         INTEGER,
	tool_name       TEXT,
	source          TEXT NOT NULL,
	harness_version TEXT,
	raw             TEXT,
	agent_type      TEXT,
	ingested_at     INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_calls_started_at ON calls(started_at);
CREATE INDEX IF NOT EXISTS idx_calls_session ON calls(harness, session_id);
CREATE INDEX IF NOT EXISTS idx_calls_harness ON calls(harness);

CREATE TABLE IF NOT EXISTS sessions (
	id          TEXT PRIMARY KEY,
	harness     TEXT NOT NULL,
	session_id  TEXT NOT NULL,
	first_seen  INTEGER NOT NULL,
	last_seen   INTEGER NOT NULL,
	turn_count  INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_sessions_last_seen ON sessions(last_seen);

CREATE TABLE IF NOT EXISTS ingest_cursor (
	source     TEXT PRIMARY KEY,
	inode      INTEGER,
	offset     INTEGER NOT NULL DEFAULT 0,
	state      TEXT NOT NULL DEFAULT '',
	updated_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS model_rates (
	model       TEXT PRIMARY KEY,
	name        TEXT NOT NULL DEFAULT '',
	input       REAL NOT NULL,
	cache_read  REAL NOT NULL,
	cache_write REAL NOT NULL,
	output      REAL NOT NULL,
	source      TEXT NOT NULL DEFAULT 'openrouter',
	updated_at  INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS meta (
	key   TEXT PRIMARY KEY,
	value TEXT NOT NULL
);
`

func (d *DB) migrate() error {
	var version int
	if err := d.db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		return fmt.Errorf("read schema version: %w", err)
	}
	switch {
	case version == schemaVersion:
		return nil
	case version == 0:
		// Distinguish a fresh database from an empty legacy one: both have
		// user_version 0, but a fresh database has no tables at all.
		var name string
		err := d.db.QueryRow(
			"SELECT name FROM sqlite_master WHERE type='table' AND name='calls'").Scan(&name)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("inspect schema: %w", err)
		}
		if err == nil {
			return fmt.Errorf("ledger schema exists but user_version is 0; refusing to guess migration")
		}
		if _, err := d.db.Exec(schemaV1); err != nil {
			return fmt.Errorf("create schema: %w", err)
		}
		if _, err := d.db.Exec(fmt.Sprintf("PRAGMA user_version=%d", schemaVersion)); err != nil {
			return fmt.Errorf("stamp schema version: %w", err)
		}
		return nil
	case version == 1:
		// v2: ingest_cursor gains a `state` column carrying the rollout
		// parser state (session/model/cli_version/parent/records flag) so
		// resumed tail polls keep their attribution.
		if _, err := d.db.Exec(`ALTER TABLE ingest_cursor ADD COLUMN state TEXT NOT NULL DEFAULT ''`); err != nil {
			return fmt.Errorf("migrate v1->v2: %w", err)
		}
		if err := d.migrateV3(); err != nil {
			return err
		}
		return nil
	case version == 2:
		// v3: model_rates caches per-model list prices (OpenRouter catalog
		// snapshots and manual overrides) used for API-equivalent costing.
		if err := d.migrateV3(); err != nil {
			return err
		}
		return nil
	default:
		return fmt.Errorf("ledger schema version %d is newer than supported version %d", version, schemaVersion)
	}
}

// migrateV3 creates the model_rates table and stamps the schema version.
func (d *DB) migrateV3() error {
	if _, err := d.db.Exec(`CREATE TABLE IF NOT EXISTS model_rates (
		model       TEXT PRIMARY KEY,
		name        TEXT NOT NULL DEFAULT '',
		input       REAL NOT NULL,
		cache_read  REAL NOT NULL,
		cache_write REAL NOT NULL,
		output      REAL NOT NULL,
		source      TEXT NOT NULL DEFAULT 'openrouter',
		updated_at  INTEGER NOT NULL
	)`); err != nil {
		return fmt.Errorf("migrate to v3: %w", err)
	}
	if _, err := d.db.Exec(fmt.Sprintf("PRAGMA user_version=%d", schemaVersion)); err != nil {
		return fmt.Errorf("stamp schema version: %w", err)
	}
	return nil
}

// Close closes the database.
func (d *DB) Close() error { return d.db.Close() }

var ErrInvalidCall = errors.New("call rejected by invariant")

// InsertCall validates and idempotently inserts one row. It returns
// inserted=false when the row already exists (restart/replay dedupe).
// prompt_total and total_tokens are always derived from the additive fields,
// so an inconsistent caller cannot double-count.
func (d *DB) InsertCall(c Call) (bool, error) {
	if c.ID == "" {
		return false, fmt.Errorf("%w: empty id", ErrInvalidCall)
	}
	if c.Harness == "" {
		return false, fmt.Errorf("%w: empty harness", ErrInvalidCall)
	}
	if c.Source == "" {
		return false, fmt.Errorf("%w: empty source", ErrInvalidCall)
	}
	tokens := []int64{c.InputUncached, c.CacheRead, c.CacheWrite, c.Output, c.Reasoning}
	for _, n := range tokens {
		if n < 0 {
			return false, fmt.Errorf("%w: negative token count", ErrInvalidCall)
		}
	}
	if c.Reasoning > c.Output {
		return false, fmt.Errorf("%w: reasoning exceeds output", ErrInvalidCall)
	}
	c.PromptTotal = c.InputUncached + c.CacheRead + c.CacheWrite
	c.TotalTokens = c.PromptTotal + c.Output
	if c.TotalTokens == 0 {
		return false, fmt.Errorf("%w: zero total", ErrInvalidCall)
	}
	if c.BillingMode == "" {
		c.BillingMode = "subscription"
	}
	if c.CostSource == "" {
		c.CostSource = "none"
	}

	tx, err := d.db.Begin()
	if err != nil {
		return false, err
	}
	defer tx.Rollback()

	var exists int
	if err := tx.QueryRow(`SELECT EXISTS(SELECT 1 FROM calls WHERE id=?)`, c.ID).Scan(&exists); err != nil {
		return false, err
	}
	if exists == 1 {
		return false, nil
	}

	if c.SessionID == "" {
		c.SessionID = "unknown"
	}
	if err := tx.QueryRow(
		`SELECT COALESCE(MAX(turn_index),0)+1 FROM calls WHERE harness=? AND session_id=?`,
		c.Harness, c.SessionID).Scan(&c.TurnIndex); err != nil {
		return false, err
	}

	now := time.Now().UnixMilli()
	_, err = tx.Exec(`INSERT INTO calls (
		id, harness, session_id, agent_id, turn_index, request_id, model, billing_mode,
		input_uncached, cache_read, cache_write, output, reasoning, prompt_total, total_tokens,
		cost_usd, cost_source, started_at, ended_at, duration_ms, ttft_ms, stop_reason,
		attempt, tool_name, source, harness_version, raw, agent_type, ingested_at
	) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		c.ID, c.Harness, c.SessionID, c.AgentID, c.TurnIndex, c.RequestID, c.Model, c.BillingMode,
		c.InputUncached, c.CacheRead, c.CacheWrite, c.Output, c.Reasoning, c.PromptTotal, c.TotalTokens,
		c.CostUSD, c.CostSource, c.StartedAt, c.EndedAt, c.DurationMs, c.TTFTMs, c.StopReason,
		c.Attempt, c.ToolName, c.Source, c.HarnessVersion, c.Raw, c.AgentType, now)
	if err != nil {
		return false, err
	}

	sid := c.Harness + ":" + c.SessionID
	_, err = tx.Exec(`INSERT INTO sessions (id, harness, session_id, first_seen, last_seen, turn_count)
		VALUES (?,?,?,?,?,1)
		ON CONFLICT(id) DO UPDATE SET last_seen=excluded.last_seen, turn_count=turn_count+1`,
		sid, c.Harness, c.SessionID, c.StartedAt, c.StartedAt)
	if err != nil {
		return false, err
	}
	return true, tx.Commit()
}
