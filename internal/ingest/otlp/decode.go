// Package otlp decodes OTLP/HTTP JSON payloads (Claude Code telemetry) and
// normalizes them into ledger rows. Only usage metadata is read; message
// bodies, prompts, and tool output are never touched.
package otlp

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// Kind identifies the OTLP signal carried by a request body.
type Kind int

const (
	KindTraces Kind = iota
	KindMetrics
	KindLogs
)

// ParseKind maps an endpoint path to its signal kind.
func ParseKind(path string) (Kind, bool) {
	switch path {
	case "/v1/traces":
		return KindTraces, true
	case "/v1/metrics":
		return KindMetrics, true
	case "/v1/logs":
		return KindLogs, true
	}
	return 0, false
}

// String renders the kind for logs.
func (k Kind) String() string {
	switch k {
	case KindTraces:
		return "traces"
	case KindMetrics:
		return "metrics"
	case KindLogs:
		return "logs"
	default:
		return "unknown"
	}
}

// ReadBody decodes a request body, transparently handling gzip
// (Content-Encoding: gzip) per the OTLP/HTTP specification.
func ReadBody(r io.Reader, contentEncoding string) ([]byte, error) {
	if strings.EqualFold(contentEncoding, "gzip") {
		zr, err := gzip.NewReader(r)
		if err != nil {
			return nil, fmt.Errorf("gzip body: %w", err)
		}
		defer zr.Close()
		var buf bytes.Buffer
		if _, err := buf.ReadFrom(io.LimitReader(zr, maxBodyBytes)); err != nil {
			return nil, fmt.Errorf("read gzip body: %w", err)
		}
		return buf.Bytes(), nil
	}
	buf := bytes.NewBuffer(nil)
	if _, err := buf.ReadFrom(io.LimitReader(r, maxBodyBytes)); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

const maxBodyBytes = 64 << 20 // 64 MiB OTLP limit

// IntOrString decodes OTLP JSON integer fields, which may appear as JSON
// numbers or as strings (the proto3 JSON mapping emits 64-bit ints as
// strings).
type IntOrString int64

// UnmarshalJSON implements json.Unmarshaler.
func (i *IntOrString) UnmarshalJSON(b []byte) error {
	s := strings.TrimSpace(string(b))
	if s == "null" {
		*i = 0
		return nil
	}
	if len(s) > 0 && s[0] == '"' {
		var str string
		if err := json.Unmarshal(b, &str); err != nil {
			return err
		}
		v, err := strconv.ParseInt(str, 10, 64)
		if err != nil {
			return err
		}
		*i = IntOrString(v)
		return nil
	}
	var v int64
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	*i = IntOrString(v)
	return nil
}

// Attr is one OTLP key-value attribute.
type Attr struct {
	Key   string     `json:"key"`
	Value *AttrValue `json:"value"`
}

// AttrValue is the protobuf AnyValue union from the OTLP JSON mapping.
type AttrValue struct {
	StringValue *string      `json:"stringValue"`
	IntValue    *IntOrString `json:"intValue"`
	DoubleValue *float64     `json:"doubleValue"`
	BoolValue   *bool        `json:"boolValue"`
	BytesValue  *string      `json:"bytesValue"`
	KvlistValue *AttrList    `json:"kvlistValue"`
	ArrayValue  *struct {
		Values []AttrValue `json:"values"`
	} `json:"arrayValue"`
}

// AttrList is a repeated key-value list.
type AttrList struct {
	Values []Attr `json:"values"`
}

func (a *Attr) rawValue() any {
	if a.Value == nil {
		return nil
	}
	v := a.Value
	switch {
	case v.StringValue != nil:
		return *v.StringValue
	case v.IntValue != nil:
		return int64(*v.IntValue)
	case v.DoubleValue != nil:
		return *v.DoubleValue
	case v.BoolValue != nil:
		return *v.BoolValue
	case v.KvlistValue != nil:
		m := make(map[string]any, len(v.KvlistValue.Values))
		for _, kv := range v.KvlistValue.Values {
			m[kv.Key] = kv.rawValue()
		}
		return m
	case v.ArrayValue != nil:
		out := make([]any, 0, len(v.ArrayValue.Values))
		for _, item := range v.ArrayValue.Values {
			out = append(out, (&Attr{Value: &item}).rawValue())
		}
		return out
	default:
		return nil
	}
}

// attrMap flattens an attribute list into a plain map.
func attrMap(list []Attr) map[string]any {
	m := make(map[string]any, len(list))
	for _, a := range list {
		m[a.Key] = a.rawValue()
	}
	return m
}

func asString(m map[string]any, key string) (string, bool) {
	v, ok := m[key]
	if !ok {
		return "", false
	}
	s, ok := v.(string)
	return s, ok && s != ""
}

func asInt(m map[string]any, key string) (int64, bool) {
	v, ok := m[key]
	if !ok {
		return 0, false
	}
	switch n := v.(type) {
	case int64:
		return n, true
	case float64:
		return int64(n), true
	case string:
		iv, err := strconv.ParseInt(n, 10, 64)
		if err != nil {
			return 0, false
		}
		return iv, true
	default:
		return 0, false
	}
}

func asBool(m map[string]any, key string) (bool, bool) {
	v, ok := m[key]
	if !ok {
		return false, false
	}
	b, ok := v.(bool)
	return b, ok
}

// parseNanoMs converts a decimal Unix-nanosecond string to epoch milliseconds.
func parseNanoMs(s string) (int64, bool) {
	if s == "" {
		return 0, false
	}
	ns, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, false
	}
	return ns / 1_000_000, true
}

