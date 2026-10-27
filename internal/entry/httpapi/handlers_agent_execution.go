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
	sources := 0
	if in.Content != nil {
		sources++
	}
	if in.Remote != nil {
		sources++
	}
	if in.RemoteBatch != nil {
		sources++
	}
	if sources != 1 {
		writeError(
			w,
			domain.NewError(domain.CodeInvalidArgument, "exactly one of content, remote or remote_batch is required"),
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
		RemoteBatch:      in.RemoteBatch,
		ParseProcessors:  in.ParseProcessors,
		RenderProcessors: in.RenderProcessors,
		Options:          in.Options,
		Meta:             in.Meta,
	})
	writeAgentRenderResult(w, result, err)
}

func writeAgentRenderResult(w http.ResponseWriter, result *domain.RenderResult, err error) {
	if err != nil {
		if result != nil && len(result.Report.SourceResults) > 0 {
			writeJSON(w, serviceErrorStatus(err), errorResponse{Error: serviceErrorBody(err), Report: &result.Report})
			return
		}
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, agentRenderResponse{
		ContentType: result.ContentType,
		Body:        string(result.Body),
		Report:      result.Report,
	})
}
