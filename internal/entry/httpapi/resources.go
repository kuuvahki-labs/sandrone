package httpapi

import (
	"context"
	"net/http"
	"net/url"
	"strings"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

func (s *Server) deleteResource(w http.ResponseWriter, r *http.Request, prefix string, delete func(context.Context, string) error) {
	name, err := deletePathResourceName(r, prefix)
	if err != nil {
		writeError(w, err, http.StatusBadRequest)
		return
	}
	if err := delete(r.Context(), name); err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func queryArgs(q url.Values) map[string]string {
	args := map[string]string{}
	for key, values := range q {
		name, ok := strings.CutPrefix(key, "arg.")
		if !ok || name == "" || len(values) == 0 {
			continue
		}
		args[name] = values[len(values)-1]
	}
	if len(args) == 0 {
		return nil
	}
	return args
}

func requestArgs(r *http.Request, bodyArgs map[string]string) map[string]string {
	args := queryArgs(r.URL.Query())
	if len(bodyArgs) == 0 {
		return args
	}
	if args == nil {
		args = map[string]string{}
	}
	for key, value := range bodyArgs {
		args[key] = value
	}
	return args
}

func pathResourceName(rawPath string, prefix string) (string, error) {
	raw := strings.TrimPrefix(rawPath, prefix)
	if raw == rawPath || raw == "" {
		return "", domain.NewError(domain.CodeInvalidArgument, "resource name is required")
	}
	name, err := url.PathUnescape(raw)
	if err != nil {
		return "", domain.WrapError(domain.CodeInvalidArgument, "invalid resource name", err)
	}
	if err := validateRequiredPublicResourceName("resource name", name); err != nil {
		return "", err
	}
	return name, nil
}

func validateDeleteResourcePath(r *http.Request) error {
	if r.Method != http.MethodDelete {
		return nil
	}
	path := r.URL.EscapedPath()
	for _, exact := range []string{"/v1/subscriptions", "/v1/files", "/v1/shares"} {
		if path == exact {
			return domain.NewError(domain.CodeInvalidArgument, "resource name is required")
		}
	}
	for _, prefix := range []string{"/v1/subscriptions/", "/v1/files/", "/v1/shares/"} {
		if strings.HasPrefix(path, prefix) {
			_, err := deletePathResourceName(r, prefix)
			return err
		}
	}
	return nil
}

func deletePathResourceName(r *http.Request, prefix string) (string, error) {
	escapedPath := r.URL.EscapedPath()
	raw := strings.TrimPrefix(escapedPath, prefix)
	if raw == escapedPath || raw == "" {
		return "", domain.NewError(domain.CodeInvalidArgument, "resource name is required")
	}
	name, err := url.PathUnescape(raw)
	if err != nil {
		return "", domain.WrapError(domain.CodeInvalidArgument, "invalid resource name", err)
	}
	if err := validateDeleteResourceName(name); err != nil {
		return "", err
	}
	return name, nil
}

func validateDeleteResourceName(name string) error {
	return validateRequiredPublicResourceName("resource name", name)
}

func validateOptionalPublicResourceName(label string, name string) error {
	if strings.TrimSpace(name) == "" {
		return nil
	}
	return validateRequiredPublicResourceName(label, name)
}

func validateRequiredPublicResourceName(label string, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return domain.NewError(domain.CodeInvalidArgument, label+" is required")
	}
	if strings.Contains(name, "/") || strings.Contains(name, "\\") || name == "." || name == ".." {
		return domain.NewError(domain.CodeInvalidArgument, label+" must be a single path segment")
	}
	return nil
}

func subscriptionActionName(r *http.Request) (string, string, error) {
	const prefix = "/v1/subscriptions/"
	escapedPath := r.URL.EscapedPath()
	if !strings.HasPrefix(escapedPath, prefix) {
		return "", "", domain.NewError(domain.CodeInvalidArgument, "subscription action path is invalid")
	}
	if strings.HasSuffix(escapedPath, "/stats") {
		return "", "", errSubscriptionActionNotFound
	}
	for _, action := range []string{"preview", "traffic", "render"} {
		suffix := "/" + action
		if !strings.HasSuffix(escapedPath, suffix) {
			continue
		}
		raw := strings.TrimSuffix(strings.TrimPrefix(escapedPath, prefix), suffix)
		if raw == "" {
			return "", "", domain.NewError(domain.CodeInvalidArgument, "subscription name is required")
		}
		name, err := url.PathUnescape(raw)
		if err != nil {
			return "", "", domain.WrapError(domain.CodeInvalidArgument, "invalid subscription name", err)
		}
		if err := validateRequiredPublicResourceName("subscription name", name); err != nil {
			return "", "", err
		}
		return action, name, nil
	}
	return "", "", domain.NewError(domain.CodeInvalidArgument, "subscription action path must end with /preview or /traffic or /render")
}
