package httpapi

import (
	"errors"
	"net/http"
	"strconv"
)

var errBackupExportFailed = errors.New("backup export failed")

func (s *Server) getBackup(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	result, err := s.rt.Service.ExportBackup(r.Context())
	if err != nil {
		writeError(w, errBackupExportFailed, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", "attachment; filename="+strconv.Quote(result.Filename))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(result.Body)
}

func (s *Server) restoreBackup(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	body, ok := readBackupBody(w, r)
	if !ok {
		return
	}
	if err := s.rt.Service.RestoreBackup(r.Context(), body); err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
