package server

import (
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"sort"

	"github.com/BlasterSystems/token-literacy/internal/ledger"
	"github.com/BlasterSystems/token-literacy/internal/pricing"
)

//go:embed assets/fonts/*.woff2
var fontsFS embed.FS

// dashboardData is everything the template needs.
type dashboardData struct {
	Version    string
	DBPath     string
	Filter     filterView
	Summary    ledger.Summary
	Rows       []rowView
	Sessions   []ledger.SessionRow
	Banners    []string
	Empty      string
	ChartJSON  template.JS
	CostEst    float64
	CostTotal  float64
	CostPerOut float64
	Unpriced   int64
	HasEst     bool
}

type filterView struct {
	Harness string
	Session string
	Since   string
}

type rowView struct {
	Time    string
	Harness string
	Model   string
	U, R, W,
	O int64
	HitPct string
	Cost   string
	Source string
}

// chartRow is one per-turn point for the client-side charts (ascending).
type chartRow struct {
	T   int64   `json:"t"`
	U   int64   `json:"u"`
	R   int64   `json:"r"`
	W   int64   `json:"w"`
	O   int64   `json:"o"`
	Hit float64 `json:"hit"`
}

// chartRows renders rows oldest-first for the JS charts.
func chartRows(rows []ledger.Call) []chartRow {
	asc := ascending(rows)
	out := make([]chartRow, 0, len(asc))
	for _, c := range asc {
		out = append(out, chartRow{T: c.StartedAt, U: c.InputUncached, R: c.CacheRead, W: c.CacheWrite, O: c.Output, Hit: rowHitRate(c) * 100})
	}
	return out
}

// dashboardTmpl renders the dashboard; the only assets are the embedded DS
// fonts (no external requests).
var dashboardTmpl = template.Must(template.New("dashboard").Funcs(template.FuncMap{
	"pct":   func(f float64) string { return fmt.Sprintf("%.1f%%", f*100) },
	"ratio": func(f float64) string { return fmt.Sprintf("%.1f", f) },
	"cost":  func(f float64) string { return fmt.Sprintf("$%.4f", f) },
	"cost6": func(f float64) string { return fmt.Sprintf("$%.6f", f) },
	"sel": func(v, cur string) string {
		if v == cur {
			return " selected"
		}
		return ""
	},
}).Parse(dashboardHTML))

const dashboardHTML = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>TokL - Token Literacy</title>
<style>
/* ===== BlasterSystems DS v1.1 tokens (verbatim, identity v0.2) ===== */
:root{
  --paper:oklch(0.965 0.004 250); --paper-2:oklch(0.935 0.005 250); --card:oklch(0.99 0.002 250);
  --ink:oklch(0.22 0.015 255); --ink-2:oklch(0.40 0.014 255); --ink-3:oklch(0.51 0.012 255);
  --rule:oklch(0.87 0.006 255); --rule-2:oklch(0.78 0.008 255);
  --thread:oklch(0.60 0.19 32); --thread-ink:oklch(0.52 0.18 31); --thread-wash:oklch(0.60 0.19 32 / 0.12);
  --grid:oklch(0.22 0.015 255 / 0.05);
  --surface-page:var(--paper); --surface-sunken:var(--paper-2); --surface-card:var(--card); --surface-tooltip:var(--paper-2);
  --text-1:var(--ink); --text-2:var(--ink-2); --text-3:var(--ink-3);
  --border-1:var(--rule); --border-2:var(--rule-2); --border-strong:var(--ink);
  --action-bg:var(--ink); --action-bg-hover:var(--ink-2); --action-fg:var(--paper);
  --accent:var(--thread); --accent-text:var(--thread-ink); --accent-wash:var(--thread-wash);
  color-scheme:light;
}
[data-theme="dark"]{
  --paper:oklch(0.18 0.012 255); --paper-2:oklch(0.21 0.013 255); --card:oklch(0.225 0.014 255);
  --ink:oklch(0.95 0.004 250); --ink-2:oklch(0.80 0.008 250); --ink-3:oklch(0.66 0.010 250);
  --rule:oklch(0.31 0.012 255); --rule-2:oklch(0.40 0.014 255);
  --thread:oklch(0.68 0.17 35); --thread-ink:oklch(0.74 0.15 38); --thread-wash:oklch(0.68 0.17 35 / 0.16);
  --grid:oklch(0.95 0.004 250 / 0.05);
  color-scheme:dark;
}
:root{
  --font-sans:'Fira Sans',system-ui,-apple-system,'Segoe UI',sans-serif;
  --font-mono:'DM Mono',ui-monospace,'SF Mono',Menlo,monospace;
  --fs-label:11.5px; --fs-small:13px; --fs-ui:14px; --fs-body:16px;
  --fs-wordmark:17px; --fs-figure:34px;
  --radius-control:2px; --radius-panel:4px; --panel-pad:16px 18px 18px;
  --dur-ui:200ms;
}
@font-face{font-family:'Fira Sans';font-weight:300;font-style:normal;src:url(/assets/fonts/fira-sans-300-latin.woff2) format('woff2')}
@font-face{font-family:'Fira Sans';font-weight:400;font-style:normal;src:url(/assets/fonts/fira-sans-400-latin.woff2) format('woff2')}
@font-face{font-family:'Fira Sans';font-weight:500;font-style:normal;src:url(/assets/fonts/fira-sans-500-latin.woff2) format('woff2')}
@font-face{font-family:'DM Mono';font-weight:400;font-style:normal;src:url(/assets/fonts/dm-mono-400-latin.woff2) format('woff2')}
@font-face{font-family:'DM Mono';font-weight:500;font-style:normal;src:url(/assets/fonts/dm-mono-500-latin.woff2) format('woff2')}

