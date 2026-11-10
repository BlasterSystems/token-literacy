package codex

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/BlasterSystems/token-literacy/internal/ledger"
	"github.com/BlasterSystems/token-literacy/internal/pricing"
)

// SessionsDir resolves the Codex rollout directory: $CODEX_HOME/sessions or
// ~/.codex/sessions.
func SessionsDir() (string, bool) {
	if env := os.Getenv("CODEX_HOME"); env != "" {
		return filepath.Join(env, "sessions"), true
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", false
	}
	return filepath.Join(home, ".codex", "sessions"), true
}

// Poller tails the Codex sessions directory into the ledger.
type Poller struct {
	DB       *ledger.DB
	Dir      string
	Interval time.Duration // default 1s
	Override pricing.Override
	Logf     func(format string, args ...any)
	Warnf    func(format string, args ...any)
}

// ScanOnce walks the sessions directory once, ingesting new complete lines
// from every rollout file. Per-file (path, inode, offset) cursors are kept
// in the ledger; a shrunk file restarts at 0 (dedupe makes that safe).
// A missing directory is not an error.
func (p *Poller) ScanOnce(ctx context.Context) (inserted int, warnings []string, err error) {
	if p.DB == nil {
		return 0, nil, errors.New("codex poller has no ledger")
	}
	if p.Dir == "" {
		return 0, nil, nil
	}
	walkErr := filepath.WalkDir(p.Dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // unreadable entries are skipped, not fatal
		}
		if d.IsDir() || !strings.HasSuffix(path, ".jsonl") {
			return nil
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		n, w, ferr := p.ingestFile(path)
		inserted += n
		warnings = append(warnings, w...)
		if ferr != nil {
			p.warnf("codex: %s: %v", path, ferr)
		}
		return nil
	})
	if walkErr != nil && !os.IsNotExist(walkErr) {
		return inserted, warnings, walkErr
	}
	return inserted, warnings, nil
}

// Run polls until ctx is done.
func (p *Poller) Run(ctx context.Context) {
	interval := p.Interval
	if interval == 0 {
		interval = time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			n, warnings, err := p.ScanOnce(ctx)
			if err != nil {
				p.warnf("codex scan failed: %v", err)
				continue
			}
			for _, w := range warnings {
				p.warnf("codex: %s", w)
			}
			if n > 0 {
				p.logf("codex: inserted %d rows", n)
			}
		}
	}
}

func (p *Poller) logf(format string, args ...any) {
	if p.Logf != nil {
		p.Logf(format, args...)
	}
}

func (p *Poller) warnf(format string, args ...any) {
	if p.Warnf != nil {
		p.Warnf(format, args...)
	}
}

func (p *Poller) ingestFile(path string) (int, []string, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return 0, nil, err
	}
	size := fi.Size()
	key := "codex:" + path
	_, offset, _, err := p.DB.GetCursor(key)
	if err != nil {
		return 0, nil, err
	}
	if offset > size {
		offset = 0 // rotated or truncated; response_id dedupe protects rows
	}
	if offset == size {
		return 0, nil, nil
	}

	f, err := os.Open(path)
	if err != nil {
		return 0, nil, err
	}
	defer f.Close()
	if offset > 0 {
		if _, err := f.Seek(offset, io.SeekStart); err != nil {
			return 0, nil, err
		}
	}

	pr := &parser{override: p.Override}
	consumed, feedErr := feed(bufio.NewReader(f), pr.handleLine)
	if feedErr != nil {
		return 0, nil, feedErr
	}
	res := pr.finalize()
	if len(res.Warnings) > 0 {
		// Surface the newest warning to doctor and the dashboard.
		_ = p.DB.SetMeta("codex_last_warning", strings.Join(res.Warnings, "; "))
	}

	inserted := 0
	for _, c := range res.Calls {
		ok, err := p.DB.InsertCall(c)
		if err != nil {
			return inserted, res.Warnings, err
		}
		if ok {
			inserted++
		}
	}
	// Advance the cursor past complete, successfully stored lines only.
	if err := p.DB.SetCursor(key, fileID(fi), offset+consumed); err != nil {
		return inserted, res.Warnings, err
	}
	return inserted, res.Warnings, nil
}

// feed reads complete newline-terminated lines. A trailing line without a
// newline is left for the next poll (the writer may still be appending).
func feed(r *bufio.Reader, handle func([]byte)) (int64, error) {
	var consumed int64
	for {
		chunk, err := r.ReadBytes('\n')
		if n := len(chunk); n > 0 {
			if chunk[n-1] != '\n' {
				return consumed, nil // incomplete trailing line
			}
			consumed += int64(n)
			handle(chunk)
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				return consumed, nil
			}
			return consumed, err
		}
	}
}

// ParseResult is exported for tests: parse a whole byte payload.
func ParseResult(data []byte, override pricing.Override) Result {
	pr := &parser{override: override}
	feed(bufio.NewReader(bytes.NewReader(data)), pr.handleLine)
	return pr.finalize()
}
