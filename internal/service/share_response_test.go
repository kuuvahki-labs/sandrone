package service

import (
	"mime"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

func TestSetShareContentDispositionReplacesUpstreamHeaderCaseInsensitively(t *testing.T) {
	for _, upstreamName := range []string{
		"Content-Disposition",
		"content-disposition",
		"cOnTeNt-DiSpOsItIoN",
	} {
		t.Run(upstreamName, func(t *testing.T) {
			result := domain.ShareRenderResult{Headers: map[string]string{
				upstreamName: "attachment; filename=unsafe.txt",
				"X-Test":     "preserved",
			}}

			setShareContentDisposition(&result, "safe.txt")

			require.Len(t, result.Headers, 2)
			require.Equal(t, "preserved", result.Headers["X-Test"])
			mediaType, params, err := mime.ParseMediaType(result.Headers["Content-Disposition"])
			require.NoError(t, err)
			require.Equal(t, "inline", mediaType)
			require.Equal(t, "safe.txt", params["filename"])
		})
	}
}

func TestAgeShareFilenameCollapsesTerminalSuffixes(t *testing.T) {
	for _, tt := range []struct {
		name     string
		filename string
		expected string
	}{
		{name: "repeated lowercase", filename: "backup.age.age", expected: "backup.age"},
		{name: "mixed case", filename: "backup.AGE.age", expected: "backup.age"},
		{name: "suffixes only", filename: ".AGE.age", expected: "share.age"},
		{name: "empty", filename: "", expected: "share.age"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.expected, ageShareFilename(tt.filename))
		})
	}
}