html,body{margin:0;background:var(--paper)}
body{color:var(--ink);font-family:var(--font-sans);font-size:16px;line-height:1.5;-webkit-font-smoothing:antialiased}
*,*::before,*::after{box-sizing:border-box}
button{font:inherit;color:inherit}
::selection{background:var(--thread-wash)}
:focus-visible{outline:2px solid var(--thread);outline-offset:3px}
.bs-label{font-family:var(--font-mono);font-size:11.5px;letter-spacing:.08em;text-transform:uppercase;color:var(--ink-3)}
.bs-num{font-variant-numeric:tabular-nums}
.bs-figure{font-size:var(--fs-figure);font-weight:400;letter-spacing:-0.02em;line-height:1.05;font-variant-numeric:tabular-nums}
.bs-sub{font-size:14px;line-height:1.45;color:var(--ink-2);text-wrap:pretty}
.bs-note{margin:0;font-size:13px;line-height:1.45;color:var(--ink-3)}

.bs-panel{position:relative;display:flex;flex-direction:column;gap:12px;min-width:0;padding:var(--panel-pad);background:var(--card);border:1px solid var(--rule);border-radius:var(--radius-panel)}
.bs-panel__head{display:flex;flex-wrap:wrap;row-gap:8px;align-items:center;justify-content:space-between;gap:8px;min-height:24px}
.bs-panel__aside{font-family:var(--font-mono);font-size:12.5px;color:var(--ink-2)}
.bs-glyph{flex:none;display:inline-block}
.bs-glyph--recorded{width:8px;height:8px;background:currentColor}
.bs-mark{display:inline-flex;align-items:center;gap:8px;font-family:var(--font-mono);font-size:11px;letter-spacing:.06em;text-transform:uppercase;color:var(--ink-3)}
.bs-mark .bs-glyph--recorded{width:7px;height:7px}
.bs-chip{align-self:flex-start;display:inline-flex;align-items:center;gap:8px;padding:5px 9px;border:1px solid var(--rule-2);border-radius:2px;font-family:var(--font-mono);font-size:11px;letter-spacing:.06em;text-transform:uppercase;line-height:1.3;color:var(--ink)}
.bs-chip--stage{color:var(--ink-2)}
.bs-info{position:relative;flex:none;width:28px;height:28px;display:grid;place-items:center;margin:-6px -8px -6px 0;padding:0;border:0;border-radius:2px;background:transparent;color:var(--ink-3);cursor:help}
.bs-info:hover{color:var(--ink)}
.bs-tip{position:absolute;z-index:10;top:34px;right:-8px;width:280px;display:none;flex-direction:column;gap:10px;padding:14px 16px;background:var(--paper-2);border:1px solid var(--rule-2);border-radius:4px}
.bs-info:hover .bs-tip,.bs-info:focus-within .bs-tip{display:flex}
.bs-tip__text{font-size:13.5px;line-height:1.45;color:var(--ink-2);text-wrap:pretty}
.bs-calc{display:grid;grid-template-columns:minmax(0,1fr) auto;gap:6px 0;font-family:var(--font-mono);font-size:12px;line-height:1.4;font-variant-numeric:tabular-nums}
.bs-calc>span:nth-child(odd){color:var(--ink-3)}
.bs-calc>span:nth-child(even){padding-left:12px;text-align:right}
.bs-calc>span.bs-calc__total{padding-top:7px;border-top:1px solid var(--rule-2);color:var(--ink)}
.bs-alert{display:flex;gap:10px;align-items:baseline;padding:8px 0;border-top:1px solid var(--rule);font-size:13.5px;line-height:1.45;color:var(--ink-2)}
.bs-alert .bs-glyph--recorded{margin-top:6px;color:var(--thread-ink)}

.bs-field{display:flex;flex-direction:column;gap:6px;min-width:0}
.bs-field__label{font-family:var(--font-mono);font-size:11px;letter-spacing:.06em;text-transform:uppercase;color:var(--ink-3)}
.bs-field__box{display:flex;align-items:baseline;gap:8px;padding:8px 0;border-bottom:1px solid var(--rule-2);transition:border-color 150ms}
.bs-field__box:hover{border-bottom-color:var(--ink-2)}
.bs-field__box:focus-within{border-bottom:1.5px solid var(--thread);padding-bottom:7.5px}
.bs-field__input{flex:1;min-width:0;padding:0;border:0;background:transparent;color:var(--ink);font:400 15px/1.3 var(--font-sans);outline:none;appearance:none}
.bs-field__input::placeholder{color:var(--ink-3)}
.bs-field__input option{background:var(--card);color:var(--ink)}
.bs-field--mono .bs-field__input{font-family:var(--font-mono);font-variant-numeric:tabular-nums}
.bs-field__hint{display:flex;align-items:center;gap:8px;font-size:13px;line-height:1.4;color:var(--ink-2)}

.bs-btn{display:inline-flex;align-items:center;justify-content:flex-start;gap:10px;min-height:44px;padding:0 18px;border:0;border-radius:2px;font:500 15px/1.2 var(--font-sans);text-decoration:none;white-space:nowrap;cursor:pointer;transition:background-color 150ms,box-shadow 150ms,color 150ms}
.bs-btn--primary{background:var(--ink);color:var(--paper)}
.bs-btn--primary:hover{background:var(--ink-2);color:var(--paper)}
.bs-btn--secondary{background:transparent;color:var(--ink);box-shadow:inset 0 0 0 1px var(--rule-2)}
.bs-btn--secondary:hover{box-shadow:inset 0 0 0 1px var(--ink);color:var(--ink)}
.bs-btn--sm{min-height:36px;padding:0 14px;font-size:14px}
.bs-link{display:inline-flex;align-items:center;gap:6px;padding:0;border:0;background:none;font:500 14px/1.3 var(--font-sans);color:var(--ink-2);text-decoration:none;cursor:pointer}
.bs-link:hover{color:var(--thread-ink)}
.bs-theme{flex:none;width:36px;height:36px;display:grid;place-items:center;padding:0;border:1px solid var(--rule-2);border-radius:2px;background:transparent;color:var(--ink);cursor:pointer}
.bs-theme:hover{border-color:var(--ink)}
.bs-theme>span{width:13px;height:13px;border:1.5px solid currentColor;border-radius:50%;background:linear-gradient(90deg,currentColor 50%,transparent 50%)}

