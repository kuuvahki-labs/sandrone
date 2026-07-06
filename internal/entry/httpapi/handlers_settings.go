package httpapi

import (
	"net/http"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

func (s *Server) getRuntimeSettings(w http.ResponseWriter, r *http.Request) {
	result, err := s.rt.Service.GetRuntimeSettings(r.Context())
	writeResult(w, result, err)
}

func (s *Server) putRuntimeSettings(w http.ResponseWriter, r *http.Request) {
	var settings domain.RuntimeSettings
	if !decodeJSON(w, r, &settings) {
		return
	}
	if err := s.rt.Service.PutRuntimeSettings(r.Context(), settings); err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
