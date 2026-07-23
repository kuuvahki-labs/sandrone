package mcpapi

import (
	"encoding/json"
	"os"
	"os/exec"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

func TestToolErrorCompatibilityModeDoesNotExposeCause(t *testing.T) {
	command := exec.Command(os.Args[0], "-test.run=TestToolErrorCompatibilityHelper")
	command.Env = append(os.Environ(),
		"MCPGODEBUG=seterroroverwrite=1",
		"SANDRONE_TEST_MCP_ERROR_HELPER=1",
	)
	output, err := command.CombinedOutput()
	require.NoError(t, err, string(output))
	require.NotContains(t, string(output), "cause-secret")
	require.Contains(t, string(output), "processor_config_invalid")
}

func TestToolErrorCompatibilityHelper(t *testing.T) {
	if os.Getenv("SANDRONE_TEST_MCP_ERROR_HELPER") != "1" {
		t.Skip("subprocess helper")
	}
	result := newToolErrorResult(&domain.AppError{
		Code:      domain.CodeProcessorConfigInvalid,
		Message:   "invalid processor configuration",
		Processor: "rename",
		Cause:     sensitiveTestError("cause-secret"),
	}, toolErrorContext{})
	body, err := json.Marshal(result)
	require.NoError(t, err)
	_, err = os.Stdout.Write(body)
	require.NoError(t, err)
}

func TestToolErrorPreservesNonURLSource(t *testing.T) {
	result := newToolErrorResult(&domain.AppError{
		Code:    domain.CodeFileInputNotFound,
		Message: "source unavailable",
		Source:  "subscription:demo",
	}, toolErrorContext{})
	body, err := json.Marshal(result.StructuredContent)
	require.NoError(t, err)
	require.Contains(t, string(body), `"source":"subscription:demo"`)
}

func TestToolErrorSanitizesBarePathNotExist(t *testing.T) {
	const (
		privatePath = "/Users/private-owner/Library/Application Support/sandrone/secrets.yaml"
		privateOp   = "open-private-config"
		wantMessage = "requested resource was not found"
	)
	result := newToolErrorResult(&os.PathError{
		Op:   privateOp,
		Path: privatePath,
		Err:  os.ErrNotExist,
	}, toolErrorContext{})

	require.True(t, result.IsError)
	require.Equal(t, []mcp.Content{&mcp.TextContent{
		Text: string(domain.CodeFileInputNotFound) + ": " + wantMessage,
	}}, result.Content)

	bodyJSON, err := json.Marshal(result.StructuredContent)
	require.NoError(t, err)
	var body toolErrorEnvelope
	require.NoError(t, json.Unmarshal(bodyJSON, &body))
	require.Equal(t, string(domain.CodeFileInputNotFound), body.Error.Code)
	require.Equal(t, wantMessage, body.Error.Message)
	require.Empty(t, body.Error.Field)
	require.Empty(t, body.Error.ResourceName)

	for _, public := range []string{result.Content[0].(*mcp.TextContent).Text, string(bodyJSON)} {
		require.NotContains(t, public, privatePath)
		require.NotContains(t, public, privateOp)
	}
}

type sensitiveTestError string

func (err sensitiveTestError) Error() string { return string(err) }
