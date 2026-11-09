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

- [ ] `setup --dry-run|--force|--with-codex-otel` merges config with backups and uninstall markers.
- [ ] `doctor` reports config, harness versions, last Claude span, last Codex row, cursor lag, port, DB path, permissions, WSL detection.
- [ ] `install`/`uninstall` write and remove a per-user login item; never a Windows service.
- [ ] Single-instance guard; a second `up` opens the existing dashboard.
- [ ] `CGO_ENABLED=0` builds for `darwin/arm64`, `darwin/amd64`, `windows/amd64`, `windows/arm64`; macOS universal via `lipo`.
- [ ] `export --csv|--json --since` and `simulate --fixture` work.
- [ ] CI matrix is green on Ubuntu, macOS, and Windows.

## Scope

### Included

- Config merge, doctor, autostart, single instance, cross-builds, release archives with checksums.

### Excluded

- Code signing/notarization (M6), Tauri shell (M5), installer UI.

## Plan

- [ ] Implement safe JSON/TOML merge and uninstall markers.
- [ ] Implement full `doctor`.
- [ ] Implement per-user autostart on both OSes.
- [ ] Add single-instance guard and port handling.
- [ ] Add cross-build/release scripts and CI matrix.
- [ ] Smoke-test on macOS and Windows.

## Architecture impact

- **ADR required:** No new ADR; ADR-0001 and ADR-0003 cover the posture.
- **Relevant ADRs:** ADR-0001, ADR-0003, ADR-0004.
- **Proposed ADR:** None unless autostart diverges from the per-user model.

## Validation evidence

- Configuration validation:
- Tests:
- Build:
- Documentation check:

## Review

- Correctness: Config diffs are exact; uninstall leaves no orphan login item.
- Security: No admin rights; no service; backups never overwrite unparsable files.
- Compatibility: Native Windows first; WSL documented.
- Residual risks: Unsigned artifacts trigger OS warnings until M6.
