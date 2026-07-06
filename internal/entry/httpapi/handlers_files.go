package httpapi

import (
	"net/http"
	"strings"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

func (s *Server) listFiles(w http.ResponseWriter, r *http.Request) {
	result, err := s.rt.Service.ListFiles(r.Context())
	writeResult(w, result, err)
}

func (s *Server) putFile(w http.ResponseWriter, r *http.Request) {
	var file domain.FileSpec
	if !decodeJSON(w, r, &file) {
		return
	}
	if err := validateRequiredPublicResourceName("file name", file.Name); err != nil {
		writeError(w, err, http.StatusBadRequest)
		return
	}
	if err := s.rt.Service.PutFile(r.Context(), file); err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"ok": true})
}

func (s *Server) getFile(w http.ResponseWriter, r *http.Request) {
	name, err := pathResourceName(r.URL.EscapedPath(), "/v1/files/")
	if err != nil {
		writeError(w, err, http.StatusBadRequest)
		return
	}
	q := r.URL.Query()
	switch strings.ToLower(strings.TrimSpace(q.Get("mode"))) {
	case "", "render":
	case "spec":
		result, err := s.rt.Service.GetFileSpec(r.Context(), name)
		writeResult(w, result, err)
		return
	case "source":
		result, err := s.rt.Service.GetFileSource(r.Context(), name)
		if err != nil {
			writeServiceError(w, err)
			return
		}
		if strings.EqualFold(q.Get("response"), "json") {
			writeJSON(w, http.StatusOK, sourceResponse{ContentType: result.MediaType, Body: string(result.Content)})
			return
		}
		contentType := result.MediaType
		if contentType == "" {
			contentType = "application/octet-stream"
		}
		w.Header().Set("Content-Type", contentType)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(result.Content) //nolint:gosec // Source bodies use service-controlled non-HTML media types and disable MIME sniffing above.
		return
	default:
		writeError(w, domain.NewError(domain.CodeInvalidArgument, "unsupported file mode"), http.StatusBadRequest)
		return
	}
	result, err := s.rt.Service.GetFile(r.Context(), domain.FileRequest{
		Name:    name,
		Request: domain.RequestInfo{Args: queryArgs(q)},
	})
	if err != nil {
		writeServiceError(w, err)
		return
	}
	if strings.EqualFold(q.Get("response"), "json") {
		writeJSON(w, http.StatusOK, renderResponse{
			ContentType: fileContentType(result),
			Body:        string(result.Content),
			Response:    result.Response,
			Warnings:    reportWarnings(result.Report),
		})
		return
	}
	writeFileContent(w, result)
}

func (s *Server) deleteFile(w http.ResponseWriter, r *http.Request) {
	s.deleteResource(w, r, "/v1/files/", s.rt.Service.DeleteFile)
}
