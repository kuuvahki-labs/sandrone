package domain

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestReportCloneRemoteSourceResults(t *testing.T) {
	report := Report{SourceResults: []RemoteSourceResult{{Index: 0, URL: "https://example.com", Error: &AppError{Code: CodeRemoteFetchFailed, Message: "failed"}}}}
	cloned := report.Clone()
	cloned.SourceResults[0].URL = "https://different.example"
	cloned.SourceResults[0].Error.Message = "changed"
	require.Equal(t, "https://example.com", report.SourceResults[0].URL)
	require.Equal(t, "failed", report.SourceResults[0].Error.Message)
}
