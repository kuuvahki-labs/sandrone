package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/app"
	"github.com/kuuvahki-labs/sandrone/internal/buildinfo"
	"github.com/kuuvahki-labs/sandrone/internal/domain"
	"github.com/kuuvahki-labs/sandrone/internal/envconfig"
	"github.com/kuuvahki-labs/sandrone/pkg/sandrone"
)

func TestVersionFlagPrintsBuildVersion(t *testing.T) {
	code, stdout, stderr := runCLI(t, []string{"--version"}, "")

	require.Equal(t, 0, code)
	require.Equal(t, "sandrone version "+buildinfo.Summary()+"\n", stdout)
	require.Empty(t, stderr)
}

func TestConvertHelpDocumentsFormatsAndExamples(t *testing.T) {
	help := runHelp(t, "convert")

	for _, want := range []string{
		"input formats",
		"uri-list",
		"base64",
		"mihomo",
		"sing-box",
		"target formats",
		"mihomo-proxies",
		"shadowrocket-proxies",
		"sing-box-outbounds",
		"json-nodes",
		"sandrone convert --from uri-list --to mihomo-proxies",
	} {
		require.Contains(t, help, want)
	}
	require.Contains(t, help, "target formats: base64")
}

func TestCommandHelpDocumentsOperationalInputs(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want []string
	}{
		{
			name: "diagnose",
			args: []string{"diagnose"},
			want: []string{"input", "url", "subscription", "file"},
		},
		{
			name: "render",
			args: []string{"render"},
			want: []string{"subscription", "file"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			help := runHelp(t, tt.args...)
			for _, want := range tt.want {
				require.Contains(t, help, want)
			}
		})
	}
}

func TestCLIHelpDoesNotExposeFileCommand(t *testing.T) {
	help := runHelp(t)
	require.NotContains(t, help, "\n  file ")

	code, _, stderr := runCLI(t, []string{"file", "--help"}, "")
	require.Equal(t, 1, code)
	require.Contains(t, stderr, "unknown command")
}

func TestRenderSubscriptionMapsRequestAndWritesBody(t *testing.T) {
	rec := &recordingEngine{
		subscriptionRenderResult: &sandrone.RenderResult{Body: []byte("rendered\n")},
	}
	code, stdout, stderr := runCLI(t,
		[]string{"render", "subscription", "example", "--format", "mihomo-proxies", "--arg", "environment=test", "--arg", "empty=", "--refresh"},
		"",
		WithEngineFactory(func(string) engine { return rec }),
	)

	require.Equal(t, 0, code, stderr)
	require.Equal(t, "rendered\n", stdout)
	require.Empty(t, stderr)
	require.Len(t, rec.subscriptionRenderRequests, 1)
	req := rec.subscriptionRenderRequests[0]
	require.Equal(t, "example", req.Name)
	require.Equal(t, "mihomo-proxies", req.Format)
	require.Equal(t, map[string]string{"environment": "test", "empty": ""}, req.Request.Args)
	require.True(t, req.Refresh)
}

func TestRenderSubscriptionRequiresFormatAndValidArgs(t *testing.T) {
	for _, args := range [][]string{
		{"render", "subscription", "example"},
		{"render", "subscription", "example", "--format", "uri-list", "--arg", "missing-separator"},
		{"render", "subscription", "example", "--format", "uri-list", "--arg", "=value"},
	} {
		code, stdout, stderr := runCLI(t, args, "")
		require.Equal(t, 1, code)
		require.Empty(t, stdout)
		require.NotEmpty(t, stderr)
	}
}

func TestRenderFileMapsStoredRequestAndWritesOutputs(t *testing.T) {
	dir := t.TempDir()
	outputPath := filepath.Join(dir, "client.conf")
	reportPath := filepath.Join(dir, "report.json")
	rec := &recordingEngine{
		fileResult: &sandrone.FileResult{
			Content: []byte("final body"),
			Report:  sandrone.Report{Kind: "file"},
		},
	}
	code, stdout, stderr := runCLI(t,
		[]string{"render", "file", "client.conf", "--arg", "profile=travel", "--refresh", "--output", outputPath, "--report-output", reportPath},
		"",
		WithEngineFactory(func(string) engine { return rec }),
	)

	require.Equal(t, 0, code, stderr)
	require.Empty(t, stdout)
	require.Empty(t, stderr)
	require.Len(t, rec.fileRequests, 1)
	req := rec.fileRequests[0]
	require.Equal(t, "client.conf", req.Name)
	require.Nil(t, req.Spec)
	require.Equal(t, map[string]string{"profile": "travel"}, req.Request.Args)
	require.True(t, req.Refresh)
	body, err := os.ReadFile(outputPath)
	require.NoError(t, err)
	require.Equal(t, "final body", string(body))
	reportBody, err := os.ReadFile(reportPath)
	require.NoError(t, err)
	require.Contains(t, string(reportBody), `"kind": "file"`)
}

func TestRenderFileReadsLocalJSONSpec(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "file.json")
	require.NoError(t, os.WriteFile(specPath, []byte(`{"name":"local.txt","kind":"static","source":{"type":"inline","content":"body"}}`), 0o644))

	code, stdout, stderr := runCLI(t,
		[]string{"--data-dir", filepath.Join(dir, "data"), "render", "file", specPath},
		"",
	)

	require.Equal(t, 0, code, stderr)
	require.Equal(t, "body", stdout)
	require.Empty(t, stderr)
}

