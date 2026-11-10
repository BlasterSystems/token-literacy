package server

import (
	"fmt"
	"html/template"
	"net/http"
	"strings"

	"github.com/BlasterSystems/token-literacy/internal/ledger"
)

// dashboardData is everything the template needs.
type dashboardData struct {
	Version  string
	Filter   filterView
	Summary  ledger.Summary
	Rows     []rowView
	Sessions []ledger.SessionRow
	Banners  []string
	Empty    string
	BarsSVG  template.HTML
	RateSVG  template.HTML
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

// dashboardTmpl renders the dashboard; no external assets are referenced.
var dashboardTmpl = template.Must(template.New("dashboard").Funcs(template.FuncMap{
	"pct":   func(f float64) string { return fmt.Sprintf("%.1f%%", f*100) },
	"ratio": func(f float64) string { return fmt.Sprintf("%.2f", f) },
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
<title>token-literacy</title>
<style>
:root{--fg:#111827;--muted:#6b7280;--border:#e5e7eb;--surface:#fff;--accent:#2563eb;--c1:#2563eb;--c2:#10b981;--c3:#f59e0b;--c4:#8b5cf6;--warn:#b45309;}
@media (prefers-color-scheme:dark){:root{--fg:#e5e7eb;--muted:#9ca3af;--border:#374151;--surface:#111827;}}
*{box-sizing:border-box;}
body{font:14px/1.5 -apple-system,BlinkMacSystemFont,"Segoe UI",sans-serif;color:var(--fg);background:var(--surface);margin:0;padding:16px;max-width:1280px;margin-inline:auto;}
h1{font-size:18px;margin:0 0 4px;}
.sub{color:var(--muted);font-size:12px;margin:0 0 16px;}
.cards{display:grid;grid-template-columns:repeat(auto-fit,minmax(150px,1fr));gap:8px;margin-bottom:16px;}
.card{border:1px solid var(--border);border-radius:8px;padding:10px;min-width:0;}
.card .label{color:var(--muted);font-size:11px;text-transform:uppercase;letter-spacing:.03em;}
.card .value{font-size:17px;font-weight:600;overflow-wrap:anywhere;}
.banner{border:1px solid var(--c3);color:var(--warn);padding:8px 10px;border-radius:8px;margin-bottom:8px;font-size:13px;}
.empty{border:1px dashed var(--border);padding:16px;border-radius:8px;color:var(--muted);margin-bottom:16px;}
form.filters{display:flex;gap:10px;flex-wrap:wrap;align-items:end;margin-bottom:16px;}
form.filters label{display:flex;flex-direction:column;gap:2px;font-size:11px;color:var(--muted);}
select,button{font:inherit;color:var(--fg);background:var(--surface);border:1px solid var(--border);border-radius:6px;padding:4px 6px;}
.svgwrap{overflow-x:auto;border:1px solid var(--border);border-radius:8px;padding:8px;margin-bottom:8px;}
.legend{color:var(--muted);font-size:12px;margin-bottom:16px;}
.legend .sw{display:inline-block;width:10px;height:10px;border-radius:2px;vertical-align:middle;margin-right:3px;}
.tablewrap{overflow-x:auto;}
table{border-collapse:collapse;width:100%;font-size:12px;}
th,td{border-bottom:1px solid var(--border);padding:4px 6px;text-align:right;white-space:nowrap;}
th{color:var(--muted);font-weight:500;}
th:first-child,td:first-child{text-align:left;}
.num{font-variant-numeric:tabular-nums;}
</style>
</head>
<body>
<h1>token-literacy</h1>
<p class="sub">fresh vs cached input, output, and API-equivalent cost per turn — local only{{if .Version}} · {{.Version}}{{end}}</p>
{{range .Banners}}<div class="banner">{{.}}</div>{{end}}
{{if .Empty}}<div class="empty">{{.Empty}}</div>{{end}}
<div class="cards">
  <div class="card"><div class="label">Total tokens</div><div class="value num">{{.Summary.TotalTokens}}</div></div>
  <div class="card"><div class="label">Cache hit rate</div><div class="value num">{{pct .Summary.HitRate}}</div></div>
  <div class="card"><div class="label">Fresh share</div><div class="value num">{{pct .Summary.FreshShare}}</div></div>
  <div class="card"><div class="label">Context tax</div><div class="value num">{{ratio .Summary.ContextTax}}</div></div>
  <div class="card"><div class="label">API-equiv. cost</div><div class="value num">{{cost .Summary.CostUSD}}</div></div>
  <div class="card"><div class="label">Cost / output token</div><div class="value num">{{cost6 .Summary.CostPerOutputToken}}</div></div>
  <div class="card"><div class="label">Turns</div><div class="value num">{{.Summary.Calls}}</div></div>
</div>
<form class="filters" method="get">
  <label>harness
    <select name="harness">
      <option value="">all</option>
      <option value="claude-code"{{sel "claude-code" .Filter.Harness}}>claude-code</option>
      <option value="codex"{{sel "codex" .Filter.Harness}}>codex</option>
    </select>
  </label>
  <label>session
    <select name="session">
      <option value="">all</option>
      {{range .Sessions}}<option value="{{.SessionID}}"{{sel .SessionID $.Filter.Session}}>{{.Harness}} · {{.SessionID}} ({{.Turns}})</option>{{end}}
    </select>
  </label>
  <label>since
    <select name="since">
      <option value=""{{sel "" .Filter.Since}}>all</option>
      <option value="1d"{{sel "1d" .Filter.Since}}>1 day</option>
      <option value="7d"{{sel "7d" .Filter.Since}}>7 days</option>
      <option value="30d"{{sel "30d" .Filter.Since}}>30 days</option>
    </select>
  </label>
  <button type="submit">apply</button>
  <a href="/export.csv?harness={{.Filter.Harness}}&session={{.Filter.Session}}&since={{.Filter.Since}}">export.csv</a>
  <a href="/export.json?harness={{.Filter.Harness}}&session={{.Filter.Session}}&since={{.Filter.Since}}">export.json</a>
</form>
<h2>Per-turn tokens</h2>
<div class="svgwrap">{{.BarsSVG}}</div>
<div class="legend"><span><span class="sw" style="background:var(--c1)"></span>U fresh</span><span><span class="sw" style="background:var(--c2)"></span>R cache read</span><span><span class="sw" style="background:var(--c3)"></span>W cache write</span><span><span class="sw" style="background:var(--c4)"></span>O output</span></div>
<h2>Cache hit rate</h2>
<div class="svgwrap">{{.RateSVG}}</div>
<h2>Turns</h2>
<div class="tablewrap">
<table>
<thead><tr><th>time</th><th>harness</th><th>model</th><th>U</th><th>R</th><th>W</th><th>O</th><th>hit %</th><th>cost</th><th>source</th></tr></thead>
<tbody>
{{range .Rows}}<tr><td>{{.Time}}</td><td>{{.Harness}}</td><td>{{.Model}}</td><td class="num">{{.U}}</td><td class="num">{{.R}}</td><td class="num">{{.W}}</td><td class="num">{{.O}}</td><td class="num">{{.HitPct}}</td><td class="num">{{.Cost}}</td><td>{{.Source}}</td></tr>{{end}}
</tbody>
</table>
</div>
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

	data := dashboardData{
		Version:  s.info.Version,
		Filter:   filterView{Harness: filter.Harness, Session: filter.Session, Since: r.URL.Query().Get("since")},
		Summary:  sum,
		Rows:     make([]rowView, 0, len(rows)),
		Sessions: sessions,
		Banners:  s.banners(r),
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
	data.BarsSVG = template.HTML(stackedBarsSVG(rows))
	data.RateSVG = template.HTML(rateLineSVG(rows))

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

// ---- SVG builders ----

const (
	barsHeight = 160
	rateHeight = 120
	barWidth   = 6
	barStep    = 10
)

// stackedBarsSVG renders one stacked bar per turn (U/R/W/O), oldest first.
func stackedBarsSVG(rows []ledger.Call) string {
	if len(rows) == 0 {
		return `<svg viewBox="0 0 320 40" width="100%" height="40" xmlns="http://www.w3.org/2000/svg"><text x="8" y="24" font-size="12" fill="currentColor">no turns in range</text></svg>`
	}
	asc := ascending(rows)
	maxTotal := int64(1)
	for _, c := range asc {
		if t := c.PromptTotal + c.Output; t > maxTotal {
			maxTotal = t
		}
	}
	width := len(asc)*barStep + 20
	if width < 320 {
		width = 320
	}
	inner := barsHeight - 20
	var b strings.Builder
	fmt.Fprintf(&b, `<svg viewBox="0 0 %d %d" width="100%%" height="%d" preserveAspectRatio="xMinYMid meet" xmlns="http://www.w3.org/2000/svg">`, width, barsHeight, barsHeight)
	for i, c := range asc {
		x := i*barStep + 8
		y := float64(barsHeight - 10)
		stack := []struct {
			v int64
			c string
		}{{c.InputUncached, "var(--c1)"}, {c.CacheRead, "var(--c2)"}, {c.CacheWrite, "var(--c3)"}, {c.Output, "var(--c4)"}}
		for _, seg := range stack {
			if seg.v == 0 {
				continue
			}
			h := float64(seg.v) / float64(maxTotal) * float64(inner)
			if h < 1 {
				h = 1
			}
			y -= h
			fmt.Fprintf(&b, `<rect x="%d" y="%.2f" width="%d" height="%.2f" fill="%s"><title>%s U=%d R=%d W=%d O=%d</title></rect>`,
				x, y, barWidth, h, seg.c, c.ID, c.InputUncached, c.CacheRead, c.CacheWrite, c.Output)
		}
	}
	fmt.Fprintf(&b, `<line x1="0" y1="%d" x2="%d" y2="%d" stroke="var(--border)"/>`, barsHeight-10, width, barsHeight-10)
	fmt.Fprintf(&b, `<text x="4" y="8" font-size="10" fill="var(--muted)">max %d tok/turn</text>`, maxTotal)
	b.WriteString(`</svg>`)
	return b.String()
}

// rateLineSVG renders the cache hit rate per turn (R/(U+R+W)).
func rateLineSVG(rows []ledger.Call) string {
	if len(rows) == 0 {
		return `<svg viewBox="0 0 320 40" width="100%" height="40" xmlns="http://www.w3.org/2000/svg"><text x="8" y="24" font-size="12" fill="currentColor">no turns in range</text></svg>`
	}
	asc := ascending(rows)
	width := len(asc)*barStep + 20
	if width < 320 {
		width = 320
	}
	var b strings.Builder
	fmt.Fprintf(&b, `<svg viewBox="0 0 %d %d" width="100%%" height="%d" preserveAspectRatio="xMinYMid meet" xmlns="http://www.w3.org/2000/svg">`, width, rateHeight, rateHeight)
	for _, gy := range []int{0, 25, 50, 75, 100} {
		y := rateHeight - 10 - gy
		fmt.Fprintf(&b, `<line x1="24" y1="%d" x2="%d" y2="%d" stroke="var(--border)"/>`, y, width-4, y)
		fmt.Fprintf(&b, `<text x="2" y="%d" font-size="9" fill="var(--muted)">%d%%</text>`, y+3, gy)
	}
	var points strings.Builder
	for i, c := range asc {
		x := i*barStep + 8 + barWidth/2
		y := float64(rateHeight-10) - rowHitRate(c)*100
		fmt.Fprintf(&points, "%d,%.1f ", x, y)
	}
	fmt.Fprintf(&b, `<polyline points="%s" fill="none" stroke="var(--accent)" stroke-width="2"/>`, strings.TrimRight(points.String(), " "))
	b.WriteString(`</svg>`)
	return b.String()
}

// ascending returns rows oldest-first (charts read left to right).
func ascending(rows []ledger.Call) []ledger.Call {
	out := make([]ledger.Call, len(rows))
	copy(out, rows)
	for i := range out {
		for j := i + 1; j < len(out); j++ {
			if out[j].StartedAt < out[i].StartedAt {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out
}
