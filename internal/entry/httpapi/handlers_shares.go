package httpapi

import (
	"net/http"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

func (s *Server) listShares(w http.ResponseWriter, r *http.Request) {
	result, err := s.rt.Service.ListShares(r.Context())
	writeResult(w, result, err)
}

func (s *Server) createShare(w http.ResponseWriter, r *http.Request) {
	var req domain.ShareCreateRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	for _, candidate := range []struct {
		label string
		name  string
	}{
		{label: "share id", name: req.ID},
		{label: "share name", name: req.Name},
		{label: "share target name", name: req.TargetName},
	} {
		if err := validateOptionalPublicResourceName(candidate.label, candidate.name); err != nil {
			writeError(w, err, http.StatusBadRequest)
			return
		}
	}
	share, err := s.rt.Service.CreateShare(r.Context(), req)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"share": share})
}

func (s *Server) getShare(w http.ResponseWriter, r *http.Request) {
	id, err := pathResourceName(r.URL.EscapedPath(), "/v1/shares/")
	if err != nil {
		writeError(w, err, http.StatusBadRequest)
		return
	}
	result, err := s.rt.Service.GetShare(r.Context(), id)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"share": result})
}

func (s *Server) deleteShare(w http.ResponseWriter, r *http.Request) {
	s.deleteResource(w, r, "/v1/shares/", s.rt.Service.DeleteShare)
}

func (s *Server) publicShare(w http.ResponseWriter, r *http.Request) {
	id, err := pathResourceName(r.URL.EscapedPath(), "/s/")
	if err != nil {
		writeError(w, err, http.StatusBadRequest)
		return
	}
	result, err := s.rt.Service.RenderShare(r.Context(), domain.ShareRenderRequest{
		ID:     id,
		Format: r.URL.Query().Get("format"),
		Request: domain.RequestInfo{
			Args: queryArgs(r.URL.Query()),
		},
	})
	if err != nil {
		writeServiceError(w, err)
		return
	}
	for key, value := range result.Headers {
		w.Header().Set(key, value)
	}
	contentType := result.ContentType
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	if w.Header().Get("Content-Type") == "" {
		w.Header().Set("Content-Type", contentType)
	}
	status := result.Status
	if status == 0 {
		status = http.StatusOK
	}
	w.WriteHeader(status)
	_, _ = w.Write(result.Body)
}