func TestConvertMapsFlagsToEngineRequest(t *testing.T) {
	rec := &recordingEngine{
		convertResult: &sandrone.RenderResult{Body: []byte("proxies: []\n")},
	}
	dataDir := t.TempDir()

	code, stdout, stderr := runCLI(t,
		[]string{"--data-dir", dataDir, "convert", "--from", "uri-list", "--to", "mihomo-proxies", "--input", "-"},
		"raw input",
		WithEngineFactory(func(gotDataDir string) engine {
			require.Equal(t, dataDir, gotDataDir)
			return rec
		}),
	)

	require.Equal(t, 0, code)
	require.Equal(t, "proxies: []\n", stdout)
	require.Empty(t, stderr)
	require.Len(t, rec.convertRequests, 1)
	require.Equal(t, "uri-list", rec.convertRequests[0].FromFormat)
	require.Equal(t, "mihomo-proxies", rec.convertRequests[0].ToFormat)
	require.Equal(t, "raw input", string(rec.convertRequests[0].Content))
	require.Empty(t, rec.parseRequests)
	require.Empty(t, rec.renderRequests)
}

func TestConvertMapsInputURLToRemoteRequest(t *testing.T) {
	rec := &recordingEngine{convertResult: &sandrone.RenderResult{Body: []byte("[]\n")}}
	code, _, stderr := runCLI(t,
		[]string{"convert", "--to", "json-nodes", "--input-url", "https://example.com/sub", "--user-agent", "ua", "--proxy", "http://127.0.0.1:8080", "--remote-timeout", "7s"},
		"stdin should not be read",
		WithEngineFactory(func(string) engine { return rec }),
	)
	require.Equal(t, 0, code, stderr)
	require.Len(t, rec.convertRequests, 1)
	req := rec.convertRequests[0]
	require.Empty(t, req.FromFormat)
	require.Equal(t, "json-nodes", req.ToFormat)
	require.Empty(t, req.Content)
	require.NotNil(t, req.Remote)
	require.Equal(t, "https://example.com/sub", req.Remote.URL)
	require.Equal(t, "ua", req.Remote.UserAgent)
	require.Equal(t, "http://127.0.0.1:8080", req.Remote.Proxy)
	require.Equal(t, 7000, req.Remote.TimeoutMS)
}

func TestConvertRejectsInputAndInputURLTogether(t *testing.T) {
	for _, args := range [][]string{
		{"convert", "--from", "uri-list", "--to", "json-nodes", "--input", "nodes.txt", "--input-url", "https://example.com/sub"},
		{"convert", "--from", "uri-list", "--to", "json-nodes", "--input", "-", "--input-url", "https://example.com/sub"},
	} {
		code, stdout, stderr := runCLI(t, args, "")
		require.Equal(t, 1, code)
		require.Empty(t, stdout)
		require.Contains(t, stderr, "--input and --input-url are mutually exclusive")
	}
}

func TestConvertURIListToMihomoProxies(t *testing.T) {
	code, stdout, stderr := runCLI(t,
		[]string{"convert", "--from", "uri-list", "--to", "mihomo-proxies", "--input", "-"},
		"ss://aes-128-gcm:secret@example.com:8388#node-a",
	)

	require.Equal(t, 0, code, stderr)
	require.Empty(t, stderr)
	require.Contains(t, stdout, "proxies:")
	require.Contains(t, stdout, "node-a")
}

func TestConvertURIListToJSONNodes(t *testing.T) {
	code, stdout, stderr := runCLI(t,
		[]string{"convert", "--from", "uri-list", "--to", "json-nodes", "--input", "-"},
		"ss://aes-128-gcm:secret@example.com:8388#node-a",
	)

	require.Equal(t, 0, code, stderr)
	require.Empty(t, stderr)
	var nodes []sandrone.NodeIR
	require.NoError(t, json.Unmarshal([]byte(stdout), &nodes))
	require.Len(t, nodes, 1)
	require.Equal(t, "node-a", nodes[0].Name)
}

func TestDiagnoseCommandTreeReplacesProbeAndValidate(t *testing.T) {
	root := NewRootCommand(WithEnv(map[string]string{}))
	names := map[string]bool{}
	for _, command := range root.Commands() {
		names[command.Name()] = true
	}
	require.True(t, names["diagnose"])
	require.False(t, names["probe"])
	require.False(t, names["validate"])

	diagnose, _, err := root.Find([]string{"diagnose"})
	require.NoError(t, err)
	children := map[string]bool{}
	for _, command := range diagnose.Commands() {
		children[command.Name()] = true
	}
	require.Equal(t, map[string]bool{"file": true, "input": true, "subscription": true, "url": true}, children)
}

