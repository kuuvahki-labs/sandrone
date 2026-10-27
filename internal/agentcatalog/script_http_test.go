package agentcatalog_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/agentcatalog"
	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

func TestScriptAPICatalogDescribesNodeHTTPContract(t *testing.T) {
	doc, err := agentcatalog.ScriptAPI()
	require.NoError(t, err)
	for _, method := range doc.Methods {
		if method.Name != "api.http.request" {
			continue
		}
		require.Equal(t, []domain.Stage{domain.StageNodes}, method.Stages)
		require.Len(t, method.Arguments, 2)
		require.True(t, method.Arguments[0].Required)
		require.True(t, method.Arguments[1].Required)
		require.Equal(t, "nodes", method.Arguments[0].Name)
		require.Equal(t, "options", method.Arguments[1].Name)
		opts := method.Arguments[1].Schema
		require.Contains(t, opts.Required, "url")
		require.Equal(t, []any{"sing-box", "mihomo"}, opts.Properties["core"].Enum)
		require.JSONEq(t, `"sing-box"`, string(opts.Properties["core"].Default))
		require.JSONEq(t, "5000", string(opts.Properties["timeout_ms"].Default))
		require.Contains(t, method.Returns.Schema.Types, "array")
		require.Contains(t, method.Returns.Schema.Items.Properties, "status_code")
		require.Contains(t, method.ErrorCodes, domain.CodeProbeBackendUnavailable)
		return
	}
	t.Fatal("api.http.request missing")
}
