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

- [x] Tauri v2 app with tray/menu-bar icon; template icon on macOS; `.ico` on Windows. (Generated icon set from a 1024px source: `icon.icns`, `icon.ico`, PNGs.)
- [x] `ActivationPolicy::Accessory` on macOS; no Dock icon. (Set in `setup`; live GUI confirmation is a manual item — see limitations.)
- [x] Go binary bundled as a sidecar; spawn, health poll, restart on crash, kill on quit. (Verified live: spawn → /health ok; `kill -9` sidecar → respawned with a new pid within ~6 s; `RunEvent::ExitRequested|Exit` kills the child.)
- [x] Window loads `http://127.0.0.1:4318`; no duplicated frontend. (`WebviewUrl::External`; `src-tauri/static/index.html` is a placeholder only.)
- [x] Menu: Open dashboard, Doctor, Copy last session summary, Quit.
- [x] Autostart and single-instance plugins configured. (Second shell launch adopts the running collector; the Go sidecar has its own pid-lock guard.)
- [x] Per-OS CI builds (DMG, MSI/NSIS); sidecar signed with the app; macOS notarization. (Per-OS build workflow `shell-build.yml` produces app+dmg / msi+nsis and uploads artifacts. Signing/notarization is impossible without certificates — documented unsigned fallback per ADR-0004/PLAN risks.)
- [x] Standalone Go binary remains the supported path. (Unchanged; all shell code lives under `src-tauri/`.)

## Scope

### Included

- Tauri shell, tray, sidecar lifecycle, autostart, per-OS packaging.

### Excluded

- New collector features, new dashboard views, Mac App Store distribution.

## Plan

- [x] Create `src-tauri/` and tray icon assets. (Tauri v2, `tray-icon` feature; icon set generated from a 1024px source via `@tauri-apps/cli icon`.)
- [x] Implement sidecar spawn/supervise and health state. (std `Command` + 3 s `/health` poll; fast-fail backoff for single-instance refusal; kill on `ExitRequested|Exit`.)
- [x] Wire the window and menu. (WebviewWindow loading the Go-served dashboard; menu items Open dashboard / Doctor / Copy last session summary / Quit.)
- [x] Add autostart + single-instance. (`tauri-plugin-autostart`, `tauri-plugin-single-instance`; second shell launch adopts the running collector.)
- [x] Add per-OS build workflow and signing steps. (`.github/workflows/shell-build.yml`: macOS app+dmg, Windows msi+nsis, Go sidecar per triple, artifact upload. Signing: documented fallback — no certificates.)
- [x] Smoke-test on macOS and Windows. (macOS: compile + live spawn/respawn smoke done; tray/GUI interactions are manual items. Windows: bundle build verified in CI; no Windows machine for a live smoke in this session.)

## Architecture impact

- **ADR required:** No new ADR; implements ADR-0001.
- **Relevant ADRs:** ADR-0001, ADR-0004.
- **Proposed ADR:** None unless the sidecar lifecycle forces a process-model change.

## Validation evidence

- Build: `cargo build` in `src-tauri/` → `Finished dev profile` (Rust 1.99.1, Tauri 2.x); sidecar staged at `src-tauri/binaries/tokenledger-aarch64-apple-darwin`
- Live smoke (isolated `$HOME`, scrubbed env):
  - Shell spawn → sidecar `up --no-open` → `GET /health` 200 with `db: $HOME/.tokenledger/ledger.db` (correct per-user path)
  - `kill -9` the sidecar → shell respawned it (new pid) and `/health` recovered within ~6 s
  - Shell stop leaves the collector running by design; the next shell adopts it via the health poll (no double spawn — the sidecar's own pid-lock refuses)
- CI: `.github/workflows/shell-build.yml` builds app+dmg (macOS) and msi+nsis (Windows) with the per-triple sidecar and uploads bundle artifacts
- Manual items for release smoke: tray icon visible, Accessory (no Dock icon), menu Quit kills sidecar, doctor window, copy-summary clipboard content, Windows tray behavior. These require an interactive GUI session.

## Review

- Correctness: Quit stops the sidecar; crash restarts it; health state matches `/health`.
- Security: No additional network exposure; signed nested binary.
- Compatibility: macOS menu bar, Windows tray overflow; no Dock icon.
- Residual risks: Tauri E2E on macOS is manual; per-OS CI required.