func TestDiagnoseInputAndURLMapRequests(t *testing.T) {
	dir := t.TempDir()
	processorsPath := filepath.Join(dir, "processors.json")
	require.NoError(t, os.WriteFile(processorsPath, []byte(`[{"type":"rename","stage":"nodes","params":{"mode":"prefix","value":"x-"}}]`), 0o600))
	rec := &recordingEngine{diagnoseResult: &sandrone.DiagnoseResult{Status: sandrone.DiagnoseStatusOK}}

	code, _, stderr := runCLI(t,
		[]string{"diagnose", "input", "-", "--kind", "nodes", "--format", "uri-list", "--processors", processorsPath},
		"node input", WithEngineFactory(func(string) engine { return rec }),
	)
	require.Equal(t, 0, code, stderr)
	require.Len(t, rec.diagnoseRequests, 1)
	require.Equal(t, sandrone.DiagnoseInputNodes, rec.diagnoseRequests[0].Kind)
	require.Equal(t, "uri-list", rec.diagnoseRequests[0].Format)
	require.Equal(t, "node input", string(rec.diagnoseRequests[0].Content))
	require.Len(t, rec.diagnoseRequests[0].Processors, 1)

	code, _, stderr = runCLI(t,
		[]string{"diagnose", "url", "https://example.com/sub", "--user-agent", "ua", "--proxy", "http://127.0.0.1:8080", "--remote-timeout", "7s"},
		"", WithEngineFactory(func(string) engine { return rec }),
	)
	require.Equal(t, 0, code, stderr)
	require.Len(t, rec.diagnoseRequests, 2)
	remote := rec.diagnoseRequests[1].Remote
	require.NotNil(t, remote)
	require.Equal(t, "https://example.com/sub", remote.URL)
	require.Equal(t, "ua", remote.UserAgent)
	require.Equal(t, "http://127.0.0.1:8080", remote.Proxy)
	require.Equal(t, 7000, remote.TimeoutMS)
}

func TestDiagnoseSubscriptionAndFileMapStoredAndLocalInputs(t *testing.T) {
	rec := &recordingEngine{diagnoseResult: &sandrone.DiagnoseResult{Status: sandrone.DiagnoseStatusOK}}
	factory := WithEngineFactory(func(string) engine { return rec })

	code, _, stderr := runCLI(t, []string{"diagnose", "subscription", "provider", "--cache-mode", "reuse"}, "", factory)
	require.Equal(t, 0, code, stderr)
	require.Equal(t, "provider", rec.diagnoseRequests[0].SubscriptionName)
	require.Equal(t, sandrone.DiagnoseCacheModeReuse, rec.diagnoseRequests[0].CacheMode)

	code, _, stderr = runCLI(t, []string{"diagnose", "file", "default.yaml"}, "", factory)
	require.Equal(t, 0, code, stderr)
	require.Equal(t, "default.yaml", rec.diagnoseRequests[1].File.Name)

	specPath := filepath.Join(t.TempDir(), "local.json")
	require.NoError(t, os.WriteFile(specPath, []byte(`{"kind":"static","source":{"type":"inline","content":"hello"}}`), 0o600))
	code, _, stderr = runCLI(t, []string{"diagnose", "file", specPath}, "", factory)
	require.Equal(t, 0, code, stderr)
	require.NotNil(t, rec.diagnoseRequests[2].File.Spec)
	require.Equal(t, sandrone.FileKindStatic, rec.diagnoseRequests[2].File.Spec.Kind)
}

func TestCLIResourceDefinitionFilesUseJSON(t *testing.T) {
	dir := t.TempDir()
	jsonPath := filepath.Join(dir, "file.json")
	require.NoError(t, os.WriteFile(jsonPath, []byte(`{"kind":"static","source":{"type":"inline","content":"hello"}}`), 0o600))
	spec, err := readFileSpec(jsonPath)
	require.NoError(t, err)
	require.Equal(t, sandrone.FileKindStatic, spec.Kind)

	yamlPath := filepath.Join(dir, "file.yml")
	require.NoError(t, os.WriteFile(yamlPath, []byte("kind: static\nsource:\n  type: inline\n  content: hello\n"), 0o600))
	_, err = readFileSpec(yamlPath)
	require.ErrorContains(t, err, "must be .json")

	processorYAMLPath := filepath.Join(dir, "processors.yaml")
	require.NoError(t, os.WriteFile(processorYAMLPath, []byte("- type: rename\n"), 0o600))
	_, err = readProcessorSpecs(processorYAMLPath, strings.NewReader(""))
	require.ErrorContains(t, err, "must be .json")

	processors, err := readProcessorSpecs("-", strings.NewReader(`[{"type":"rename"}]`))
	require.NoError(t, err)
	require.Len(t, processors, 1)
}

func TestDiagnoseFailedWritesJSONBeforeExitAndArgumentErrorsDoNot(t *testing.T) {
	rec := &recordingEngine{diagnoseResult: &sandrone.DiagnoseResult{
		Status: sandrone.DiagnoseStatusFailed,
		Error:  &domain.AppError{Code: "input_kind_unrecognized", Message: "unknown"},
	}}
	code, stdout, stderr := runCLI(t,
		[]string{"diagnose", "input", "-"}, "unknown",
		WithEngineFactory(func(string) engine { return rec }),
	)
	require.Equal(t, 1, code)
	require.JSONEq(t, `{"status":"failed","input":{"kind":""},"stages":null,"counts":{"input":0,"valid":0,"invalid":0,"error":0,"warning":0},"report":{"created_at":"0001-01-01T00:00:00Z","render":{"success_count":0,"lost_fields":0}},"error":{"code":"input_kind_unrecognized","message":"unknown"}}`, stdout)
	require.Contains(t, stderr, "diagnosis failed")

	code, stdout, stderr = runCLI(t,
		[]string{"diagnose", "input", "-", "--processors", "-"}, "anything",
		WithEngineFactory(func(string) engine { return rec }),
	)
	require.Equal(t, 1, code)
	require.Empty(t, stdout)
	require.Contains(t, stderr, "cannot both read from stdin")
	require.Len(t, rec.diagnoseRequests, 1)
}

