package httpapi

import (
	"net/http"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

func (s *Server) inspectNode(w http.ResponseWriter, r *http.Request) {
	var req domain.NodeInspectRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	result, err := s.rt.Service.InspectNode(r.Context(), req)
	writeResult(w, result, err)
}
