// Package doctor explains the current installation state in one line per
// check: config, collector reachability, ledger health, and data freshness.
package doctor

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/BlasterSystems/token-literacy/internal/ledger"
)

// Inputs carries everything doctor needs; zero values skip checks.
type Inputs struct {
	DBPath         string
	ClaudeSettings string
	ExpectedClaude map[string]string
	CollectorURL   string
	OverridePath   string
	HTTPClient     *http.Client
}

// Check is one diagnostic line.
type Check struct {
	Name   string `json:"name"`
	Status string `json:"status"` // ok | warn | fail
	Detail string `json:"detail"`
}

// Report is the ordered list of checks.
type Report struct {
	Checks []Check `json:"checks"`
}

// Failures counts checks with status fail.
func (r Report) Failures() int {
	n := 0
	for _, c := range r.Checks {
		if c.Status == "fail" {
			n++
		}
	}
	return n
}

// Render prints the report in the human "status name: detail" format.
func (r Report) Render(w io.Writer) {
	for _, c := range r.Checks {
		fmt.Fprintf(w, "%-4s %-22s %s\n", c.Status, c.Name, c.Detail)
	}
}

// Run executes all configured checks.
func Run(in Inputs) Report {
	var r Report
	r.Checks = append(r.Checks, checkLedger(in.DBPath)...)
	r.Checks = append(r.Checks, checkClaudeConfig(in.ClaudeSettings, in.ExpectedClaude))
	r.Checks = append(r.Checks, checkCollector(in.CollectorURL, in.HTTPClient))
	r.Checks = append(r.Checks, checkPricingOverride(in.OverridePath))
	return r
}

func checkLedger(dbPath string) []Check {
	if dbPath == "" {
		return []Check{{Name: "ledger", Status: "fail", Detail: "no database path configured"}}
	}
	if _, err := os.Stat(dbPath); err != nil {
		return []Check{{Name: "ledger", Status: "fail", Detail: "no ledger yet (start `tokenledger up` to create " + dbPath + ")"}}
	}
	db, err := ledger.Open(dbPath)
	if err != nil {
		return []Check{{Name: "ledger", Status: "fail", Detail: err.Error()}}
	}
	defer db.Close()

	integrity, err := db.IntegrityCheck()
	if err != nil {
		return []Check{{Name: "ledger", Status: "fail", Detail: "integrity_check: " + err.Error()}}
	}
	total, byHarness, err := db.Counts()
	if err != nil {
		return []Check{{Name: "ledger", Status: "fail", Detail: "counts: " + err.Error()}}
	}
	checks := []Check{{
		Name:   "ledger",
		Status: "ok",
		Detail: fmt.Sprintf("%s (%d calls: %v)", integrity, total, byHarness),
	}}

	for _, tc := range []struct {
		harness, name string
	}{{"claude-code", "claude-spans"}, {"codex", "codex-rows"}} {
		last, err := db.LastSourceTime(tc.harness)
		if err != nil {
			checks = append(checks, Check{Name: tc.name, Status: "warn", Detail: "query failed: " + err.Error()})
			continue
		}
		if last == 0 {
			checks = append(checks, Check{Name: tc.name, Status: "warn", Detail: "no rows yet"})
			continue
		}
		age := time.Since(time.UnixMilli(last)).Round(time.Second)
		checks = append(checks, Check{Name: tc.name, Status: "ok", Detail: fmt.Sprintf("last row %s ago", age)})
	}
	return checks
}

func checkClaudeConfig(path string, expected map[string]string) Check {
	if path == "" {
		return Check{Name: "claude-config", Status: "fail", Detail: "no settings path"}
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return Check{Name: "claude-config", Status: "fail", Detail: path + " does not exist (run `tokenledger setup`)"}
	}
	if err != nil {
		return Check{Name: "claude-config", Status: "fail", Detail: err.Error()}
	}
	var root map[string]any
	if err := json.Unmarshal(data, &root); err != nil {
		return Check{Name: "claude-config", Status: "fail", Detail: path + " is not valid JSON"}
	}
	envObj, _ := root["env"].(map[string]any)
	if envObj == nil {
		return Check{Name: "claude-config", Status: "fail", Detail: "no env block (run `tokenledger setup`)"}
	}
	var missing []string
	for k, want := range expected {
		got, ok := envObj[k]
		if !ok {
			missing = append(missing, k)
		} else if fmt.Sprint(got) != want {
			missing = append(missing, fmt.Sprintf("%s=%v", k, got))
		}
	}
	if len(missing) > 0 {
		return Check{Name: "claude-config", Status: "fail", Detail: fmt.Sprintf("mismatched/missing: %v (run `tokenledger setup`)", missing)}
	}
	return Check{Name: "claude-config", Status: "ok", Detail: path}
}

func checkCollector(url string, client *http.Client) Check {
	if url == "" {
		return Check{Name: "collector", Status: "warn", Detail: "no collector URL configured"}
	}
	if client == nil {
		client = &http.Client{Timeout: 3 * time.Second}
	}
	resp, err := client.Get(url)
	if err != nil {
		return Check{Name: "collector", Status: "fail", Detail: "not reachable at " + url + " (start `tokenledger up`)"}
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Check{Name: "collector", Status: "fail", Detail: fmt.Sprintf("%s returned %d", url, resp.StatusCode)}
	}
	var body struct {
		Status  string `json:"status"`
		Version string `json:"version"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&body)
	return Check{Name: "collector", Status: "ok", Detail: fmt.Sprintf("%s v%s", url, body.Version)}
}

func checkPricingOverride(path string) Check {
	if path == "" {
		return Check{Name: "pricing-override", Status: "ok", Detail: "none (embedded list prices)"}
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return Check{Name: "pricing-override", Status: "ok", Detail: "none (embedded list prices)"}
	}
	if err != nil {
		return Check{Name: "pricing-override", Status: "fail", Detail: err.Error()}
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return Check{Name: "pricing-override", Status: "fail", Detail: path + " is not valid JSON"}
	}
	return Check{Name: "pricing-override", Status: "ok", Detail: fmt.Sprintf("%s (%d entries)", path, len(m))}
}