func TestConvertWritesOutputFile(t *testing.T) {
	dir := t.TempDir()
	inputPath := filepath.Join(dir, "input.txt")
	outputPath := filepath.Join(dir, "out", "nodes.json")
	require.NoError(t, os.WriteFile(inputPath, []byte("ss://aes-128-gcm:secret@example.com:8388#node-a"), 0o644))

	code, stdout, stderr := runCLI(t,
		[]string{"convert", "--from", "uri-list", "--to", "json-nodes", "--input", inputPath, "--output", outputPath},
		"",
	)

	require.Equal(t, 0, code, stderr)
	require.Empty(t, stdout)
	require.Empty(t, stderr)
	body, err := os.ReadFile(outputPath)
	require.NoError(t, err)
	var nodes []sandrone.NodeIR
	require.NoError(t, json.Unmarshal(body, &nodes))
	require.Len(t, nodes, 1)
}

func TestFiniteJSONCommandsWriteOutputFile(t *testing.T) {
	tests := []struct {
		name string
		args func(string, string) []string
		rec  *recordingEngine
	}{
		{
			name: "diagnose",
			args: func(_ string, output string) []string {
				return []string{"diagnose", "input", "-", "--kind", "nodes", "--format", "uri-list", "--output", output}
			},
			rec: &recordingEngine{diagnoseResult: &sandrone.DiagnoseResult{Status: sandrone.DiagnoseStatusOK}},
		},
		{
			name: "inspect",
			args: func(_ string, output string) []string {
				return []string{"inspect", "--output", output}
			},
			rec: &recordingEngine{inspectResult: &sandrone.InspectResult{Formats: sandrone.InspectFormats{Parse: []string{"uri-list"}}}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			outputPath := filepath.Join(dir, "nested", "result.json")
			code, stdout, stderr := runCLI(t, tt.args(dir, outputPath), "ss://aes-128-gcm:secret@example.com:8388#node-a",
				WithEngineFactory(func(string) engine { return tt.rec }),
			)

			require.Equal(t, 0, code, stderr)
			require.Empty(t, stdout)
			body, err := os.ReadFile(outputPath)
			require.NoError(t, err)
			require.True(t, json.Valid(body))
			require.Equal(t, byte('\n'), body[len(body)-1])
			if tt.name == "diagnose" {
				info, err := os.Stat(outputPath)
				require.NoError(t, err)
				require.Equal(t, os.FileMode(0o600), info.Mode().Perm())
			}
		})
	}
}

func TestDoctorWritesOutputFile(t *testing.T) {
	dir := t.TempDir()
	outputPath := filepath.Join(dir, "nested", "doctor.json")
	code, stdout, stderr := runCLI(t,
		[]string{"--data-dir", filepath.Join(dir, "data"), "doctor", "--output", outputPath},
		"",
	)

	require.Equal(t, 0, code, stderr)
	require.Empty(t, stdout)
	body, err := os.ReadFile(outputPath)
	require.NoError(t, err)
	require.Contains(t, string(body), `"ok": true`)
}

func TestConvertWritesMainAndReportOutputFiles(t *testing.T) {
	dir := t.TempDir()
	outputPath := filepath.Join(dir, "main", "nodes.json")
	reportPath := filepath.Join(dir, "diagnostics", "report.json")
	rec := &recordingEngine{convertResult: &sandrone.RenderResult{
		Body:   []byte("rendered\n"),
		Report: sandrone.Report{Kind: "convert", Status: "ok"},
	}}

	code, stdout, stderr := runCLI(t,
		[]string{"convert", "--from", "uri-list", "--to", "json-nodes", "--input", "-", "--output", outputPath, "--report-output", reportPath},
		"raw",
		WithEngineFactory(func(string) engine { return rec }),
	)

	require.Equal(t, 0, code, stderr)
	require.Empty(t, stdout)
	mainBody, err := os.ReadFile(outputPath)
	require.NoError(t, err)
	require.Equal(t, "rendered\n", string(mainBody))
	reportBody, err := os.ReadFile(reportPath)
	require.NoError(t, err)
	require.Contains(t, string(reportBody), `"kind": "convert"`)
	require.Contains(t, string(reportBody), "\n  \"status\"")
	require.Equal(t, byte('\n'), reportBody[len(reportBody)-1])
}

func TestReportOutputRejectsStdoutAndSameMainPathBeforeServiceCall(t *testing.T) {
	for _, tt := range []struct {
		name         string
		output       func(string) string
		reportOutput func(string) string
		want         string
	}{
		{
			name:         "stdout",
			output:       func(string) string { return "" },
			reportOutput: func(string) string { return "-" },
			want:         "--report-output requires a file path",
		},
		{
			name:         "same cleaned path",
			output:       func(dir string) string { return filepath.Join(dir, "result.json") },
			reportOutput: func(dir string) string { return filepath.Join(dir, ".", "result.json") },
			want:         "must refer to different files",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			rec := &recordingEngine{convertResult: &sandrone.RenderResult{Body: []byte("body")}}
			args := []string{"convert", "--from", "uri-list", "--to", "json-nodes", "--input", "-", "--report-output", tt.reportOutput(dir)}
			if output := tt.output(dir); output != "" {
				args = append(args, "--output", output)
			}

			code, stdout, stderr := runCLI(t, args, "raw", WithEngineFactory(func(string) engine { return rec }))

			require.Equal(t, 1, code)
			require.Empty(t, stdout)
			require.Contains(t, stderr, tt.want)
			require.Empty(t, rec.convertRequests)
		})
	}
}

