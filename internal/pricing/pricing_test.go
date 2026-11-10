package pricing

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLookupPrefixAndDate(t *testing.T) {
	for _, model := range []string{
		"claude-sonnet-4-5-20250929",
		"claude-sonnet-4-5",
		"claude-opus-4-5",
		"gpt-5.2-2026-01-15",
		"gpt-5-codex",
	} {
		if _, ok := Lookup(model, nil); !ok {
			t.Errorf("no price for %s", model)
		}
	}
	if _, ok := Lookup("claude-unknown-9", nil); ok {
		t.Error("unknown model priced")
	}
}

func TestOverrideWins(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pricing.json")
	if err := os.WriteFile(path, []byte(`{"claude-sonnet-4-5":{"input":9,"cache_read":0.9,"cache_write":9,"output":90}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	override, err := LoadOverride(path)
	if err != nil {
		t.Fatal(err)
	}
	p, ok := Lookup("claude-sonnet-4-5-20250929", override)
	if !ok || p.Input != 9 || p.Output != 90 {
		t.Fatalf("override not applied: %+v %v", p, ok)
	}
	// Missing file is fine.
	if o, err := LoadOverride(filepath.Join(dir, "nope.json")); err != nil || o != nil {
		t.Fatalf("missing override: %v %v", o, err)
	}
	// Unparsable is an error.
	if err := os.WriteFile(path, []byte("nope"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadOverride(path); err == nil {
		t.Fatal("unparsable override accepted")
	}
}

func TestCost(t *testing.T) {
	// Sonnet 4.5: in 3, read 0.3, write 3.75, out 15 per MTok.
	cost, ok := Cost("claude-sonnet-4-5", nil, 707, 28032, 0, 11)
	if !ok {
		t.Fatal("no price")
	}
	want := (707*3 + 28032*0.3 + 0 + 11*15) / 1e6
	if cost != want {
		t.Errorf("cost = %v, want %v", cost, want)
	}
	if _, ok := Cost("mystery", nil, 1, 0, 0, 1); ok {
		t.Error("mystery model priced")
	}
}

func TestCacheWritePremium(t *testing.T) {
	// Claude cache write is 1.25x input: 3.75 vs 3.0 for sonnet.
	costWrite, _ := Cost("claude-sonnet-4-5", nil, 0, 0, 1_000_000, 0)
	costInput, _ := Cost("claude-sonnet-4-5", nil, 1_000_000, 0, 0, 0)
	if costWrite != costInput*1.25 {
		t.Errorf("cache write premium wrong: %v vs %v", costWrite, costInput)
	}
}
