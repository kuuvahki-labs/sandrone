package httpapi

import (
	"net/http"
	"net/url"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

type formatCapabilityListResponse struct {
	Items []formatCapabilitySummaryResponse `json:"items"`
}

type formatCapabilitySummaryResponse struct {
	domain.FormatCapabilitySummary
	Href string `json:"href"`
}

func (s *Server) listFormatCapabilities(w http.ResponseWriter, r *http.Request) {
	result, err := s.rt.Service.ListFormatCapabilities(r.Context())
	if err != nil {
		writeServiceError(w, err)
		return
	}
	items := make([]formatCapabilitySummaryResponse, len(result.Items))
	for index, item := range result.Items {
		items[index] = formatCapabilitySummaryResponse{
			FormatCapabilitySummary: item,
			Href:                    "/v1/capabilities/formats/" + url.PathEscape(string(item.Direction)) + "/" + url.PathEscape(item.Format),
		}
	}
	writeJSON(w, http.StatusOK, formatCapabilityListResponse{Items: items})
}

func (s *Server) getFormatCapability(w http.ResponseWriter, r *http.Request) {
	format := r.PathValue("format")
	if err := validateRequiredPublicResourceName("capability format", format); err != nil {
		writeServiceError(w, err)
		return
	}
	result, err := s.rt.Service.GetFormatCapability(r.Context(), domain.FormatCapabilityRequest{
		Direction: domain.CapabilityDirection(r.PathValue("direction")),
		Format:    format,
	})
	writeResult(w, result, err)
}