func TestReportOutputRejectsSameExistingFileBeforeServiceCall(t *testing.T) {
	dir := t.TempDir()
	outputPath := filepath.Join(dir, "main.json")
	reportPath := filepath.Join(dir, "report.json")
	require.NoError(t, os.WriteFile(outputPath, []byte("old"), 0o644))
	require.NoError(t, os.Link(outputPath, reportPath))
	rec := &recordingEngine{convertResult: &sandrone.RenderResult{Body: []byte("new")}}

	code, stdout, stderr := runCLI(t,
		[]string{"convert", "--from", "uri-list", "--to", "json-nodes", "--input", "-", "--output", outputPath, "--report-output", reportPath},
		"raw",
		WithEngineFactory(func(string) engine { return rec }),
	)

	require.Equal(t, 1, code)
	require.Empty(t, stdout)
	require.Contains(t, stderr, "must refer to different files")
	require.Empty(t, rec.convertRequests)
}

func TestReportOutputFailureKeepsWrittenMainOutput(t *testing.T) {
	dir := t.TempDir()
	outputPath := filepath.Join(dir, "main.txt")
	rec := &recordingEngine{convertResult: &sandrone.RenderResult{
		Body:   []byte("new body"),
		Report: sandrone.Report{Kind: "convert"},
	}}

	code, stdout, stderr := runCLI(t,
		[]string{"convert", "--from", "uri-list", "--to", "json-nodes", "--input", "-", "--output", outputPath, "--report-output", dir},
		"raw",
		WithEngineFactory(func(string) engine { return rec }),
	)

	require.Equal(t, 1, code)
	require.Empty(t, stdout)
	require.NotEmpty(t, stderr)
	mainBody, err := os.ReadFile(outputPath)
	require.NoError(t, err)
	require.Equal(t, "new body", string(mainBody))
}

func TestOutputFileIsOverwritten(t *testing.T) {
	dir := t.TempDir()
	outputPath := filepath.Join(dir, "result.txt")
	require.NoError(t, os.WriteFile(outputPath, []byte("old content that is longer"), 0o600))
	rec := &recordingEngine{convertResult: &sandrone.RenderResult{Body: []byte("new")}}

	code, _, stderr := runCLI(t,
		[]string{"convert", "--from", "uri-list", "--to", "json-nodes", "--input", "-", "--output", outputPath},
		"raw",
		WithEngineFactory(func(string) engine { return rec }),
	)

	require.Equal(t, 0, code, stderr)
	body, err := os.ReadFile(outputPath)
	require.NoError(t, err)
	require.Equal(t, "new", string(body))
}

func TestInspectCommandOutputsRuntimeSummary(t *testing.T) {
	code, stdout, stderr := runCLI(t,
		[]string{"inspect"},
		"",
	)
	require.Equal(t, 0, code, stderr)
	require.Contains(t, stdout, `"formats"`)
	require.NotContains(t, stdout, `"fields"`)
}

func TestCapabilityCommandsOutputFormatIndexAndDetail(t *testing.T) {
	rec := &recordingEngine{
		formatCapabilityList: &sandrone.FormatCapabilityListResult{Items: []sandrone.FormatCapabilitySummary{{
			Direction: sandrone.CapabilityDirectionRender,
			Format:    "uri-list",
		}}},
		formatCapability: &sandrone.FormatCapability{
			Direction: sandrone.CapabilityDirectionRender,
			Format:    "uri-list",
		},
	}

	code, stdout, stderr := runCLI(t, []string{"capability", "formats"}, "",
		WithEngineFactory(func(string) engine { return rec }),
	)
	require.Equal(t, 0, code, stderr)
	require.Contains(t, stdout, `"format": "uri-list"`)

	code, stdout, stderr = runCLI(t, []string{"capability", "format", "render", "uri-list"}, "",
		WithEngineFactory(func(string) engine { return rec }),
	)
	require.Equal(t, 0, code, stderr)
	require.Contains(t, stdout, `"direction": "render"`)
	require.Equal(t, []sandrone.FormatCapabilityRequest{{
		Direction: sandrone.CapabilityDirectionRender,
		Format:    "uri-list",
	}}, rec.formatCapabilityRequests)
}

func TestErrorOutputAndExitCode(t *testing.T) {
	code, stdout, stderr := runCLI(t,
		[]string{"convert", "--from", "unsupported", "--to", "json-nodes", "--input", "-"},
		"not-a-node",
	)

	require.Equal(t, 1, code)
	require.Empty(t, stdout)
	require.Contains(t, stderr, "unsupported parse format")
}

