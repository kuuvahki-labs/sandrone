package httpapi

import (
	"net/http"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

type uiCapabilityListResponse struct {
	Features []domain.UICapability `json:"features"`
}

func (s *Server) listUICapabilities(w http.ResponseWriter, r *http.Request) {
	result, err := s.rt.Service.ListUICapabilities(r.Context())
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, uiCapabilityListResponse{Features: result.Features})
}
