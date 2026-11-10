package pricing

import "testing"

func TestTableHasEntries(t *testing.T) {
	m := Table()
	if len(m) == 0 {
		t.Fatal("embedded table empty")
	}
	for _, prefix := range []string{"claude-sonnet-4-5", "gpt-5"} {
		p, ok := m[prefix]
		if !ok || p.Input <= 0 || p.Output <= 0 {
			t.Errorf("missing/invalid %s: %+v", prefix, p)
		}
	}
}