func TestDoctorReportsFormatsAndDataDir(t *testing.T) {
	dataDir := t.TempDir()

	code, stdout, stderr := runCLI(t,
		[]string{"--data-dir", dataDir, "doctor"},
		"",
	)

	require.Equal(t, 0, code, stderr)
	require.Empty(t, stderr)
	var result doctorResult
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))
	require.True(t, result.OK)
	require.Equal(t, "filesystem", result.StorageBackend)
	require.True(t, result.StorageOK)
	require.True(t, result.DataDirWritable)
	parseFormats := make([]string, 0, len(result.ParseFormats))
	for _, check := range result.ParseFormats {
		parseFormats = append(parseFormats, check.Name)
	}
	require.ElementsMatch(t, []string{"uri", "uri-list", "base64", "mihomo", "sing-box", "json-nodes"}, parseFormats)
	renderFormats := make([]string, 0, len(result.RenderFormats))
	for _, check := range result.RenderFormats {
		renderFormats = append(renderFormats, check.Name)
	}
	require.ElementsMatch(t, []string{"base64", "mihomo-proxies", "shadowrocket-proxies", "sing-box-outbounds", "json-nodes", "uri-list"}, renderFormats)
}

func TestCLIRejectsInvalidStorageBackend(t *testing.T) {
	code, stdout, stderr := runCLI(t, []string{"inspect"}, "", WithEnv(map[string]string{
		envconfig.StorageBackend: "r2",
	}))

	require.Equal(t, 1, code)
	require.Empty(t, stdout)
	require.Contains(t, stderr, envconfig.StorageBackend)
}

func TestServeCommandRunsWithoutSubcommands(t *testing.T) {
	cmd := NewRootCommand(WithEnv(map[string]string{}))
	serve, _, err := cmd.Find([]string{"serve"})
	require.NoError(t, err)
	require.Equal(t, "serve", serve.Name())
	require.Empty(t, serve.Commands())
}

func TestServeHelpDocumentsUnifiedEntrypointFlags(t *testing.T) {
	help := runHelp(t, "serve")

	require.Contains(t, help, "--path")
	require.Contains(t, help, "--max-output-bytes")
}

func TestServeHelpOmitsRemovedAuthenticationAndTransportFlags(t *testing.T) {
	help := runHelp(t, "serve")

	require.NotContains(t, help, "--token-required")
	require.NotContains(t, help, "--transport")
	require.NotContains(t, help, "--allow-management-tools")
}

func TestServePassesFlagsToRuntime(t *testing.T) {
	dataDir := t.TempDir()
	stopErr := errors.New("stop after runtime")
	var got app.Config

	code, _, stderr := runCLI(t,
		[]string{
			"--data-dir", dataDir,
			"serve",
			"--listen", "127.0.0.1:0",
			"--token", "secret",
			"--path", "/agent",
			"--max-output-bytes", "2048",
		},
		"",
		WithRuntimeFactory(func(cfg app.Config) (*app.Runtime, error) {
			got = cfg
			return nil, stopErr
		}),
	)

	require.Equal(t, 1, code)
	require.Contains(t, stderr, stopErr.Error())
	require.Equal(t, dataDir, got.DataDir)
	require.Equal(t, "127.0.0.1:0", got.HTTP.Listen)
	require.Equal(t, "secret", got.HTTP.Token)
	require.Equal(t, "/agent", got.MCP.Path)
	require.Equal(t, 2048, got.MCP.MaxOutputBytes)
	require.Equal(t, "flag", got.OverrideSources["mcp.path"])
	require.Equal(t, "flag", got.OverrideSources["mcp.max_output_bytes"])
}

func TestServeRejectsRemovedManagementToolsFlag(t *testing.T) {
	code, _, stderr := runCLI(t,
		[]string{"serve", "--allow-management-tools"},
		"",
	)

	require.Equal(t, 1, code)
	require.Contains(t, stderr, "unknown flag: --allow-management-tools")
}

func TestServeUsesTokenEnv(t *testing.T) {
	dataDir := t.TempDir()
	stopErr := errors.New("stop after runtime")
	var got app.Config

	code, _, stderr := runCLI(t,
		[]string{"--data-dir", dataDir, "serve"},
		"",
		WithEnv(map[string]string{envconfig.Token: "env-secret"}),
		WithRuntimeFactory(func(cfg app.Config) (*app.Runtime, error) {
			got = cfg
			return nil, stopErr
		}),
	)

	require.Equal(t, 1, code)
	require.Contains(t, stderr, stopErr.Error())
	require.Equal(t, "env-secret", got.HTTP.Token)
}

func TestServeIgnoresRemovedWebUIStaticDirEnv(t *testing.T) {
	dataDir := t.TempDir()
	stopErr := errors.New("stop after runtime")
	var got app.Config

	code, _, stderr := runCLI(t,
		[]string{"--data-dir", dataDir, "serve"},
		"",
		WithEnv(map[string]string{"SANDRONE_WEBUI_STATIC_DIR": t.TempDir()}),
		WithRuntimeFactory(func(cfg app.Config) (*app.Runtime, error) {
			got = cfg
			return nil, stopErr
		}),
	)

	require.Equal(t, 1, code)
	require.Contains(t, stderr, stopErr.Error())
	require.NotContains(t, got.OverrideSources, "webui.static_dir")
}

func TestServeRejectsRemovedWebUIStaticDirFlag(t *testing.T) {
	dataDir := t.TempDir()

	code, _, stderr := runCLI(t,
		[]string{"--data-dir", dataDir, "serve", "--webui-static-dir", t.TempDir()},
		"",
	)

	require.Equal(t, 1, code)
	require.Contains(t, stderr, "unknown flag: --webui-static-dir")
}

