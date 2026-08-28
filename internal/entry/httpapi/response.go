package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

const maxBackupRestoreBodyBytes = 32 << 20

func writeFileContent(w http.ResponseWriter, result *domain.FileResult) {
	for key, value := range result.Response.Headers {
		w.Header().Set(key, value)
	}
	if w.Header().Get("Content-Type") == "" {
		w.Header().Set("Content-Type", fileContentType(result))
	}
	status := result.Response.Status
	if status == 0 {
		status = http.StatusOK
	}
	w.WriteHeader(status)
	_, _ = w.Write(result.Content)
}

func fileContentType(result *domain.FileResult) string {
	if result.ContentType != "" {
		return result.ContentType
	}
	return "application/octet-stream"
}

func writeResult(w http.ResponseWriter, result any, err error) {
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func reportWarnings(report domain.Report) []domain.Warning {
	if report.Warnings == nil {
		return []domain.Warning{}
	}
	return report.Warnings
}

func decodeJSON(w http.ResponseWriter, r *http.Request, out any) bool {
	defer func() { _ = r.Body.Close() }()
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 16<<20))
	if err != nil {
		writeError(w, domain.NewError(domain.CodeInvalidArgument, "request body is too large"), http.StatusRequestEntityTooLarge)
		return false
	}
	if len(strings.TrimSpace(string(body))) == 0 {
		writeError(w, domain.NewError(domain.CodeInvalidArgument, "request body is required"), http.StatusBadRequest)
		return false
	}
	if err := json.Unmarshal(body, out); err != nil {
		writeError(w, domain.WrapError(domain.CodeInvalidArgument, "invalid JSON body", err), http.StatusBadRequest)
		return false
	}
	return true
}

func decodeOptionalJSON(w http.ResponseWriter, r *http.Request, out any) bool {
	defer func() { _ = r.Body.Close() }()
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 16<<20))
	if err != nil {
		writeError(w, domain.NewError(domain.CodeInvalidArgument, "request body is too large"), http.StatusRequestEntityTooLarge)
		return false
	}
	if len(strings.TrimSpace(string(body))) == 0 {
		return true
	}
	if err := json.Unmarshal(body, out); err != nil {
		writeError(w, domain.WrapError(domain.CodeInvalidArgument, "invalid JSON body", err), http.StatusBadRequest)
		return false
	}
	return true
}

func readBackupBody(w http.ResponseWriter, r *http.Request) ([]byte, bool) {
	defer func() { _ = r.Body.Close() }()
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxBackupRestoreBodyBytes))
	if err != nil {
		if _, ok := errors.AsType[*http.MaxBytesError](err); ok {
			writeError(w, domain.NewError(domain.CodeBackupTooLarge, "backup archive is too large"), http.StatusRequestEntityTooLarge)
			return nil, false
		}
		writeError(w, domain.NewError(domain.CodeBackupInvalid, "backup archive is unreadable"), http.StatusBadRequest)
		return nil, false
	}
	if len(body) == 0 {
		writeError(w, domain.NewError(domain.CodeBackupInvalid, "backup archive is required"), http.StatusBadRequest)
		return nil, false
	}
	return body, true
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	_ = encoder.Encode(value)
}

func writeServiceError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	switch {
	case domain.IsCode(err, domain.CodeBackupInvalid):
		status = http.StatusBadRequest
	case domain.IsCode(err, domain.CodeBackupTooLarge):
		status = http.StatusRequestEntityTooLarge
	case domain.IsCode(err, domain.CodeBackupIncompatible):
		status = http.StatusUnprocessableEntity
	case domain.IsCode(err, domain.CodeBackupRestoreFailed):
		status = http.StatusInternalServerError
	default:
		if domain.IsCode(err, domain.CodeInvalidArgument) ||
			domain.IsCode(err, domain.CodeNodeValidationFailed) ||
			domain.IsCode(err, domain.CodeFileInputNotFound) ||
			domain.IsCode(err, domain.CodeFileDependencyCycle) {
			status = http.StatusBadRequest
		}
		if errors.Is(err, os.ErrNotExist) {
			status = http.StatusNotFound
		}
	}
	writeError(w, err, status)
}

func writeError(w http.ResponseWriter, err error, status int) {
	body := errorBody{Code: "internal_error", Message: err.Error()}
	if appErr, ok := errors.AsType[*domain.AppError](err); ok {
		body.Code = string(appErr.Code)
		body.Message = appErr.Message
	}
	writeJSON(w, status, errorResponse{Error: body})
}
