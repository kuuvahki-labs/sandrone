package httpapi

import (
	"errors"
	"net/http"
	"strings"

	"github.com/kuuvahki-labs/sandrone/internal/service"
)

func (s *Server) listRuleSetCatalog(w http.ResponseWriter, r *http.Request) {
	result, err := s.rt.Service.ListRuleSetCatalog(r.Context(), strings.TrimSpace(r.URL.Query().Get("target")))
	if errors.Is(err, service.ErrRuleSetCatalogUnavailable) {
		writeError(w, err, http.StatusServiceUnavailable)
		return
	}
	writeResult(w, result, err)
}