func TestServeUsesDefaultListen(t *testing.T) {
	dataDir := t.TempDir()
	stopErr := errors.New("stop after runtime")
	var got app.Config

	code, _, stderr := runCLI(t,
		[]string{"--data-dir", dataDir, "serve"},
		"",
		WithRuntimeFactory(func(cfg app.Config) (*app.Runtime, error) {
			got = cfg
			return nil, stopErr
		}),
	)

	require.Equal(t, 1, code)
	require.Contains(t, stderr, stopErr.Error())
	require.Equal(t, "127.0.0.1:1137", got.HTTP.Listen)
}

func TestServeMarksEnvironmentAndExplicitFlagOverrideSources(t *testing.T) {
	dataDir := t.TempDir()
	stopErr := errors.New("stop after runtime")
	var got app.Config

	code, _, stderr := runCLI(t,
		[]string{"--data-dir", dataDir, "serve", "--listen", "127.0.0.1:3237"},
		"",
		WithEnv(map[string]string{
			envconfig.Listen:   "127.0.0.1:2237",
			envconfig.LogLevel: "warn",
		}),
		WithRuntimeFactory(func(cfg app.Config) (*app.Runtime, error) {
			got = cfg
			return nil, stopErr
		}),
	)

	require.Equal(t, 1, code)
	require.Contains(t, stderr, stopErr.Error())
	require.Equal(t, "127.0.0.1:3237", got.HTTP.Listen)
	require.Equal(t, "flag", got.OverrideSources["http.listen"])
	require.Equal(t, "warn", got.Log.Level)
	require.Equal(t, "environment", got.OverrideSources["log.level"])
	require.NotContains(t, got.OverrideSources, "mcp.path")
}

func TestServeReadsIntegerStartupEnvironmentOverride(t *testing.T) {
	dataDir := t.TempDir()
	stopErr := errors.New("stop after runtime")
	var got app.Config

	code, _, stderr := runCLI(t,
		[]string{"--data-dir", dataDir, "serve"},
		"",
		WithEnv(map[string]string{
			envconfig.Token:             "env-secret",
			envconfig.MCPPath:           "/agent",
			envconfig.MCPMaxOutputBytes: "2048",
		}),
		WithRuntimeFactory(func(cfg app.Config) (*app.Runtime, error) {
			got = cfg
			return nil, stopErr
		}),
	)

	require.Equal(t, 1, code)
	require.Contains(t, stderr, stopErr.Error())
	require.Equal(t, "/agent", got.MCP.Path)
	require.Equal(t, 2048, got.MCP.MaxOutputBytes)
	require.Equal(t, "environment", got.OverrideSources["mcp.path"])
	require.Equal(t, "environment", got.OverrideSources["mcp.max_output_bytes"])
}

func TestServePassesLogLevelToRuntime(t *testing.T) {
	dataDir := t.TempDir()
	stopErr := errors.New("stop after runtime")
	var got app.Config

	code, _, stderr := runCLI(t,
		[]string{"--data-dir", dataDir, "serve", "--log-level", "debug"},
		"",
		WithRuntimeFactory(func(cfg app.Config) (*app.Runtime, error) {
			got = cfg
			return nil, stopErr
		}),
	)

	require.Equal(t, 1, code)
	require.Contains(t, stderr, stopErr.Error())
	require.Equal(t, "debug", got.Log.Level)
}

func TestServePassesS3StorageToRuntime(t *testing.T) {
	stopErr := errors.New("stop after runtime")
	var got app.Config
	env := map[string]string{
		envconfig.StorageBackend:    "s3",
		envconfig.S3Endpoint:        "https://account.example.invalid",
		envconfig.S3Region:          "auto",
		envconfig.S3Bucket:          "bucket",
		envconfig.S3AccessKeyID:     "access-marker",
		envconfig.S3SecretAccessKey: "secret-marker",
	}

	code, _, stderr := runCLI(t, []string{"serve"}, "",
		WithEnv(env),
		WithRuntimeFactory(func(cfg app.Config) (*app.Runtime, error) {
			got = cfg
			return nil, stopErr
		}),
	)

	require.Equal(t, 1, code)
	require.Contains(t, stderr, stopErr.Error())
	require.Equal(t, app.StorageS3, got.Storage.Backend)
	require.Equal(t, app.DefaultS3Prefix, got.Storage.S3.Prefix)
}

func TestServeRejectsInvalidLogLevel(t *testing.T) {
	dataDir := t.TempDir()

	code, stdout, stderr := runCLI(t,
		[]string{"--data-dir", dataDir, "serve", "--log-level", "trace"},
		"",
	)

	require.Equal(t, 1, code)
	require.Empty(t, stdout)
	require.Contains(t, stderr, "unsupported log level")
}

func TestServeRejectsRemovedSubcommands(t *testing.T) {
	for _, name := range []string{"http", "mcp", "all"} {
		t.Run(name, func(t *testing.T) {
			code, stdout, stderr := runCLI(t, []string{"serve", name}, "")

			require.Equal(t, 1, code)
			require.Empty(t, stdout)
			require.Contains(t, stderr, "unknown command")
		})
	}
}

