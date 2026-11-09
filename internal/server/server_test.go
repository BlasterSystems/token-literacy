package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHealth(t *testing.T) {
	handler := NewHandler(Info{
		Version: "test-version",
		Started: time.Now(),
		DBPath:  "/tmp/ledger.db",
	})

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("content-type = %q, want application/json", got)
	}

	var body struct {
		Status  string `json:"status"`
		Version string `json:"version"`
		DB      string `json:"db"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Status != "ok" {
		t.Errorf("status = %q, want ok", body.Status)
	}
	if body.Version != "test-version" {
		t.Errorf("version = %q, want test-version", body.Version)
	}
	if body.DB != "/tmp/ledger.db" {
		t.Errorf("db = %q, want /tmp/ledger.db", body.DB)
	}
}

func TestRootAndNotFound(t *testing.T) {
	handler := NewHandler(Info{Version: "test", Started: time.Now()})

	for _, tc := range []struct {
		path string
		want int
	}{
		{path: "/", want: http.StatusOK},
		{path: "/missing", want: http.StatusNotFound},
	} {
		req := httptest.NewRequest(http.MethodGet, tc.path, nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != tc.want {
			t.Errorf("%s status = %d, want %d", tc.path, rec.Code, tc.want)
		}
	}
}
