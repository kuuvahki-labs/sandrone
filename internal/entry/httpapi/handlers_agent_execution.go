package httpapi

import (
	"net/http"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

func (s *Server) agentConvert(w http.ResponseWriter, r *http.Request) {
	var in convertRequest
	if !decodeJSON(w, r, &in) {
		return
	}
	if (in.Content == nil) == (in.Remote == nil) {
		writeError(
			w,
			domain.NewError(domain.CodeInvalidArgument, "exactly one of content or remote is required"),
			http.StatusBadRequest,
		)
		return
	}
	var content []byte
	if in.Content != nil {
		content = []byte(*in.Content)
	}
	result, err := s.rt.Service.Convert(r.Context(), domain.ConvertRequest{
		FromFormat:       in.FromFormat,
		ToFormat:         in.ToFormat,
		Content:          content,
		Remote:           in.Remote,
		ParseProcessors:  in.ParseProcessors,
		RenderProcessors: in.RenderProcessors,
		Options:          in.Options,
		Meta:             in.Meta,
	})
	writeAgentRenderResult(w, result, err)
}

func writeAgentRenderResult(w http.ResponseWriter, result *domain.RenderResult, err error) {
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, agentRenderResponse{
		ContentType: result.ContentType,
		Body:        string(result.Body),
		Report:      result.Report,
	})
}