.bs-chart{display:flex;flex-direction:column;gap:8px;min-width:0}
.grid2>.bs-panel{min-width:0}
#rate{overflow:hidden}
#bars{overflow-x:auto}
.bs-chart__y{position:absolute;left:0;width:38px;transform:translateY(-50%);text-align:right;font-family:var(--font-mono);font-size:11px;color:var(--ink-3)}
.bs-chart__x{position:relative;height:14px;font-family:var(--font-mono);font-size:11px;letter-spacing:.06em;text-transform:uppercase;color:var(--ink-3)}
.bs-chart__x>span{position:absolute;top:0}
.bs-legend{display:flex;flex-wrap:wrap;gap:6px 16px;font-family:var(--font-mono);font-size:11px;letter-spacing:.06em;text-transform:uppercase;color:var(--ink-3)}
.bs-legend>span{display:inline-flex;align-items:center;gap:7px}
.sw{display:inline-block;width:9px;height:9px}
.sw--u{background:var(--ink-2)} .sw--r{background:var(--ink)} .sw--w{background:var(--rule-2)}
.sw--o{background:transparent;border:1.5px solid var(--thread)}

.bs-lockup{display:inline-flex;align-items:flex-start;gap:12px;min-width:0;color:var(--ink);vertical-align:middle}
.bs-lockup__text{display:flex;flex-direction:column;gap:8px;min-width:0}
.bs-lockup__name{font-weight:600;letter-spacing:-0.015em;line-height:1;font-size:var(--fs-wordmark)}
.bs-lockup__endorse{white-space:nowrap;font-family:var(--font-mono);font-size:10px;letter-spacing:.06em;text-transform:uppercase;line-height:1;color:color-mix(in oklch,currentColor 62%,transparent)}
.bs-lockup__mark{position:relative;display:inline-block;flex:none;width:30px;height:10px}
.bs-lockup__mark::before{content:"";position:absolute;left:0;top:0;width:10px;height:10px;border:1.5px solid currentColor;border-radius:50%}
.bs-lockup__mark::after{content:"";position:absolute;right:0;top:0;width:10px;height:10px;background:currentColor}
.bs-lockup__mark i{position:absolute;left:10px;right:10px;top:4.25px;height:1.5px;background:var(--thread)}

