// tokenledger tray shell: supervises the bundled Go collector, opens the
// existing dashboard, and manages autostart. The Go binary is the product;
// this shell only adds the menu-bar/tray entry point (ADR-0001).
#![cfg_attr(not(debug_assertions), windows_subsystem = "windows")]

use std::io::Read;
use std::net::TcpStream;
use std::path::PathBuf;
use std::sync::Mutex;
use std::time::Duration;

use tauri::{
    menu::{Menu, MenuItem},
    tray::TrayIconBuilder,
    Manager, WebviewUrl, WebviewWindowBuilder,
};

const DASHBOARD_URL: &str = "http://127.0.0.1:4318";
const SIDECAR: &str = "tokenledger";

#[derive(Default)]
struct Collector(Mutex<Option<std::process::Child>>);

fn main() {
    let app = tauri::Builder::default()
        .plugin(tauri_plugin_clipboard_manager::init())
        .plugin(tauri_plugin_autostart::init(
            tauri_plugin_autostart::MacosLauncher::LaunchAgent,
            None,
        ))
        .plugin(tauri_plugin_single_instance::init(|app, _args, _cwd| {
            // Second launch: surface the existing collector's dashboard.
            open_dashboard(app);
        }))
        .manage(Collector::default())
        .setup(|app| {
            // Menu-bar accessory on macOS: no Dock icon (M5 acceptance).
            #[cfg(target_os = "macos")]
            app.set_activation_policy(tauri::ActivationPolicy::Accessory);

            let open = MenuItem::with_id(app, "open", "Open dashboard", true, None::<&str>)?;
            let doctor = MenuItem::with_id(app, "doctor", "Doctor", true, None::<&str>)?;
            let copy =
                MenuItem::with_id(app, "copy", "Copy last session summary", true, None::<&str>)?;
            let quit = MenuItem::with_id(app, "quit", "Quit", true, None::<&str>)?;
            let menu = Menu::with_items(app, &[&open, &doctor, &copy, &quit])?;

            TrayIconBuilder::with_id("main")
                .icon(app.default_window_icon().expect("app icon").clone())
                .tooltip("tokenledger")
                .menu(&menu)
                .show_menu_on_left_click(true)
                .on_menu_event(|app, event| match event.id().as_ref() {
                    "open" => open_dashboard(app),
                    "doctor" => show_doctor(app),
                    "copy" => copy_summary(app),
                    "quit" => {
                        app.exit(0);
                    }
                    _ => {}
                })
                .build(app)?;

            // Spawn + supervise the bundled collector.
            spawn_collector(app.handle().clone());
            let handle = app.handle().clone();
            std::thread::spawn(move || supervise(handle));

            // Show the dashboard as soon as the collector answers. First
            // launch only: later launches open it via the single-instance
            // handler, and a broken install gets no window (tray + Doctor
            // remain usable). Window creation must run on the main thread.
            let handle = app.handle().clone();
            std::thread::spawn(move || {
                for _ in 0..40 {
                    if http_get_status(DASHBOARD_URL) == 200 {
                        let h = handle.clone();
                        let _ = handle.run_on_main_thread(move || open_dashboard(&h));
                        return;
                    }
                    std::thread::sleep(Duration::from_millis(250));
                }
            });

            Ok(())
        })
        .on_window_event(|window, event| {
            // Closing the dashboard window hides it; the collector keeps running.
            if let tauri::WindowEvent::CloseRequested { .. } = event {
                window.hide().ok();
            }
        })
        .build(tauri::generate_context!())
        .expect("error while building tokenledger shell");

    app.run(|app, event| {
        // Quit must stop the sidecar (M5 acceptance).
        if matches!(
            event,
            tauri::RunEvent::ExitRequested { .. } | tauri::RunEvent::Exit
        ) {
            if let Some(c) = app.state::<Collector>().0.lock().unwrap().as_mut() {
                let _ = c.kill();
                let _ = c.wait();
            }
        }
    });
}

fn sidecar_path() -> Option<PathBuf> {
    let exe = std::env::current_exe().ok()?;
    let mut dir = exe.parent()?.to_path_buf();
    let name = if cfg!(windows) {
        format!("{SIDECAR}.exe")
    } else {
        SIDECAR.to_string()
    };
    let p = dir.join(&name);
    if p.exists() {
        return Some(p);
    }
    // Fallback: resources dir next to the executable.
    dir = dir.join("resources");
    let p = dir.join(&name);
    if p.exists() {
        return Some(p);
    }
    None
}

fn spawn_collector(app: tauri::AppHandle) {
    let Some(path) = sidecar_path() else {
        eprintln!("tokenledger sidecar not found next to the shell binary");
        return;
    };
    let child = std::process::Command::new(path)
        .args(["up", "--no-open"])
        .spawn()
        .ok();
    *app.state::<Collector>().0.lock().unwrap() = child;
}

/// Health-poll the collector and restart the sidecar if it dies. A child
/// that exits immediately (e.g. single-instance refusal) backs off.
fn supervise(app: tauri::AppHandle) {
    let mut backoff_secs = 3u64;
    loop {
        std::thread::sleep(Duration::from_secs(backoff_secs));
        if http_get_status(DASHBOARD_URL) == 200 {
            backoff_secs = 3;
            continue;
        }
        let restart = {
            let state = app.state::<Collector>();
            let mut guard = state.0.lock().unwrap();
            let alive = match guard.as_mut() {
                Some(c) => matches!(c.try_wait(), Ok(None)),
                None => false,
            };
            if !alive {
                *guard = None;
                true
            } else {
                false
            }
        };
        if restart {
            let started = std::time::Instant::now();
            spawn_collector(app.clone());
            if started.elapsed() < Duration::from_secs(2) {
                backoff_secs = 30; // fast-fail: single-instance refusal or bad install
            } else {
                backoff_secs = 3;
            }
        }
    }
}

