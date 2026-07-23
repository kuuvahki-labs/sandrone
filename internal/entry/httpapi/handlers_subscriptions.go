package httpapi

import (
	"errors"
	"net/http"
	"strings"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

var errSubscriptionActionNotFound = errors.New("subscription action not found")

func (s *Server) listSubscriptions(w http.ResponseWriter, r *http.Request) {
	result, err := s.rt.Service.ListSubscriptions(r.Context())
	writeResult(w, result, err)
}

func (s *Server) putSubscription(w http.ResponseWriter, r *http.Request) {
	var sub domain.Subscription
	if !decodeJSON(w, r, &sub) {
		return
	}
	if err := validateRequiredPublicResourceName("subscription name", sub.Name); err != nil {
		writeError(w, err, http.StatusBadRequest)
		return
	}
	if err := s.rt.Service.PutSubscription(r.Context(), sub); err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"ok": true})
}

func (s *Server) getSubscription(w http.ResponseWriter, r *http.Request) {
	escapedPath := r.URL.EscapedPath()
	for _, action := range []string{"preview", "traffic", "render"} {
		if strings.HasSuffix(escapedPath, "/"+action) {
			writeError(w, domain.NewError(domain.CodeInvalidArgument, "subscription action requires POST"), http.StatusBadRequest)
			return
		}
	}
	name, err := pathResourceName(escapedPath, "/v1/subscriptions/")
	if err != nil {
		writeError(w, err, http.StatusBadRequest)
		return
	}
	result, err := s.rt.Service.GetSubscription(r.Context(), name)
	writeResult(w, result, err)
}

func (s *Server) subscriptionAction(w http.ResponseWriter, r *http.Request) {
	action, name, err := subscriptionActionName(r)
	if err != nil {
		if errors.Is(err, errSubscriptionActionNotFound) {
			http.NotFound(w, r)
			return
		}
		writeServiceError(w, err)
		return
	}
	switch action {
	case "preview":
		var req subscriptionPreviewRequest
		if !decodeOptionalJSON(w, r, &req) {
			return
		}
		result, err := s.rt.Service.PreviewSubscription(r.Context(), name, requestArgs(r, req.Args))
		writeSubscriptionPreviewResult(w, result, err)
		return
	case "traffic":
		var req subscriptionTrafficRequest
		if !decodeOptionalJSON(w, r, &req) {
			return
		}
		result, err := s.rt.Service.SubscriptionTraffic(r.Context(), domain.SubscriptionTrafficRequest{
			Name:    name,
			Refresh: req.Refresh,
		})
		writeResult(w, result, err)
		return
	case "render":
		var req subscriptionRenderRequest
		if !decodeJSON(w, r, &req) {
			return
		}
		if strings.TrimSpace(req.Format) == "" {
			writeError(w, domain.NewError(domain.CodeInvalidArgument, "subscription render format is required"), http.StatusBadRequest)
			return
		}
		result, err := s.rt.Service.RenderSubscription(
			r.Context(),
			name,
			req.Format,
			domain.RequestInfo{Args: requestArgs(r, req.Args)},
		)
		writeAgentRenderResult(w, result, err)
		return
	default:
		writeError(w, domain.NewError(domain.CodeInvalidArgument, "unsupported subscription action"), http.StatusBadRequest)
	}
}

func writeSubscriptionPreviewResult(w http.ResponseWriter, result *domain.SubscriptionPreviewResult, err error) {
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, subscriptionPreviewResponse{
		SubscriptionName: result.SubscriptionName,
		Type:             result.Type,
		Format:           result.Format,
		BeforeCount:      result.BeforeCount,
		AfterCount:       result.AfterCount,
		StatusCounts:     result.StatusCounts,
		Nodes:            result.Nodes,
		Warnings:         reportWarnings(result.Report),
	})
}

func (s *Server) deleteSubscription(w http.ResponseWriter, r *http.Request) {
	s.deleteResource(w, r, "/v1/subscriptions/", s.rt.Service.DeleteSubscription)
}
