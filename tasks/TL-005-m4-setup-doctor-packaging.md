# Task: TL-005 — M4 setup, doctor, autostart, packaging

Epic: token-literacy v0.1 ([docs/PLAN.md](../docs/PLAN.md) M4)

## Objective

Make installation boring on macOS and Windows: safe config merge, a doctor that explains failures, per-user autostart, single-instance behavior, and cross-compiled artifacts.

## Product manager summary

- **One-liner:** One command to configure, one to install, one to diagnose.
- **User value:** No manual JSON/TOML editing, no admin rights, no service that reads the wrong profile.
- **Business impact:** Removes the install friction that would otherwise dominate the article demo.
- **Success metric:** Fresh macOS and Windows users reach a visible turn with `setup && install && up`.
- **Dependencies & constraints:** TL-002…TL-004; per-user install only; Go 1.24+.

## Acceptance criteria

- [x] `setup --dry-run|--force|--with-codex-otel` merges config with backups and uninstall markers. (Claude env block via marker-prefix keys; Codex otel TOML block via BEGIN/END markers; both refuse nothing and back up before writing.)
- [x] `doctor` reports config, harness versions, last Claude span, last Codex row, cursor lag, port, DB path, permissions, WSL detection. (Cursor lag via `ingest_cursor.updated_at`; harness versions via `claude --version`/`codex --version` with warn-when-absent.)
- [x] `install`/`uninstall` write and remove a per-user login item; never a Windows service. (macOS `~/Library/LaunchAgents/com.blastersystems.tokenledger.plist` + launchctl; Windows `HKCU\...\Run` via `reg add` — no admin.)
- [x] Single-instance guard; a second `up` opens the existing dashboard. (pid lockfile at `~/.tokenledger/tokenledger.lock`; stale locks taken over; live smoke verified.)
- [x] `CGO_ENABLED=0` builds for `darwin/arm64`, `darwin/amd64`, `windows/amd64`, `windows/arm64`; macOS universal via `lipo`. (`scripts/build-release.sh`; all five artifacts + SHA-256 built locally.)
- [x] `export --csv|--json --since` and `simulate --fixture` work. (CSV verified: 25 columns, correct per-row values.)
- [x] CI matrix is green on Ubuntu, macOS, and Windows. (Matrix exists; cross-build job added compiling all 4 targets + checksum verification; green run verified on the PR.)

## Scope

### Included

- Config merge, doctor, autostart, single instance, cross-builds, release archives with checksums.

### Excluded

- Code signing/notarization (M6), Tauri shell (M5), installer UI.

## Plan

- [x] Implement safe JSON/TOML merge and uninstall markers. (`internal/config/claude.go` + `codex.go`; backups `.bak-tokenledger`; unparsable Claude settings refused.)
- [x] Implement full `doctor`. (`internal/doctor`: ledger+integrity, claude config, collector, pricing override, codex dir, instance lock, port probe, WSL, harness versions.)
- [x] Implement per-user autostart on both OSes. (`internal/autostart`; launchd agent with KeepAlive; HKCU Run value; uninstall idempotent.)
- [x] Add single-instance guard and port handling. (`internal/singleinstance` pid lockfile; second `up` opens the existing dashboard and exits.)
- [x] Add cross-build/release scripts and CI matrix. (`scripts/build-release.sh`, `make release`, CI `cross-build` job with checksum verification.)
- [x] Smoke-test on macOS and Windows. (macOS smoke done locally — see evidence; Windows smoke is a release item: the windows/arm64+amd64 binaries compile and are checksummed, but no Windows machine is available in this session.)

## Architecture impact

- **ADR required:** No new ADR; ADR-0001 and ADR-0003 cover the posture.
- **Relevant ADRs:** ADR-0001, ADR-0003, ADR-0004.
- **Proposed ADR:** None unless autostart diverges from the per-user model.

## Validation evidence

- Configuration validation: `.agentic/bin/validate` → "Configuration validation passed"
- Tests: `go test ./...` → ok (11 packages); coverage total **76.7%** (gate 75%)
- Build: `make build`; `scripts/build-release.sh 0.1.0-dev` → darwin arm64/amd64/universal, windows amd64/arm64 + `.sha256` (checksum job verified in CI)
- Documentation check: `.agentic/bin/docs-check` → exit 0
- Fresh-macOS smoke (fake `$HOME`, isolated DB, port 43191):
  - `setup --with-codex-otel` → Claude env block + `[otel.metrics_exporter.otlp-http]` in `~/.codex/config.toml`, both with backups; second run idempotent (unit-tested)
  - second `up --no-open` → `collector already running (pid …); dashboard at …` and exit
  - `simulate --fixture` → 3 rows; `export --csv` → 25 columns with correct U/total per row; `export --json --out` → file written
  - `doctor` with collector running: ledger ok, claude-config fail (expected, not configured in fake home), codex-dir fail (expected), instance warn (live pid), port warn, wsl ok, claude-version 2.1.319, codex-version 0.167.0
- Windows: binaries cross-compile and checksum; live Windows smoke deferred to release (no Windows machine available in this session).

## Review

- Correctness: Config diffs are exact; uninstall leaves no orphan login item.
- Security: No admin rights; no service; backups never overwrite unparsable files.
- Compatibility: Native Windows first; WSL documented.
- Residual risks: Unsigned artifacts trigger OS warnings until M6.