func TestServeServerMountsHTTPWebUIAndMCP(t *testing.T) {
	rt, err := app.NewRuntime(app.Config{
		DataDir: t.TempDir(),
		HTTP:    app.HTTPConfig{Token: "secret"},
		MCP:     app.MCPConfig{Path: "/agent"},
	}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	require.NoError(t, err)
	handler := newServeServer(rt).Handler()

	request := httptest.NewRequest(http.MethodGet, "/v1/inspect", nil)
	request.Header.Set("Authorization", "Bearer secret")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	require.Equal(t, http.StatusOK, response.Code)

	response = httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/", nil))
	require.Equal(t, http.StatusMethodNotAllowed, response.Code)

	discoverBody := `{"jsonrpc":"2.0","id":1,"method":"server/discover","params":{"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28","io.modelcontextprotocol/clientInfo":{"name":"cli-test","version":"1.0.0"},"io.modelcontextprotocol/clientCapabilities":{}}}}`
	newDiscoverRequest := func() *http.Request {
		request := httptest.NewRequest(http.MethodPost, "/agent", strings.NewReader(discoverBody))
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Accept", "application/json, text/event-stream")
		request.Header.Set("Mcp-Protocol-Version", "2026-07-28")
		request.Header.Set("Mcp-Method", "server/discover")
		return request
	}
	request = newDiscoverRequest()
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	require.Equal(t, http.StatusUnauthorized, response.Code)

	request = newDiscoverRequest()
	request.Header.Set("Authorization", "Bearer secret")
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	require.Equal(t, http.StatusOK, response.Code)
}

func runHelp(t *testing.T, args ...string) string {
	t.Helper()
	code, stdout, stderr := runCLI(t, append(args, "--help"), "")
	require.Equal(t, 0, code, stderr)
	require.Empty(t, stderr)
	return stdout
}

func runCLI(t *testing.T, args []string, stdin string, opts ...Option) (int, string, string) {
	t.Helper()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	allOpts := []Option{
		WithEnv(map[string]string{}),
		WithStreams(strings.NewReader(stdin), &stdout, &stderr),
	}
	allOpts = append(allOpts, opts...)
	code := ExecuteWithOptions(context.Background(), args, allOpts...)
	return code, stdout.String(), stderr.String()
}

type recordingEngine struct {
	parseRequests              []sandrone.ParseRequest
	renderRequests             []sandrone.RenderRequest
	convertRequests            []sandrone.ConvertRequest
	diagnoseRequests           []sandrone.DiagnoseRequest
	subscriptionRenderRequests []sandrone.SubscriptionRenderRequest
	fileRequests               []sandrone.FileRequest
	parseResult                *sandrone.ParseResult
	renderResult               *sandrone.RenderResult
	convertResult              *sandrone.RenderResult
	diagnoseResult             *sandrone.DiagnoseResult
	subscriptionRenderResult   *sandrone.RenderResult
	fileResult                 *sandrone.FileResult
	inspectResult              *sandrone.InspectResult
	formatCapabilityList       *sandrone.FormatCapabilityListResult
	formatCapability           *sandrone.FormatCapability
	formatCapabilityRequests   []sandrone.FormatCapabilityRequest
}

func (e *recordingEngine) Parse(_ context.Context, req sandrone.ParseRequest) (*sandrone.ParseResult, error) {
	e.parseRequests = append(e.parseRequests, req)
	return e.parseResult, nil
}

func (e *recordingEngine) Render(_ context.Context, req sandrone.RenderRequest) (*sandrone.RenderResult, error) {
	e.renderRequests = append(e.renderRequests, req)
	return e.renderResult, nil
}

func (e *recordingEngine) Convert(_ context.Context, req sandrone.ConvertRequest) (*sandrone.RenderResult, error) {
	e.convertRequests = append(e.convertRequests, req)
	return e.convertResult, nil
}

func (e *recordingEngine) Diagnose(_ context.Context, req sandrone.DiagnoseRequest) (*sandrone.DiagnoseResult, error) {
	e.diagnoseRequests = append(e.diagnoseRequests, req)
	if e.diagnoseResult != nil {
		return e.diagnoseResult, nil
	}
	return &sandrone.DiagnoseResult{Status: sandrone.DiagnoseStatusOK}, nil
}

func (e *recordingEngine) RenderSubscriptionRequest(_ context.Context, req sandrone.SubscriptionRenderRequest) (*sandrone.RenderResult, error) {
	e.subscriptionRenderRequests = append(e.subscriptionRenderRequests, req)
	if e.subscriptionRenderResult != nil {
		return e.subscriptionRenderResult, nil
	}
	return &sandrone.RenderResult{}, nil
}

func (e *recordingEngine) GetFile(_ context.Context, req sandrone.FileRequest) (*sandrone.FileResult, error) {
	e.fileRequests = append(e.fileRequests, req)
	if e.fileResult != nil {
		return e.fileResult, nil
	}
	return &sandrone.FileResult{}, nil
}

func (e *recordingEngine) Inspect(context.Context) (*sandrone.InspectResult, error) {
	if e.inspectResult != nil {
		return e.inspectResult, nil
	}
	return &sandrone.InspectResult{}, nil
}

func (e *recordingEngine) ListFormatCapabilities(context.Context) (*sandrone.FormatCapabilityListResult, error) {
	if e.formatCapabilityList != nil {
		return e.formatCapabilityList, nil
	}
	return &sandrone.FormatCapabilityListResult{}, nil
}

func (e *recordingEngine) GetFormatCapability(_ context.Context, req sandrone.FormatCapabilityRequest) (*sandrone.FormatCapability, error) {
	e.formatCapabilityRequests = append(e.formatCapabilityRequests, req)
	if e.formatCapability != nil {
		return e.formatCapability, nil
	}
	return &sandrone.FormatCapability{}, nil
}
