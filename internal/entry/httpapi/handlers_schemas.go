package httpapi

import (
	"net/http"
	"net/url"

	"github.com/kuuvahki-labs/sandrone/internal/agentcatalog"
	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

type schemaSummaryResponse struct {
	Schemas []schemaSummaryItem `json:"schemas"`
}

type schemaSummaryItem struct {
	agentcatalog.SchemaSummaryEntry
	Href string `json:"href"`
}

type processorSummaryResponse struct {
	Processors []processorSummaryItem `json:"processors"`
}

type processorSummaryItem struct {
	agentcatalog.ProcessorSummaryEntry
	Href string `json:"href"`
}

type fileKindSummaryResponse struct {
	FileKinds []fileKindSummaryItem `json:"file_kinds"`
}

type fileKindSummaryItem struct {
	agentcatalog.FileKindSummaryEntry
	Href string `json:"href"`
}

func (s *Server) listSchemas(w http.ResponseWriter, _ *http.Request) {
	summary := agentcatalog.SchemaSummary()
	items := make([]schemaSummaryItem, len(summary.Schemas))
	paths := map[string]string{
		"processors": "/v1/schemas/processors", "file_kinds": "/v1/schemas/file-kinds",
		"script_api_v1": "/v1/schemas/script-api/v1", "subscription": "/v1/schemas/subscription",
		"file_spec": "/v1/schemas/file-spec",
	}
	for index, item := range summary.Schemas {
		items[index] = schemaSummaryItem{SchemaSummaryEntry: item, Href: paths[item.Name]}
	}
	writeJSON(w, http.StatusOK, schemaSummaryResponse{Schemas: items})
}

func (s *Server) listProcessorSchemas(w http.ResponseWriter, _ *http.Request) {
	summary := agentcatalog.ProcessorSummary(s.rt.Service.Registry().PublicDescriptors())
	items := make([]processorSummaryItem, len(summary.Processors))
	for index, item := range summary.Processors {
		items[index] = processorSummaryItem{
			ProcessorSummaryEntry: item,
			Href:                  "/v1/schemas/processors/" + url.PathEscape(string(item.Stage)) + "/" + url.PathEscape(item.Type),
		}
	}
	writeJSON(w, http.StatusOK, processorSummaryResponse{Processors: items})
}

func (s *Server) listFileKindSchemas(w http.ResponseWriter, _ *http.Request) {
	summary := agentcatalog.FileKindSummary(s.rt.Service.FileKindCapabilities())
	items := make([]fileKindSummaryItem, len(summary.FileKinds))
	for index, item := range summary.FileKinds {
		items[index] = fileKindSummaryItem{
			FileKindSummaryEntry: item,
			Href:                 "/v1/schemas/file-kinds/" + url.PathEscape(string(item.Kind)),
		}
	}
	writeJSON(w, http.StatusOK, fileKindSummaryResponse{FileKinds: items})
}

func (s *Server) getProcessorSchema(w http.ResponseWriter, r *http.Request) {
	stage := domain.Stage(r.PathValue("stage"))
	if stage != domain.StageNodes && stage != domain.StageFile {
		writeServiceError(w, domain.NewError(
			domain.CodeInvalidArgument,
			"processor stage must be nodes or file",
		))
		return
	}

	processorType := r.PathValue("type")
	if err := validateRequiredPublicResourceName("processor type", processorType); err != nil {
		writeServiceError(w, err)
		return
	}
	for _, descriptor := range s.rt.Service.Registry().PublicDescriptors() {
		if descriptor.Stage == stage && descriptor.Type == processorType {
			document, err := agentcatalog.ProcessorDetail(descriptor)
			writeResult(w, document, err)
			return
		}
	}
	writeServiceError(w, domain.NewError(domain.CodeInvalidArgument, "schema not found"))
}

func (s *Server) getFileKindSchema(w http.ResponseWriter, r *http.Request) {
	kindName := r.PathValue("kind")
	if err := validateRequiredPublicResourceName("file kind", kindName); err != nil {
		writeServiceError(w, err)
		return
	}
	kind := domain.FileKind(kindName)
	for _, capability := range s.rt.Service.FileKindCapabilities() {
		if capability.Kind == kind {
			document, err := agentcatalog.FileKindDetail(capability)
			writeResult(w, document, err)
			return
		}
	}
	writeServiceError(w, domain.NewError(domain.CodeInvalidArgument, "schema not found"))
}

func (s *Server) getScriptAPISchema(w http.ResponseWriter, _ *http.Request) {
	document, err := agentcatalog.ScriptAPI()
	writeResult(w, document, err)
}

func (s *Server) getSubscriptionSchema(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, agentcatalog.SubscriptionSchema())
}

func (s *Server) getFileSpecSchema(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, agentcatalog.FileSpecSchema(true))
}
