package httpapi

import (
	"net/http"

	"github.com/kuuvahki-labs/sandrone/internal/agentcatalog"
	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

func (s *Server) listProcessorSchemas(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, agentcatalog.ProcessorSummary(
		s.rt.Service.Registry().PublicDescriptors(),
	))
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
