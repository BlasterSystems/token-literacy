package server

import (
	"net/http"
	"time"

	"github.com/BlasterSystems/token-literacy/internal/ledger"
)

// exportRows is the shared row query for CSV and JSON export. Export is
// always ascending and unpaginated so files are reproducible.
func (s *Server) exportRows(r *http.Request) ([]ledger.Call, error) {
	f := queryFilter(r)
	f.Asc = true
	f.Limit = 0
	f.Offset = 0
	return s.db.Rows(f)
}

func (s *Server) handleExportCSV(w http.ResponseWriter, r *http.Request) {
	rows, err := s.exportRows(r)
	if err != nil {
		httpError(w, err)
		return
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="tokenledger-export.csv"`)
	_ = ledger.WriteCSV(w, rows)
}

func (s *Server) handleExportJSON(w http.ResponseWriter, r *http.Request) {
	rows, err := s.exportRows(r)
	if err != nil {
		httpError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", `attachment; filename="tokenledger-export.json"`)
	_ = ledger.WriteJSON(w, rows)
}

// formatTime renders epoch ms for the dashboard table.
func formatTime(ms int64) string {
	if ms == 0 {
		return "—"
	}
	return time.UnixMilli(ms).Local().Format("01-02 15:04:05")
}