fn open_dashboard(app: &tauri::AppHandle) {
    if let Some(win) = app.get_webview_window("dashboard") {
        let _ = win.show();
        let _ = win.set_focus();
        return;
    }
    let url: tauri::Url = DASHBOARD_URL.parse().expect("dashboard url");
    let _ = WebviewWindowBuilder::new(app, "dashboard", WebviewUrl::External(url))
        .title("TokL - Token Literacy")
        .inner_size(1100.0, 760.0)
        .min_inner_size(390.0, 400.0)
        .build();
}

/// Minimal loopback HTTP GET; returns the status code (0 on failure).
fn http_get_status(_url: &str) -> u16 {
    let Ok(mut stream) = TcpStream::connect(("127.0.0.1", 4318)) else {
        return 0;
    };
    let _ = stream.set_read_timeout(Some(Duration::from_secs(3)));
    use std::io::Write;
    let req = "GET /health HTTP/1.1\r\nHost: 127.0.0.1:4318\r\nConnection: close\r\n\r\n";
    if stream.write_all(req.as_bytes()).is_err() {
        return 0;
    }
    let mut buf = [0u8; 512];
    let n = stream.read(&mut buf).unwrap_or(0);
    let head = String::from_utf8_lossy(&buf[..n]);
    head.split_whitespace()
        .nth(1)
        .and_then(|s| s.parse::<u16>().ok())
        .unwrap_or(0)
}

/// Fetch /api/summary and put a one-line digest on the clipboard.
fn copy_summary(app: &tauri::AppHandle) {
    let body = http_get_body(DASHBOARD_URL, "/api/summary");
    let summary = body
        .as_deref()
        .and_then(|b| serde_json::from_str::<serde_json::Value>(b).ok());
    let text = match summary {
        Some(v) => format!(
            "tokenledger: {} turns, {} tokens (U={} R={} W={} O={}), cache hit {:.1}%, API-equivalent ${:.4}",
            v["calls"], v["total_tokens"], v["input_fresh"], v["cache_read"], v["cache_write"],
            v["output"], v["cache_hit_rate"].as_f64().unwrap_or(0.0) * 100.0,
            v["cost_usd"].as_f64().unwrap_or(0.0)
        ),
        None => "tokenledger: collector not reachable".to_string(),
    };
    use tauri_plugin_clipboard_manager::ClipboardExt;
    let _ = app.clipboard().write_text(text);
}

fn http_get_body(_base: &str, path: &str) -> Option<String> {
    let mut stream = TcpStream::connect(("127.0.0.1", 4318)).ok()?;
    let _ = stream.set_read_timeout(Some(Duration::from_secs(3)));
    use std::io::Write;
    let req = format!("GET {path} HTTP/1.1\r\nHost: 127.0.0.1:4318\r\nConnection: close\r\n\r\n");
    stream.write_all(req.as_bytes()).ok()?;
    let mut buf = Vec::new();
    stream.read_to_end(&mut buf).ok()?;
    let s = String::from_utf8_lossy(&buf);
    let idx = s.find("\r\n\r\n")?;
    Some(s[idx + 4..].to_string())
}

/// Run `tokenledger doctor` (bundled sidecar) and show its report.
fn show_doctor(app: &tauri::AppHandle) {
    let output = sidecar_path()
        .and_then(|p| {
            std::process::Command::new(p)
                .arg("doctor")
                .output()
                .ok()
                .map(|o| String::from_utf8_lossy(&o.stdout).to_string())
        })
        .unwrap_or_else(|| "tokenledger binary not found".to_string());
    let html = format!(
        "<html><head><meta charset=\"utf-8\"><title>tokenledger doctor</title><style>body{{font-family:ui-monospace,Menlo,monospace;font-size:13px;padding:16px;background:#111827;color:#e5e7eb}}</style></head><body><pre>{}</pre></body></html>",
        html_escape(&output)
    );
    let url: tauri::Url = format!("data:text/html;charset=utf-8,{}", urlencode(&html))
        .parse()
        .expect("data url");
    if WebviewWindowBuilder::new(app, "doctor", WebviewUrl::External(url))
        .title("tokenledger doctor")
        .inner_size(640.0, 480.0)
        .build()
        .is_err()
    {
        // Fallback: put the report on the clipboard.
        use tauri_plugin_clipboard_manager::ClipboardExt;
        let _ = app.clipboard().write_text(output);
    }
}

fn html_escape(s: &str) -> String {
    s.replace('&', "&amp;").replace('<', "&lt;").replace('>', "&gt;")
}

fn urlencode(s: &str) -> String {
    let mut out = String::with_capacity(s.len());
    for b in s.bytes() {
        match b {
            b'A'..=b'Z' | b'a'..=b'z' | b'0'..=b'9' | b'-' | b'_' | b'.' | b'~' => {
                out.push(b as char)
            }
            _ => out.push_str(&format!("%{b:02X}")),
        }
    }
    out
}
