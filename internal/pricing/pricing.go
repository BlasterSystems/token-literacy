// Package pricing computes API-equivalent cost from list prices.
//
// Prices are USD per million tokens. Cache-write rates encode the effective
// 5-minute prompt-cache premium (1.25x input for Anthropic); OpenAI does not
// bill cache writes. Unknown models return ok=false and the row is stored
// with cost_source=none. All figures are API list prices: subscription plans
// are not billed per token, and the dashboard labels costs accordingly.
package pricing

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// Price is a per-model rate table in USD per million tokens.
type Price struct {
	Input      float64 `json:"input"`
	CacheRead  float64 `json:"cache_read"`
	CacheWrite float64 `json:"cache_write"`
	Output     float64 `json:"output"`
}

type entry struct {
	prefix string
	p      Price
}

// table is ordered longest-prefix first; the first match wins.
var table = []entry{
	// Anthropic (claude-<family>-<version>[-date])
	{"claude-opus-4-5", Price{Input: 5, CacheRead: 0.5, CacheWrite: 6.25, Output: 25}},
	{"claude-opus-4-1", Price{Input: 5, CacheRead: 0.5, CacheWrite: 6.25, Output: 25}},
	{"claude-opus-4", Price{Input: 5, CacheRead: 0.5, CacheWrite: 6.25, Output: 25}},
	{"claude-sonnet-4-5", Price{Input: 3, CacheRead: 0.3, CacheWrite: 3.75, Output: 15}},
	{"claude-sonnet-4", Price{Input: 3, CacheRead: 0.3, CacheWrite: 3.75, Output: 15}},
	{"claude-haiku-4-5", Price{Input: 1, CacheRead: 0.1, CacheWrite: 1.25, Output: 5}},
	{"claude-haiku-4", Price{Input: 0.8, CacheRead: 0.08, CacheWrite: 1, Output: 4}},
	{"claude-3-7-sonnet", Price{Input: 3, CacheRead: 0.3, CacheWrite: 3.75, Output: 15}},
	{"claude-3-5-sonnet", Price{Input: 3, CacheRead: 0.3, CacheWrite: 3.75, Output: 15}},
	{"claude-3-5-haiku", Price{Input: 0.8, CacheRead: 0.08, CacheWrite: 1, Output: 4}},
	{"claude-3-opus", Price{Input: 15, CacheRead: 1.5, CacheWrite: 18.75, Output: 75}},
	{"claude-3-sonnet", Price{Input: 3, CacheRead: 0.3, CacheWrite: 3.75, Output: 15}},
	{"claude-3-haiku", Price{Input: 0.25, CacheRead: 0.03, CacheWrite: 0.3, Output: 1.25}},
	// OpenAI (Codex)
	{"gpt-5.2", Price{Input: 1.25, CacheRead: 0.125, Output: 10}},
	{"gpt-5.1", Price{Input: 1.25, CacheRead: 0.125, Output: 10}},
	{"gpt-5", Price{Input: 1.25, CacheRead: 0.125, Output: 10}},
	{"gpt-4.1", Price{Input: 2, CacheRead: 0.5, Output: 8}},
	{"gpt-4o", Price{Input: 2.5, CacheRead: 1.25, Output: 10}},
	{"codex-mini-latest", Price{Input: 1.5, CacheRead: 0.15, Output: 6}},
	{"o4-mini", Price{Input: 1.1, CacheRead: 0.275, Output: 4.4}},
	{"o3", Price{Input: 2, CacheRead: 0.5, Output: 8}},
}

// Table returns a copy of the embedded table (for doctor/debug output).
func Table() map[string]Price {
	m := make(map[string]Price, len(table))
	for _, e := range table {
		m[e.prefix] = e.p
	}
	return m
}

// Override is a user-supplied price override keyed by model prefix.
type Override map[string]Price

// LoadOverride reads ~/.tokenledger/pricing.json if present. A missing file
// is not an error; an unparsable one is.
func LoadOverride(path string) (Override, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var o Override
	if err := json.Unmarshal(data, &o); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return o, nil
}

// Lookup returns the price for a model id. User overrides win over the
// embedded table. ok=false means the model has no list price.
func Lookup(model string, override Override) (Price, bool) {
	m := strings.ToLower(model)
	for prefix, p := range override {
		if strings.HasPrefix(m, strings.ToLower(prefix)) {
			return p, true
		}
	}
	for _, e := range table {
		if strings.HasPrefix(m, e.prefix) {
			return e.p, true
		}
	}
	return Price{}, false
}

// Cost computes the API-equivalent cost in USD for one row.
func Cost(model string, override Override, inputUncached, cacheRead, cacheWrite, output int64) (float64, bool) {
	p, ok := Lookup(model, override)
	if !ok {
		return 0, false
	}
	const mtok = 1_000_000.0
	cost := float64(inputUncached)*p.Input/mtok +
		float64(cacheRead)*p.CacheRead/mtok +
		float64(cacheWrite)*p.CacheWrite/mtok +
		float64(output)*p.Output/mtok
	return cost, true
}
