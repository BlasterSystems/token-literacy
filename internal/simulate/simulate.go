// Package simulate loads fixture files through the production ingest path:
// OTLP JSON (.json/.json.gz, traces or metrics) and Codex rollouts (.jsonl).
// It is used for demos, article evidence, and tests.
package simulate

import (
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/BlasterSystems/token-literacy/internal/ingest/codex"
	"github.com/BlasterSystems/token-literacy/internal/ingest/otlp"
	"github.com/BlasterSystems/token-literacy/internal/ledger"
	"github.com/BlasterSystems/token-literacy/internal/pricing"
)

// Run inserts every supported fixture under dir into db.
func Run(db *ledger.DB, dir string, override pricing.Override) (inserted int, err error) {
	err = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		n := 0
		switch {
		case strings.HasSuffix(path, ".jsonl"):
			data, readErr := os.ReadFile(path)
			if readErr != nil {
				return readErr
			}
			res := codex.ParseResult(data, override)
			for _, c := range res.Calls {
				if ok, err := db.InsertCall(c); err == nil && ok {
					n++
				}
			}
		case strings.HasSuffix(path, ".json"), strings.HasSuffix(path, ".json.gz"):
			data, readErr := readMaybeGzip(path)
			if readErr != nil {
				return readErr
			}
			var calls []ledger.Call
			switch {
			case bytes.Contains(data, []byte(`"resourceSpans"`)):
				calls, _, err = otlp.SpanCalls(data, override)
			case bytes.Contains(data, []byte(`"resourceMetrics"`)):
				calls, _, err = otlp.MetricCalls(data, otlp.MetricState{}, override)
			default:
				return nil // not an OTLP payload; skip silently
			}
			if err != nil {
				return err
			}
			for _, c := range calls {
				if ok, err := db.InsertCall(c); err == nil && ok {
					n++
				}
			}
		default:
			return nil
		}
		inserted += n
		return nil
	})
	return inserted, err
}

func readMaybeGzip(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	if strings.HasSuffix(path, ".gz") {
		zr, err := gzip.NewReader(f)
		if err != nil {
			return nil, err
		}
		defer zr.Close()
		return io.ReadAll(zr)
	}
	return io.ReadAll(f)
}