// ---- Traces ----

type traceRequest struct {
	ResourceSpans []resourceSpans `json:"resourceSpans"`
}

type resourceSpans struct {
	Resource   *resource    `json:"resource"`
	ScopeSpans []scopeSpans `json:"scopeSpans"`
}

type resource struct {
	Attributes []Attr `json:"attributes"`
}

type scopeSpans struct {
	Scope *scope `json:"scope"`
	Spans []span `json:"spans"`
}

type scope struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type span struct {
	Name              string `json:"name"`
	StartTimeUnixNano string `json:"startTimeUnixNano"`
	EndTimeUnixNano   string `json:"endTimeUnixNano"`
	Attributes        []Attr `json:"attributes"`
}

// ---- Metrics ----

type metricRequest struct {
	ResourceMetrics []resourceMetrics `json:"resourceMetrics"`
}

type resourceMetrics struct {
	Resource     *resource      `json:"resource"`
	ScopeMetrics []scopeMetrics `json:"scopeMetrics"`
}

type scopeMetrics struct {
	Scope   *scope   `json:"scope"`
	Metrics []metric `json:"metrics"`
}

type metric struct {
	Name  string `json:"name"`
	Gauge *gauge `json:"gauge"`
	Sum   *sum   `json:"sum"`
}

type gauge struct {
	DataPoints []dataPoint `json:"dataPoints"`
}

type sum struct {
	DataPoints             []dataPoint `json:"dataPoints"`
	AggregationTemporality int         `json:"aggregationTemporality"`
}

// AggregationTemporality values from the OTLP proto.
const (
	temporalityUnspecified = 0
	temporalityDelta       = 2
)

type dataPoint struct {
	Attributes        []Attr       `json:"attributes"`
	TimeUnixNano      string       `json:"timeUnixNano"`
	StartTimeUnixNano string       `json:"startTimeUnixNano"`
	AsInt             *IntOrString `json:"asInt"`
	AsDouble          *float64     `json:"asDouble"`
}

func (d *dataPoint) value() (int64, bool) {
	switch {
	case d.AsInt != nil:
		return int64(*d.AsInt), true
	case d.AsDouble != nil:
		return int64(*d.AsDouble), true
	default:
		return 0, false
	}
}

// ---- Logs (accepted and discarded; never stored) ----

type logsRequest struct {
	ResourceLogs []json.RawMessage `json:"resourceLogs"`
}
