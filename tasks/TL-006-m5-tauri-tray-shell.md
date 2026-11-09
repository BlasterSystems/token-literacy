# Task: TL-006 — M5 optional Tauri tray shell

Epic: token-literacy v0.1 ([docs/PLAN.md](../docs/PLAN.md) M5)

## Objective

Add an optional Tauri v2 menu-bar/tray shell that supervises the Go binary, opens the existing dashboard, and manages autostart — without duplicating the collector or UI.

## Product manager summary

- **One-liner:** A menu-bar/tray icon that makes the collector discoverable on macOS and Windows.
- **User value:** Start, inspect, and quit the ledger without remembering a CLI command.
- **Business impact:** Turns the POC into something a reader can leave running during a workday.
- **Success metric:** Tray icon appears on both OSes; dashboard opens; Quit stops the sidecar; Go binary still works standalone.
- **Dependencies & constraints:** TL-005; Rust/Tauri toolchain; per-OS CI builds; sidecar signing.

## Acceptance criteria

- [ ] Tauri v2 app with tray/menu-bar icon; template icon on macOS; `.ico` on Windows.
- [ ] `ActivationPolicy::Accessory` on macOS; no Dock icon.
- [ ] Go binary bundled as a sidecar; spawn, health poll, restart on crash, kill on quit.
- [ ] Window loads `http://127.0.0.1:4318`; no duplicated frontend.
- [ ] Menu: Open dashboard, Doctor, Copy last session summary, Quit.
- [ ] Autostart and single-instance plugins configured.
- [ ] Per-OS CI builds (DMG, MSI/NSIS); sidecar signed with the app; macOS notarization.
- [ ] Standalone Go binary remains the supported path.

## Scope

### Included

- Tauri shell, tray, sidecar lifecycle, autostart, per-OS packaging.

### Excluded

- New collector features, new dashboard views, Mac App Store distribution.

## Plan

- [ ] Create `src-tauri/` and tray icon assets.
- [ ] Implement sidecar spawn/supervise and health state.
- [ ] Wire the window and menu.
- [ ] Add autostart + single-instance.
- [ ] Add per-OS build workflow and signing steps.
- [ ] Smoke-test on macOS and Windows.

## Architecture impact

- **ADR required:** No new ADR; implements ADR-0001.
- **Relevant ADRs:** ADR-0001, ADR-0004.
- **Proposed ADR:** None unless the sidecar lifecycle forces a process-model change.

## Validation evidence

- Configuration validation:
- Tests:
- Build:
- Documentation check:

## Review

- Correctness: Quit stops the sidecar; crash restarts it; health state matches `/health`.
- Security: No additional network exposure; signed nested binary.
- Compatibility: macOS menu bar, Windows tray overflow; no Dock icon.
- Residual risks: Tauri E2E on macOS is manual; per-OS CI required.
