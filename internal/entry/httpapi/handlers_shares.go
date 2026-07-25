package httpapi

import (
	"net/http"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

func (s *Server) listShares(w http.ResponseWriter, r *http.Request) {
	result, err := s.rt.Service.ListShares(r.Context())
	if err != nil {
		writeResult(w, nil, err)
		return
	}
	shares := make([]shareResponse, 0, len(result.Shares))
	for _, share := range result.Shares {
		presentation := result.Presentations[share.ID]
		shares = append(shares, shareResponse{
			Share:           share,
			PublicFilename:  presentation.PublicFilename,
			FormatFilenames: presentation.FormatFilenames,
		})
	}
	writeResult(w, shareListResponse{Shares: shares}, nil)
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
	result, err := s.rt.Service.CreateShare(r.Context(), req)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"share": shareResponse{
			Share:           result.Share,
			PublicFilename:  result.Presentation.PublicFilename,
			FormatFilenames: result.Presentation.FormatFilenames,
		},
	})
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
	id, filename, err := publicSharePath(r.URL.EscapedPath())
	if err != nil {
		writeError(w, err, http.StatusBadRequest)
		return
	}
	result, err := s.rt.Service.RenderShare(r.Context(), domain.ShareRenderRequest{
		ID:                id,
		Format:            r.URL.Query().Get("format"),
		PresentedFilename: filename,
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
