package otlp

import (
	"bytes"
	"compress/gzip"
	"strings"
	"testing"
)

func TestParseKindAndString(t *testing.T) {
	for path, want := range map[string]Kind{"/v1/traces": KindTraces, "/v1/metrics": KindMetrics, "/v1/logs": KindLogs} {
		got, ok := ParseKind(path)
		if !ok || got != want {
			t.Errorf("ParseKind(%s) = %v, %v", path, got, ok)
		}
	}
	if _, ok := ParseKind("/v1/nope"); ok {
		t.Error("unknown path accepted")
	}
	if KindTraces.String() != "traces" || KindMetrics.String() != "metrics" || KindLogs.String() != "logs" || Kind(9).String() != "unknown" {
		t.Error("String() mismatch")
	}
}

func TestReadBodyPlainAndGzip(t *testing.T) {
	payload := []byte(`{"hello":"world"}`)
	got, err := ReadBody(bytes.NewReader(payload), "")
	if err != nil || !bytes.Equal(got, payload) {
		t.Fatalf("plain: %v %s", err, got)
	}
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	zw.Write(payload)
	zw.Close()
	got, err = ReadBody(bytes.NewReader(buf.Bytes()), "gzip")
	if err != nil || !bytes.Equal(got, payload) {
		t.Fatalf("gzip: %v %s", err, got)
	}
	if _, err := ReadBody(bytes.NewReader([]byte("not gzip")), "gzip"); err == nil {
		t.Fatal("invalid gzip accepted")
	}
}

func TestAttrValueKinds(t *testing.T) {
	// kvlist, array, bool, double, and int-as-string round-trip through attrMap.
	body := []byte(`{"resourceSpans":[{"scopeSpans":[{"spans":[
		{"name":"claude_code.llm_request","startTimeUnixNano":"1",
		 "attributes":[
		   {"key":"kv","value":{"kvlistValue":{"values":[{"key":"inner","value":{"intValue":"7"}}]}}},
		   {"key":"arr","value":{"arrayValue":{"values":[{"stringValue":"a"},{"intValue":2}]}}},
		   {"key":"flag","value":{"boolValue":true}},
		   {"key":"ratio","value":{"doubleValue":1.5}},
		   {"key":"big","value":{"intValue":"9007199254740993"}},
		   {"key":"request_id","value":{"stringValue":"r1"}},
		   {"key":"input_tokens","value":{"intValue":"10"}},
		   {"key":"output_tokens","value":{"intValue":"3"}}
		 ]}
	]}]}]}`)
	calls, _, err := SpanCalls(body, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(calls) != 1 {
		t.Fatalf("calls = %d", len(calls))
	}
}

func TestAsIntForms(t *testing.T) {
	m := map[string]any{
		"i":   int64(5),
		"f":   float64(2.9),
		"s":   "42",
		"bad": "not-a-number",
		"b":   true,
	}
	if v, ok := asInt(m, "i"); !ok || v != 5 {
		t.Errorf("int64: %v %v", v, ok)
	}
	if v, ok := asInt(m, "f"); !ok || v != 2 {
		t.Errorf("float: %v %v", v, ok)
	}
	if v, ok := asInt(m, "s"); !ok || v != 42 {
		t.Errorf("string: %v %v", v, ok)
	}
	if _, ok := asInt(m, "bad"); ok {
		t.Error("bad string accepted")
	}
	if _, ok := asInt(m, "b"); ok {
		t.Error("bool accepted as int")
	}
	if _, ok := asInt(m, "missing"); ok {
		t.Error("missing accepted")
	}
	if v, ok := asBool(m, "b"); !ok || !v {
		t.Error("asBool failed")
	}
	if _, ok := asBool(m, "i"); ok {
		t.Error("asBool accepted int")
	}
}

func TestReadBodyLargeInput(t *testing.T) {
	// Inputs beyond the cap are truncated, not OOM.
	big := strings.Repeat("x", maxBodyBytes+1)
	got, err := ReadBody(strings.NewReader(big), "")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != maxBodyBytes {
		t.Fatalf("truncated to %d", len(got))
	}
}
