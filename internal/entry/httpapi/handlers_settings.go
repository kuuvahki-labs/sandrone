package httpapi

import (
	"net/http"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

func (s *Server) getSettings(w http.ResponseWriter, r *http.Request) {
	result, err := s.rt.Service.GetSettings(r.Context())
	writeResult(w, result, err)
}

func (s *Server) putSettings(w http.ResponseWriter, r *http.Request) {
	var update domain.SettingsUpdate
	if !decodeJSON(w, r, &update) {
		return
	}
	result, err := s.rt.Service.PutSettings(r.Context(), update)
	writeResult(w, result, err)
}

func (s *Server) getScheduledRefreshStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.rt.Service.ScheduledRefreshStatus(r.Context()))
}

func (s *Server) runScheduledRefreshNow(w http.ResponseWriter, r *http.Request) {
	if err := s.rt.Service.RunScheduledRefreshNow(r.Context()); err != nil {
		if domain.IsCode(err, domain.CodeNotImplemented) {
			writeError(w, err, http.StatusServiceUnavailable)
			return
		}
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]bool{"accepted": true})
}
