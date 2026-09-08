package httpapi

import (
	"encoding/json/jsontext"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWriteJSONRejectsEncodingFailureBeforeCommittingResponse(t *testing.T) {
	for name, value := range map[string]any{
		"invalid UTF-8 source": sourceResponse{Body: "private-source\xff"},
		"invalid raw JSON":     map[string]jsontext.Value{"private": jsontext.Value(`{"incomplete":`)},
		"nonfinite number":     map[string]any{"value": math.NaN()},
	} {
		t.Run(name, func(t *testing.T) {
			response := httptest.NewRecorder()
			writeJSON(response, http.StatusOK, value)
			require.Equal(t, http.StatusInternalServerError, response.Code)
			require.Equal(t, "application/json", response.Header().Get("Content-Type"))
			require.JSONEq(t, `{"error":{"code":"internal_error","message":"failed to encode JSON response"}}`, response.Body.String())
			require.NotContains(t, response.Body.String(), "private")
		})
	}
}

func TestWriteJSONKeepsSuccessfulStatusAndContent(t *testing.T) {
	response := httptest.NewRecorder()
	writeJSON(response, http.StatusCreated, map[string]any{"name": "配置", "ok": true})
	require.Equal(t, http.StatusCreated, response.Code)
	require.JSONEq(t, `{"name":"配置","ok":true}`, response.Body.String())
}