.results{display:flex;flex-direction:column;margin-top:4px}
.results .bs-row-btn{display:grid;grid-template-columns:minmax(0,1fr) auto;gap:10px;align-items:baseline;width:100%;padding:8px 2px;border:0;border-top:1px solid var(--rule);background:transparent;text-align:left;cursor:pointer}
.results .bs-row-btn:hover{background:var(--paper-2)}
.results .bs-row-btn .id{font-family:var(--font-mono);font-size:12.5px;color:var(--ink);overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
.results .bs-row-btn .nm{font-size:12px;color:var(--ink-3);white-space:nowrap}
.saved .bs-row-btn{grid-template-columns:minmax(0,1fr) auto auto auto;}
.saved .src{font-family:var(--font-mono);font-size:10px;letter-spacing:.06em;text-transform:uppercase;color:var(--ink-3)}
.saved .del{font-size:14px;color:var(--ink-3);padding:0 6px}

.wrap{max-width:1360px;margin:0 auto;padding:24px 32px 48px}
header.top{display:flex;align-items:flex-end;justify-content:space-between;gap:16px;margin:8px 0 24px}
.subline{margin:2px 0 0;color:var(--ink-3);font-size:13px}
.topright{display:flex;align-items:center;gap:16px}
.one-liner{font-size:13px;line-height:1.45;color:var(--ink-3);max-width:330px;text-align:right}
.stats{display:grid;grid-template-columns:repeat(4,1fr);gap:16px;margin-bottom:28px}
@media (max-width:900px){.stats{grid-template-columns:repeat(2,1fr)}}
.stat-head{display:flex;align-items:center;justify-content:space-between;gap:8px}
.grid2{display:grid;grid-template-columns:1fr 1fr;gap:16px;margin-bottom:16px}
@media (max-width:1100px){.grid2{grid-template-columns:1fr}}
.filters{display:flex;gap:28px;flex-wrap:wrap;align-items:flex-end;margin:0 0 28px}
.filters .bs-field{width:180px}
.filters .fright{margin-left:auto;display:flex;align-items:center;gap:16px}
table.turns{width:100%;border-collapse:collapse;font-family:var(--font-mono);font-size:12px;font-variant-numeric:tabular-nums}
table.turns th{text-align:left;font-weight:400;letter-spacing:.06em;text-transform:uppercase;color:var(--ink-3);border-bottom:1px solid var(--rule-2);padding:6px 8px 6px 0}
table.turns td{border-bottom:1px solid var(--rule);padding:7px 8px 7px 0;white-space:nowrap}
table.turns td.num{text-align:right}
table.turns tbody tr:hover td{background:var(--paper-2)}
footer.foot{display:flex;flex-wrap:wrap;gap:10px 24px;align-items:center;margin-top:24px;padding-top:16px;border-top:1px solid var(--rule)}
.mono-sm{font-family:var(--font-mono);font-size:12px;color:var(--ink-3)}
.emptybox{padding:24px;background-image:linear-gradient(var(--grid) 1px,transparent 1px),linear-gradient(90deg,var(--grid) 1px,transparent 1px);background-size:16px 16px;border:1px solid var(--rule);border-radius:4px;margin-bottom:16px}
</style>
</head>
<body>
<div class="wrap">

<header class="top">
  <div>
    <span class="bs-lockup">
      <span style="margin-top:4px;display:inline-flex"><span class="bs-lockup__mark"><i></i></span></span>
      <span class="bs-lockup__text">
        <span class="bs-lockup__name">TokL - Token Literacy</span>
        <span class="bs-lockup__endorse">A BlasterSystems product</span>
      </span>
    </span>
  </div>
  <div class="topright">
    <span class="one-liner">fresh vs cached input, output, and API-equivalent cost</span>
    <a class="bs-btn bs-btn--primary bs-btn--sm" href="/export.csv?harness={{.Filter.Harness}}&session={{.Filter.Session}}&since={{.Filter.Since}}">Export CSV ↓</a>
    <button class="bs-theme" id="theme-toggle" title="Toggle light/dark" aria-label="Toggle theme"><span></span></button>
  </div>
</header>

{{range .Banners}}<div class="bs-alert"><span class="bs-glyph bs-glyph--recorded"></span><span>{{.}}</span></div>{{end}}
{{if .Empty}}<div class="emptybox"><p class="bs-note">{{.Empty}}</p></div>{{end}}

<div class="stats" id="stats">
  <div class="bs-panel">
    <div class="stat-head"><span class="bs-label">Tokens</span><span class="bs-mark"><span class="bs-glyph bs-glyph--recorded"></span>Recorded · ledger</span></div>
    <div class="bs-figure bs-num">{{.Summary.TotalTokens}}</div>
    <div class="bs-sub">U {{.Summary.InputFresh}} · R {{.Summary.CacheRead}} · W {{.Summary.CacheWrite}} · O {{.Summary.Output}}</div>
  </div>
  <div class="bs-panel">
    <div class="stat-head">
      <span class="bs-label">Cache hit rate</span>
      <button class="bs-info" aria-label="How this is computed">?
        <span class="bs-tip">
          <span class="bs-tip__text">Share of prompt tokens served from cache, per turn and in aggregate.</span>
          <span class="bs-calc">
            <span>R</span><span>{{.Summary.CacheRead}}</span>
            <span>U + R + W</span><span>{{.Summary.PromptTotal}}</span>
            <span class="bs-calc__total">Hit rate</span><span class="bs-calc__total">{{pct .Summary.HitRate}}</span>
          </span>
        </span>
      </button>
    </div>
    <div class="bs-figure bs-num">{{pct .Summary.HitRate}}</div>
    <div class="bs-sub">R ÷ (U+R+W) across {{.Summary.Calls}} turns</div>
  </div>
  <div class="bs-panel">
    <div class="stat-head"><span class="bs-label">Fresh share</span></div>
    <div class="bs-figure bs-num">{{pct .Summary.FreshShare}}</div>
    <div class="bs-sub">U ÷ (U+R+W) — what you paid full price for</div>
  </div>
  <div class="bs-panel">
    <div class="stat-head"><span class="bs-label">Context tax</span></div>
    <div class="bs-figure bs-num">{{ratio .Summary.ContextTax}}×</div>
    <div class="bs-sub">prompt tokens pushed per output token</div>
  </div>
  <div class="bs-panel">
    <div class="stat-head"><span class="bs-label">API-equiv. cost</span>{{if .HasEst}}<span class="bs-mark"><span class="bs-glyph bs-glyph--recorded"></span>≈ default</span>{{end}}</div>
    <div class="bs-figure bs-num">{{if .HasEst}}≈{{end}}{{cost .CostTotal}}</div>
    <div class="bs-sub">{{if .HasEst}}≈{{cost .CostEst}} estimated at current rates · {{.Unpriced}} turns unpriced{{else}}at list rates ({{.Summary.CostedCalls}} turns){{end}}</div>
  </div>
  <div class="bs-panel">
    <div class="stat-head"><span class="bs-label">Cost / output token</span></div>
    <div class="bs-figure bs-num">{{if .HasEst}}≈{{end}}{{cost6 .CostPerOut}}</div>
    <div class="bs-sub">API-equivalent ÷ O</div>
  </div>
  <div class="bs-panel">
    <div class="stat-head"><span class="bs-label">Turns</span></div>
    <div class="bs-figure bs-num">{{.Summary.Calls}}</div>
    <div class="bs-sub">{{.Summary.CostedCalls}} priced at ingest{{if .Unpriced}} · {{.Unpriced}} ≈ default{{end}}</div>
  </div>
</div>

<form class="filters" method="get">
  <div class="bs-field"><span class="bs-field__label">Harness</span>
    <div class="bs-field__box"><select class="bs-field__input" name="harness" aria-label="Harness">
      <option value="">all</option>
      <option value="claude-code"{{sel "claude-code" .Filter.Harness}}>claude-code</option>
      <option value="codex"{{sel "codex" .Filter.Harness}}>codex</option>
    </select></div></div>
  <div class="bs-field"><span class="bs-field__label">Session</span>
    <div class="bs-field__box"><select class="bs-field__input" name="session" aria-label="Session">
      <option value="">all</option>
      {{range .Sessions}}<option value="{{.SessionID}}"{{sel .SessionID $.Filter.Session}}>{{.Harness}} · {{.SessionID}} ({{.Turns}})</option>{{end}}
    </select></div></div>
  <div class="bs-field"><span class="bs-field__label">Since</span>
    <div class="bs-field__box"><select class="bs-field__input" name="since" aria-label="Since">
      <option value=""{{sel "" .Filter.Since}}>all time</option>
      <option value="1d"{{sel "1d" .Filter.Since}}>1 day</option>
      <option value="7d"{{sel "7d" .Filter.Since}}>7 days</option>
      <option value="30d"{{sel "30d" .Filter.Since}}>30 days</option>
    </select></div></div>
  <span class="fright">
    <button class="bs-btn bs-btn--secondary bs-btn--sm" type="submit">Apply</button>
    <a class="bs-link" href="/export.json?harness={{.Filter.Harness}}&session={{.Filter.Session}}&since={{.Filter.Since}}">export.json</a>
  </span>
</form>

<div class="grid2">
  <div class="bs-panel">
    <div class="bs-panel__head"><span class="bs-label">Per-turn tokens</span><span class="bs-panel__aside">oldest → newest</span></div>
    <div class="bs-chart">
      <div style="display:flex;align-items:stretch">
        <div style="position:relative;width:44px;flex:none;margin-top:6px" id="bars-y"></div>
        <div id="bars" style="height:190px;flex:1;min-width:0;overflow-x:auto"></div>
      </div>
      <div class="bs-chart__x" id="bars-x"><span id="bars-x0"></span><span id="bars-x1" style="left:100%;transform:translateX(-100%)"></span></div>
      <div class="bs-legend"><span><span class="sw sw--u"></span>U fresh</span><span><span class="sw sw--r"></span>R cache read</span><span><span class="sw sw--w"></span>W cache write</span><span><span class="sw sw--o"></span>O output</span></div>
    </div>
  </div>
  <div class="bs-panel">
    <div class="bs-panel__head"><span class="bs-label">Cache hit rate</span><span class="bs-panel__aside">R ÷ (U+R+W)</span></div>
    <div class="bs-chart">
      <div id="rate" style="height:190px"></div>
      <div class="bs-chart__x"><span id="rate-x0"></span><span id="rate-x1" style="left:100%;transform:translateX(-100%)"></span></div>
      <div class="bs-legend"><span><span class="sw sw--r"></span>per turn</span><span><span class="sw" style="border-top:1.5px dashed var(--ink-3);height:0;width:12px"></span>0%</span></div>
    </div>
  </div>
</div>

<div class="bs-panel" style="margin-bottom:16px">
  <div class="bs-panel__head">
    <span class="bs-label">Model pricing</span>
    <span class="bs-panel__aside">openrouter.ai · public catalog · no key</span>
  </div>
  <div class="bs-field bs-field--mono">
    <span class="bs-field__label">Search a model</span>
    <div class="bs-field__box">
      <span style="font-family:var(--font-mono);font-size:14px;color:var(--ink-3)">$/1M</span>
      <input class="bs-field__input" id="model-search" type="search" placeholder="e.g. claude sonnet 4.5" autocomplete="off" spellcheck="false">
    </div>
    <span class="bs-field__hint">Prices populate the ledger's API-equivalent rates — same U / R / W / O split. Saved rates apply to future turns; existing rows keep their price.</span>
  </div>
  <div class="results" id="model-results"></div>
  <div id="price-edit" style="display:none">
    <div class="filters" style="margin:8px 0 0">
      <div class="bs-field bs-field--mono"><span class="bs-field__label">Input (fresh) $/1M</span><div class="bs-field__box"><input class="bs-field__input" id="rate-input" type="number" step="any" min="0"></div></div>
      <div class="bs-field bs-field--mono"><span class="bs-field__label">Cache read $/1M</span><div class="bs-field__box"><input class="bs-field__input" id="rate-read" type="number" step="any" min="0"></div></div>
      <div class="bs-field bs-field--mono"><span class="bs-field__label">Cache write $/1M</span><div class="bs-field__box"><input class="bs-field__input" id="rate-write" type="number" step="any" min="0"></div></div>
      <div class="bs-field bs-field--mono"><span class="bs-field__label">Output $/1M</span><div class="bs-field__box"><input class="bs-field__input" id="rate-output" type="number" step="any" min="0"></div></div>
      <div class="bs-field bs-field--mono" style="flex:1"><span class="bs-field__label">Applies to model (prefix)</span><div class="bs-field__box"><input class="bs-field__input" id="rate-model" type="text"></div></div>
    </div>
    <div style="display:flex;gap:12px;align-items:center;margin-top:10px">
      <button class="bs-btn bs-btn--primary bs-btn--sm" id="rate-save">Save rate</button>
      <span class="mono-sm" id="rate-msg"></span>
    </div>
  </div>
  <div class="saved" id="saved-block" style="margin-top:10px">
    <span class="bs-label">Saved rates</span>
    <div class="results" id="saved-rates"></div>
  </div>
</div>

<div class="bs-panel">
  <div class="bs-panel__head"><span class="bs-label">Turns</span><span class="bs-panel__aside">{{.Summary.Calls}} turns</span></div>
  <div style="overflow-x:auto">
  <table class="turns">
    <thead><tr><th>time</th><th>harness</th><th>model</th><th style="text-align:right">U</th><th style="text-align:right">R</th><th style="text-align:right">W</th><th style="text-align:right">O</th><th style="text-align:right">hit %</th><th style="text-align:right">cost</th><th>source</th></tr></thead>
    <tbody>
    {{range .Rows}}<tr><td>{{.Time}}</td><td>{{.Harness}}</td><td>{{.Model}}</td><td class="num">{{.U}}</td><td class="num">{{.R}}</td><td class="num">{{.W}}</td><td class="num">{{.O}}</td><td class="num">{{.HitPct}}</td><td class="num">{{.Cost}}</td><td>{{.Source}}</td></tr>{{end}}
    </tbody>
  </table>
  </div>
</div>

<footer class="foot">
  <span class="mono-sm">local only{{if .Version}} · {{.Version}}{{end}}</span>
  <span class="mono-sm">SQLite · {{.DBPath}}</span>
  <span class="mono-sm">no telemetry · no prompt content</span>
  <span style="margin-left:auto" class="mono-sm">≈ = priced at the Claude Opus 5.5 default</span>
</footer>

</div>
<script>
/* ===== theme: system default, toggle remembered ===== */
(function(){
  var saved = null;
  try { saved = localStorage.getItem('tokl-theme'); } catch (e) {}
  if (saved === 'dark' || saved === 'light') document.documentElement.dataset.theme = saved;
  document.getElementById('theme-toggle').addEventListener('click', function(){
    var h = document.documentElement;
    var next = (h.dataset.theme || (window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light')) === 'dark' ? 'light' : 'dark';
    h.dataset.theme = next;
    try { localStorage.setItem('tokl-theme', next); } catch (e) {}
  });
})();

const CHART = {{.ChartJSON}};
const fmt = n => n.toLocaleString('en-US');
const H = 170, PADT = 10;
const dt = t => new Date(t).toLocaleString('en-US',{month:'short',day:'numeric',hour:'2-digit',minute:'2-digit',hour12:false});

function barsSVG(data, W){
  const n = data.length, slot = W / n;
  const bw = Math.max(4, Math.min(22, Math.floor(slot * 0.66)));
  const xAt = i => i * slot + (slot - bw) / 2;
  const maxTot = Math.max(...data.map(d => d.u + d.r + d.w + d.o), 1);
  let out = '';
  [0,.25,.5,.75,1].forEach(f => { const y = PADT + (H - PADT) * f;
    out += '<line x1="0" x2="' + W + '" y1="' + y + '" y2="' + y + '" stroke="' + (f===0?'var(--rule-2)':'var(--rule)') + '" stroke-width="1"/>'; });
  data.forEach((d, i) => { let y = H;
    [['u',d.u,'var(--ink-2)'],['r',d.r,'var(--ink)'],['w',d.w,'var(--rule-2)']].forEach(([k,v,c]) => {
      if (!v) return; const h = (H - PADT) * v / maxTot; y -= h;
      out += '<rect x="' + xAt(i) + '" y="' + y + '" width="' + bw + '" height="' + h + '" fill="' + c + '"><title>U ' + fmt(d.u) + ' · R ' + fmt(d.r) + ' · W ' + fmt(d.w) + ' · O ' + fmt(d.o) + '</title></rect>'; });
    if (d.o) { const h = (H - PADT) * d.o / maxTot; y -= h;
      out += '<rect x="' + xAt(i) + '" y="' + y + '" width="' + bw + '" height="' + Math.max(h,2) + '" fill="none" stroke="var(--thread)" stroke-width="1.5"><title>O ' + fmt(d.o) + '</title></rect>'; } });
  return '<svg width="' + W + '" height="' + H + '" viewBox="0 0 ' + W + ' ' + H + '" style="display:block">' + out + '</svg>';
}

function rateSVG(data, W){
  const L = 8, R = 44, inner = W - L - R;
  const xAt = i => L + inner * (data.length === 1 ? 0 : i / (data.length - 1));
  const yFor = v => H - (H - PADT) * (v / 100);
  let out = '';
  [0,.25,.5,.75,1].forEach(f => { const y = yFor(f * 100); const zero = f === 0;
    out += '<line x1="0" x2="' + W + '" y1="' + y + '" y2="' + y + '" stroke="' + (zero ? 'var(--ink-3)' : 'var(--rule)') + '" stroke-width="1"' + (zero ? ' stroke-dasharray="4 3"' : '') + '/>'
      + '<text x="' + (W-6) + '" y="' + (y-4) + '" text-anchor="end" font-family="DM Mono" font-size="10" fill="var(--ink-3)">' + (f*100) + '%</text>'; });
  const pts = data.map((d, i) => [xAt(i), yFor(d.hit)]);
  out += '<polyline points="' + pts.map(p => p.join(',')).join(' ') + '" fill="none" stroke="var(--ink)" stroke-width="1.5"/>';
  const peak = data.reduce((b, d, i) => d.hit > data[b].hit ? i : b, 0);
  if (data.length && data[peak].hit > 0) {
    const px = pts[peak][0], py = pts[peak][1];
    const anchor = px > W - 120 ? 'end' : 'start', tx = anchor === 'end' ? px - 8 : px + 8;
    out += '<circle cx="' + px + '" cy="' + py + '" r="3.5" fill="var(--card)" stroke="var(--ink)" stroke-width="1.5"/>';
    out += '<text x="' + tx + '" y="' + (py-8) + '" text-anchor="' + anchor + '" font-family="DM Mono" font-size="10" fill="var(--ink-2)">' + data[peak].hit.toFixed(1) + '% peak</text>';
  }
  [pts[0], pts[pts.length-1]].forEach(p => { out += '<circle cx="' + p[0] + '" cy="' + p[1] + '" r="3.5" fill="var(--card)" stroke="var(--ink-3)" stroke-width="1.5"/>'; });
  return '<svg width="' + W + '" height="' + H + '" viewBox="0 0 ' + W + ' ' + H + '" style="display:block">' + out + '</svg>';
}

function renderCharts(){
  const bars = document.getElementById('bars'), rate = document.getElementById('rate');
  if (!CHART.length) {
    bars.innerHTML = '<p class="bs-note" style="padding:8px">no turns in range</p>';
    rate.innerHTML = '<p class="bs-note" style="padding:8px">no turns in range</p>';
    return;
  }
  bars.innerHTML = barsSVG(CHART, Math.max(bars.clientWidth || 320, 320));
  document.getElementById('bars-y').innerHTML = [0,.5,1].map(f => {
    const maxTot = Math.max(...CHART.map(d => d.u + d.r + d.w + d.o), 1);
    return '<span class="bs-chart__y" style="top:' + (PADT + (H-PADT)*f) + 'px">' + fmt(Math.round(maxTot * (1-f))) + '</span>';
  }).join('');
  rate.innerHTML = rateSVG(CHART, Math.max(rate.clientWidth || 320, 320));
  const x0 = dt(CHART[0].t), x1 = dt(CHART[CHART.length-1].t);
  document.getElementById('bars-x0').textContent = x0; document.getElementById('bars-x1').textContent = x1;
  document.getElementById('rate-x0').textContent = x0; document.getElementById('rate-x1').textContent = x1;
}
renderCharts();
let resizeTimer = null;
window.addEventListener('resize', function(){
  clearTimeout(resizeTimer);
  resizeTimer = setTimeout(renderCharts, 150);
});

/* ===== model pricing: OpenRouter search + cached overrides ===== */
const $ = id => document.getElementById(id);
let PICKED = null;
function bareName(id){ const i = id.indexOf('/'); return i < 0 ? id : id.slice(i + 1); }

function fillEditor(p, modelKey){
  $('price-edit').style.display = '';
  $('rate-input').value = p.prompt != null ? cleanNum(perTok(p.prompt)) : '';
  $('rate-read').value = p.input_cache_read != null ? cleanNum(perTok(p.input_cache_read)) : '';
  $('rate-write').value = p.input_cache_write != null ? cleanNum(perTok(p.input_cache_write)) : 0;
  $('rate-output').value = p.completion != null ? cleanNum(perTok(p.completion)) : '';
  $('rate-model').value = modelKey;
}
$('rate-save').addEventListener('click', async () => {
  const body = {
    model: $('rate-model').value.trim(), name: PICKED ? (PICKED.name || '') : '',
    input: parseFloat($('rate-input').value) || 0,
    cache_read: parseFloat($('rate-read').value) || 0,
    cache_write: parseFloat($('rate-write').value) || 0,
    output: parseFloat($('rate-output').value) || 0,
    source: 'manual'
  };
  if (!body.model) { $('rate-msg').textContent = 'Applies-to model is required.'; return; }
  const res = await fetch('/api/rates', {method:'PUT', headers:{'Content-Type':'application/json'}, body: JSON.stringify(body)});
  $('rate-msg').textContent = res.ok ? 'Saved — applies to future turns.' : ('Save failed: ' + res.status);
  if (res.ok) loadSaved();
});

async function cachePick(m){
  // Persist the catalog snapshot under its bare name so harness model ids
  // (which lack the vendor prefix) prefix-match. OpenRouter prices are
  // per-token strings; the ledger stores USD per million tokens. Anthropic
  // catalog ids use dotted versions ("claude-sonnet-4.5") while Claude Code
  // telemetry reports dashed ones ("claude-sonnet-4-5-20250929"), so dots
  // become dashes for anthropic/* ids to keep the prefix match alive.
  const p = m.pricing || {};
  let bare = bareName(m.id);
  if (m.id.startsWith('anthropic/')) bare = bare.replace(/\./g, '-');
  const body = { model: bare, name: m.name || '', input: Number(cleanNum(perTok(p.prompt))),
    cache_read: Number(cleanNum(perTok(p.input_cache_read))), cache_write: Number(cleanNum(perTok(p.input_cache_write))),
    output: Number(cleanNum(perTok(p.completion))), source: 'openrouter' };
  await fetch('/api/rates', {method:'PUT', headers:{'Content-Type':'application/json'}, body: JSON.stringify(body)});
  loadSaved();
  return bare;
}
const perTok = v => v == null ? 0 : Number(v) * 1e6;
const cleanNum = v => v === '' ? '' : String(Number(Number(v).toPrecision(12)));
const esc = s => String(s ?? '').replace(/[&<>"']/g, c => ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));

let MODELS = null;
async function ensureCatalog(){
  if (MODELS) return MODELS;
  const res = await fetch('https://openrouter.ai/api/v1/models');
  const d = await res.json();
  MODELS = d.data || [];
  return MODELS;
}
$('model-search').addEventListener('input', async e => {
  const q = e.target.value.trim().toLowerCase();
  const box = $('model-results');
  if (!q) { box.innerHTML = ''; return; }
  let models;
  try { models = await ensureCatalog(); }
  catch (err) { box.innerHTML = '<span class="bs-note">Catalog unreachable — saved rates below still apply.</span>'; return; }
  const hits = models.filter(m => m.id.toLowerCase().includes(q) || (m.name||'').toLowerCase().includes(q)).slice(0, 8);
  box.innerHTML = hits.map(m => '<button type="button" class="bs-row-btn" data-id="' + esc(m.id) + '"><span class="id">' + esc(m.id) + '</span><span class="nm">' + esc(m.name||'') + '</span></button>').join('') || '<span class="bs-note">No match in the OpenRouter catalog.</span>';
  box.querySelectorAll('.bs-row-btn').forEach(b => b.addEventListener('click', async () => {
    const m = models.find(x => x.id === b.dataset.id);
    PICKED = m;
    const bare = await cachePick(m);
    fillEditor(m.pricing || {}, bare);
    $('model-search').value = bare;
    box.innerHTML = '';
  }));
});

async function loadSaved(){
  const res = await fetch('/api/rates');
  const d = await res.json();
  const list = d.rates || [];
  $('saved-block').style.display = list.length ? '' : 'none';
  $('saved-rates').innerHTML = list.map(r =>
    '<button type="button" class="bs-row-btn" data-m="' + esc(r.model) + '">'
    + '<span class="id">' + esc(r.model) + '</span>'
    + '<span class="nm">$' + r.input + ' in · $' + r.cache_read + ' read · $' + r.output + ' out /1M</span>'
    + '<span class="src">' + esc(r.source) + '</span>'
    + '<span class="del" title="Remove" data-del="' + esc(r.model) + '">✕</span></button>').join('');
  $('saved-rates').querySelectorAll('[data-m]').forEach(b => b.addEventListener('click', e => {
    if (e.target.dataset.del) return;
    const row = list.find(r => r.model === b.dataset.m);
    // Stored rates are already per-1M: divide back to per-token so the
    // editor's perTok conversion round-trips exactly.
    if (row) { PICKED = null; $('price-edit').style.display = '';
      fillEditor({prompt: row.input / 1e6, input_cache_read: row.cache_read / 1e6, input_cache_write: row.cache_write / 1e6, completion: row.output / 1e6}, row.model); }
  }));
  $('saved-rates').querySelectorAll('[data-del]').forEach(b => b.addEventListener('click', async e => {
    e.stopPropagation();
    await fetch('/api/rates?model=' + encodeURIComponent(b.dataset.del), {method:'DELETE'});
    loadSaved();
  }));
}
loadSaved();
</script>
</body>
</html>
`

// handleDashboard renders the page from the current filter.
func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	if s.db == nil {
		http.Error(w, "ledger unavailable", http.StatusServiceUnavailable)
		return
	}
	filter := queryFilter(r)
	sum, err := s.db.Summary(filter)
	if err != nil {
		httpError(w, err)
		return
	}
	rows, err := s.db.Rows(filter)
	if err != nil {
		httpError(w, err)
		return
	}
	if len(rows) > 500 {
		rows = rows[:500]
	}
	sessions, err := s.db.Sessions(filter.SinceMS, filter.Harness)
	if err != nil {
		httpError(w, err)
		return
	}
	if len(sessions) > 200 {
		sessions = sessions[:200]
	}

	chart, err := json.Marshal(chartRows(rows))
	if err != nil {
		httpError(w, err)
		return
	}
	estUSD, unpriced, usedDefault, err := s.costEstimate(filter)
	if err != nil {
		httpError(w, err)
		return
	}

	data := dashboardData{
		Version:    s.info.Version,
		DBPath:     s.info.DBPath,
		Filter:     filterView{Harness: filter.Harness, Session: filter.Session, Since: r.URL.Query().Get("since")},
		Summary:    sum,
		Rows:       make([]rowView, 0, len(rows)),
		Sessions:   sessions,
		Banners:    s.banners(r),
		ChartJSON:  template.JS(chart),
		CostEst:    estUSD,
		CostTotal:  sum.CostUSD + estUSD,
		CostPerOut: func() float64 {
			if sum.Output == 0 {
				return 0
			}
			return (sum.CostUSD + estUSD) / float64(sum.Output)
		}(),
		Unpriced:   unpriced,
		HasEst:     unpriced > 0 && usedDefault,
	}

	for _, c := range rows {
		data.Rows = append(data.Rows, rowView{
			Time:    formatTime(c.StartedAt),
			Harness: c.Harness,
			Model:   c.Model,
			U:       c.InputUncached,
			R:       c.CacheRead,
			W:       c.CacheWrite,
			O:       c.Output,
			HitPct:  pct(rowHitRate(c)),
			Cost:    costLabel(c),
			Source:  c.Source,
		})
	}
	data.Empty = s.emptyState(sum)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := dashboardTmpl.Execute(w, data); err != nil {
		httpError(w, err)
	}
}

// pct renders a 0..1 ratio as a percentage string.
func pct(f float64) string { return fmt.Sprintf("%.1f%%", f*100) }

// rowHitRate returns the per-row cache hit rate R/(U+R+W).
func rowHitRate(c ledger.Call) float64 {
	denom := c.InputUncached + c.CacheRead + c.CacheWrite
	if denom == 0 {
		return 0
	}
	return float64(c.CacheRead) / float64(denom)
}

func costLabel(c ledger.Call) string {
	if c.CostUSD == nil {
		return "n/a"
	}
	// default_price rows are approximations at the Claude Opus 5.5 fallback.
	if c.CostSource == "default_price" {
		return "≈" + fmt.Sprintf("$%.4f", *c.CostUSD)
	}
	return fmt.Sprintf("$%.4f", *c.CostUSD)
}

// banners assembles the empty-state and warning banners from live state.
func (s *Server) banners(r *http.Request) []string {
	var out []string
	if s.info.CodexDir != "" && !s.info.CodexDirExists {
		out = append(out, fmt.Sprintf("Codex directory not found at %s — Codex rows will not appear until it exists.", s.info.CodexDir))
	}
	if warn, _ := s.db.GetMeta("codex_last_warning"); warn != "" {
		out = append(out, "Codex parser warning: "+warn)
	}
	if agg, _ := s.db.AggregateCount("claude-code"); agg > 0 {
		perTurn, _ := s.db.PerTurnCount("claude-code")
		if perTurn == 0 {
			out = append(out, "Claude Code data is aggregate-only: no claude_code.llm_request spans received. Per-turn rows need CLAUDE_CODE_ENHANCED_TELEMETRY_BETA=1 (tokenledger setup writes it).")
		}
	}
	if s.info.WSL {
		out = append(out, "WSL detected: the collector binds Windows loopback. If your browser runs in Windows, open http://127.0.0.1:4318 there; check the WSL host-IP and firewall if unreachable.")
	}
	return out
}

// emptyState returns the primary empty-state message ("" when data exists).
func (s *Server) emptyState(sum ledger.Summary) string {
	if sum.Calls > 0 {
		return ""
	}
	claude, _ := s.db.LastSourceTime("claude-code")
	codex, _ := s.db.LastSourceTime("codex")
	switch {
	case claude == 0 && codex == 0:
		return "No turns recorded yet. Run `tokenledger setup` to configure Claude Code telemetry, start a Claude Code or Codex turn, and rows appear within seconds. Codex backfills its existing history automatically."
	case claude == 0:
		return "No Claude Code turns yet. Run `tokenledger setup` and complete one Claude Code turn; spans arrive via OTLP."
	default:
		return "No Codex turns yet. Codex rollouts are backfilled from ~/.codex/sessions on start."
	}
}

// ascending returns rows oldest-first (charts read left to right).
func ascending(rows []ledger.Call) []ledger.Call {
	out := make([]ledger.Call, len(rows))
	copy(out, rows)
	sort.Slice(out, func(i, j int) bool { return out[i].StartedAt < out[j].StartedAt })
	return out
}

// costEstimate prices rows stored without a cost (older ingests, or models
// priced after the fact) at the current effective rates. Exports keep the
// raw stored values; this is display-only.
func (s *Server) costEstimate(f ledger.RowsFilter) (estUSD float64, calls int64, usedDefault bool, err error) {
	groups, err := s.db.UnpricedGroups(f)
	if err != nil {
		return 0, 0, false, err
	}
	ov := s.effectiveOverride()
	for _, g := range groups {
		c, src := pricing.CostSourced(g.Model, ov, g.InputFresh, g.CacheRead, g.CacheWrite, g.Output)
		estUSD += c
		calls += g.Calls
		if src == pricing.SourceDefaultCost {
			usedDefault = true
		}
	}
	return estUSD, calls, usedDefault, nil
}
