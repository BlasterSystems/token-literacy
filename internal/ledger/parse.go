package ledger

import (
	"strconv"
	"strings"
	"time"
)

// ParseSince accepts plain epoch-ms, a relative duration (7d, 12h; Go
// durations do not support days so Nd is handled here), or RFC3339.
// Unparsable input returns 0 (no lower bound).
func ParseSince(s string) int64 {
	if s == "" {
		return 0
	}
	if n, err := strconv.ParseInt(s, 10, 64); err == nil {
		return n
	}
	if strings.HasSuffix(s, "d") {
		if n, err := strconv.Atoi(strings.TrimSuffix(s, "d")); err == nil && n > 0 {
			return time.Now().Add(-time.Duration(n) * 24 * time.Hour).UnixMilli()
		}
	}
	if d, err := time.ParseDuration(s); err == nil {
		return time.Now().Add(-d).UnixMilli()
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.UnixMilli()
	}
	return 0
}

// CursorLag returns the newest ingest_cursor update (0 when none).
func (d *DB) CursorLag() (int64, error) {
	var max int64
	err := d.db.QueryRow(`SELECT COALESCE(MAX(updated_at),0) FROM ingest_cursor`).Scan(&max)
	return max, err
}
