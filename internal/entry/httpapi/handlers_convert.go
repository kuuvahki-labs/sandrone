package httpapi

import (
	"net/http"
	"strings"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

const maxPublicConvertContentBytes = 64 << 10

var publicConvertQueryKeys = map[string]bool{
	"content":     true,
	"from_format": true,
	"response":    true,
	"to_format":   true,
	"url":         true,
}

func (s *Server) publicConvert(w http.ResponseWriter, r *http.Request) {
	setPublicConvertHeaders(w)
	query := r.URL.Query()
	for key, values := range query {
		if !publicConvertQueryKeys[key] {
			writeError(w, domain.NewError(domain.CodeInvalidArgument, "unsupported convert query parameter "+key), http.StatusBadRequest)
			return
		}
		if len(values) != 1 {
			writeError(w, domain.NewError(domain.CodeInvalidArgument, "convert query parameter "+key+" must appear once"), http.StatusBadRequest)
			return
		}
	}
	content := query.Get("content")
	remoteURL := strings.TrimSpace(query.Get("url"))
	if (strings.TrimSpace(content) == "") == (remoteURL == "") {
		writeError(w, domain.NewError(domain.CodeInvalidArgument, "convert requires exactly one of content or url"), http.StatusBadRequest)
		return
	}
	if len(content) > maxPublicConvertContentBytes {
		writeError(w, domain.NewError(domain.CodeInvalidArgument, "convert content is too large"), http.StatusBadRequest)
		return
	}
	toFormat := strings.TrimSpace(query.Get("to_format"))
	if toFormat == "" {
		writeError(w, domain.NewError(domain.CodeInvalidArgument, "convert to_format is required"), http.StatusBadRequest)
		return
	}
	fromFormat := strings.TrimSpace(query.Get("from_format"))
	var remote *domain.RemoteInput
	if remoteURL != "" {
		remote = &domain.RemoteInput{URL: remoteURL}
	} else if fromFormat == "" {
		fromFormat = "uri-list"
	}
	result, err := s.rt.Service.ConvertPublic(r.Context(), domain.ConvertRequest{
		FromFormat: fromFormat,
		ToFormat:   toFormat,
		Content:    []byte(content),
		Remote:     remote,
	})
	if err != nil {
		writeServiceError(w, err)
		return
	}
	if strings.EqualFold(query.Get("response"), "json") {
		writeJSON(w, http.StatusOK, renderResponse{
			ContentType: result.ContentType,
			Body:        string(result.Body),
			Warnings:    reportWarnings(result.Report),
		})
		return
	}
	contentType := result.ContentType
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(result.Body)
}

func setPublicConvertHeaders(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("X-Content-Type-Options", "nosniff")
}

func (s *Server) validate(w http.ResponseWriter, r *http.Request) {
	var req validateRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := validateOptionalPublicResourceName("file name", req.File); err != nil {
		writeError(w, err, http.StatusBadRequest)
		return
	}
	if req.File != "" || req.Spec != nil {
		result, err := s.rt.Service.ValidateFile(r.Context(), domain.FileRequest{
			Name:   req.File,
			Spec:   req.Spec,
			Target: req.Target,
		})
		writeValidateResult(w, result, err)
		return
	}
	if req.Format != "" || req.Remote != nil {
		result, err := s.rt.Service.ValidateNodes(r.Context(), domain.ParseRequest{
			Format:     req.Format,
			Content:    []byte(req.Content),
			Remote:     req.Remote,
			Target:     req.Target,
			Processors: req.Processors,
		})
		writeValidateResult(w, result, err)
		return
	}
	writeError(w, domain.NewError(domain.CodeInvalidArgument, "validate requires file/spec or format/content"), http.StatusBadRequest)
}

func (s *Server) inspect(w http.ResponseWriter, r *http.Request) {
	result, err := s.rt.Service.Inspect(r.Context())
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, inspectResponse{
		InspectResult: *result,
		Catalogs: inspectCatalogs{
			Formats: "/v1/capabilities/formats", Processors: "/v1/schemas/processors",
			Schemas: "/v1/schemas", FileKinds: "/v1/schemas/file-kinds",
		},
	})
}
