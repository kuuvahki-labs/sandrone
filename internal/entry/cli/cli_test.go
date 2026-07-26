package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/app"
	"github.com/kuuvahki-labs/sandrone/internal/buildinfo"
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

func TestProbeHelpDocumentsHealthCheckMethods(t *testing.T) {
	help := runHelp(t, "probe")

	for _, want := range []string{
		"tcp-connect",
		"udp-ntp",
		"url-test",
		"--ntp-server",
		"--expected-status",
		"--cache-ttl",
		"http://www.gstatic.com/generate_204",
		"200-299",
	} {
		require.Contains(t, help, want)
	}
}

func TestCommandHelpDocumentsOperationalInputs(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want []string
	}{
		{
			name: "validate",
			args: []string{"validate"},
			want: []string{"--file", "--format", ".yaml", ".json"},
		},
		{
			name: "file",
			args: []string{"file"},
			want: []string{"mihomo", "sing-box", "Shadowrocket configuration", "shadowrocket-proxies"},
		},
		{
			name: "file render",
			args: []string{"file", "render"},
			want: []string{"name-or-spec-path", "FileSpec", "--output"},
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

func TestProbeMapsFlagsToEngineRequest(t *testing.T) {
	rec := &recordingEngine{
		probeResult: &sandrone.ProbeResult{
			Results: []sandrone.NodeProbeResult{{NodeName: "node-a", Method: "tcp_connect", Alive: true}},
		},
	}
	dataDir := t.TempDir()

	code, stdout, stderr := runCLI(t,
		[]string{"--data-dir", dataDir, "probe", "--format", "uri-list", "--method", "tcp-connect", "--input", "-", "--timeout", "5s", "--attempts", "2", "--concurrency", "3"},
		`{"name":"node-a","type":"ss","server":"example.com","port":443}`,
		WithEngineFactory(func(gotDataDir string) engine {
			require.Equal(t, dataDir, gotDataDir)
			return rec
		}),
	)

	require.Equal(t, 0, code)
	require.NotEmpty(t, stdout)
	require.Empty(t, stderr)
	require.Len(t, rec.probeRequests, 1)
	require.Equal(t, sandrone.ProbeTCPConnect, rec.probeRequests[0].Method)
	require.Equal(t, "uri-list", rec.probeRequests[0].Input.Format)
	require.Equal(t, 5000, rec.probeRequests[0].TimeoutMS)
	require.Equal(t, 2, rec.probeRequests[0].Attempts)
	require.Equal(t, 3, rec.probeRequests[0].Concurrency)
}

func TestProbeMapsInputURLToRemoteNodeInput(t *testing.T) {
	rec := &recordingEngine{
		probeResult: &sandrone.ProbeResult{
			Results: []sandrone.NodeProbeResult{{NodeName: "node-a", Method: "url_test", Core: "sing-box", Alive: true}},
		},
	}
	code, _, stderr := runCLI(t,
		[]string{"probe", "--input-url", "https://example.com/sub", "--user-agent", "ua", "--proxy", "http://127.0.0.1:8080", "--remote-timeout", "7s", "--timeout", "5s"},
		"stdin should not be read",
		WithEngineFactory(func(string) engine { return rec }),
	)
	require.Equal(t, 0, code, stderr)
	require.Len(t, rec.probeRequests, 1)
	req := rec.probeRequests[0]
	require.Equal(t, sandrone.ProbeURLTest, req.Method)
	require.Equal(t, "sing-box", req.Core)
	require.Equal(t, 5000, req.TimeoutMS)
	require.Equal(t, "remote", req.Input.Type)
	require.Empty(t, req.Input.Format)
	require.Equal(t, "https://example.com/sub", req.Input.URL)
	require.Equal(t, "ua", req.Input.UserAgent)
	require.Equal(t, "http://127.0.0.1:8080", req.Input.Proxy)
	require.Equal(t, 7000, req.Input.TimeoutMS)
}

func TestInputURLRejectsMutuallyExclusiveProbeAndValidateFlags(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{
			name: "probe path input with input-url",
			args: []string{"probe", "--format", "uri-list", "--input", "nodes.txt", "--input-url", "https://example.com/sub"},
		},
		{
			name: "probe explicit stdin with input-url",
			args: []string{"probe", "--format", "uri-list", "--input", "-", "--input-url", "https://example.com/sub"},
		},
		{
			name: "validate path input with input-url",
			args: []string{"validate", "--format", "uri-list", "--input", "nodes.txt", "--input-url", "https://example.com/sub"},
		},
		{
			name: "validate explicit stdin with input-url",
			args: []string{"validate", "--format", "uri-list", "--input", "-", "--input-url", "https://example.com/sub"},
		},
		{
			name: "validate file with input-url",
			args: []string{"validate", "--file", "something.yaml", "--input-url", "https://example.com/sub"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, stdout, stderr := runCLI(t, tt.args, "")
			require.Equal(t, 1, code)
			require.Empty(t, stdout)
			if strings.Contains(strings.Join(tt.args, " "), "--file") {
				require.Contains(t, stderr, "--file and --input-url are mutually exclusive")
				return
			}
			require.Contains(t, stderr, "--input and --input-url are mutually exclusive")
		})
	}
}

func TestProbeURIListOutputsProbeResult(t *testing.T) {
	code, stdout, stderr := runCLI(t,
		[]string{"probe", "--format", "uri-list", "--method", "tcp-connect", "--input", "-"},
		`{"name":"invalid","type":"ss"}`,
	)

	require.Equal(t, 1, code, stderr)
	require.Empty(t, stdout)
	require.Contains(t, stderr, "node_validation_failed")
}

func TestProbeJSONNodesOutputsProbeResult(t *testing.T) {
	code, stdout, stderr := runCLI(t,
		[]string{"probe", "--format", "json-nodes", "--method", "tcp-connect", "--input", "-"},
		`[{"name":"invalid","type":"ss","server":"example.com"}]`,
	)

	require.Equal(t, 1, code, stderr)
	require.Empty(t, stdout)
	require.Contains(t, stderr, "node_validation_failed")
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
			name: "probe",
			args: func(_ string, output string) []string {
				return []string{"probe", "--format", "uri-list", "--input", "-", "--output", output}
			},
			rec: &recordingEngine{probeResult: &sandrone.ProbeResult{Report: sandrone.Report{Kind: "probe"}}},
		},
		{
			name: "validate",
			args: func(_ string, output string) []string {
				return []string{"validate", "--format", "uri-list", "--input", "-", "--output", output}
			},
			rec: &recordingEngine{validateResult: &sandrone.ValidateResult{OK: true}},
		},
		{
			name: "inspect",
			args: func(_ string, output string) []string {
				return []string{"inspect", "--output", output}
			},
			rec: &recordingEngine{inspectResult: &sandrone.InspectResult{Capabilities: map[string]any{"ready": true}}},
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

func TestFileRenderWritesReportOutputFile(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "file.yaml")
	require.NoError(t, os.WriteFile(specPath, []byte("name: out.txt\nkind: static\nsource:\n  type: inline\n  content: body\n"), 0o644))
	reportPath := filepath.Join(dir, "nested", "report.json")

	code, stdout, stderr := runCLI(t,
		[]string{"--data-dir", filepath.Join(dir, "data"), "file", "render", specPath, "--report-output", reportPath},
		"",
	)

	require.Equal(t, 0, code, stderr)
	require.Equal(t, "body", stdout)
	reportBody, err := os.ReadFile(reportPath)
	require.NoError(t, err)
	require.Contains(t, string(reportBody), `"kind": "file"`)
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

func TestFileRenderLocalSpec(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "file.yaml")
	require.NoError(t, os.WriteFile(specPath, []byte(`
name: local.yaml
kind: static
source:
  type: inline
  content: |
    key: value
`), 0o644))

	code, stdout, stderr := runCLI(t,
		[]string{"--data-dir", filepath.Join(dir, "data"), "file", "render", specPath},
		"",
	)

	require.Equal(t, 0, code, stderr)
	require.Contains(t, stdout, "key: value")
	require.Empty(t, stderr)
}

func TestFileRenderLocalSpecRequiresCanonicalKindAndNewConfigWire(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{name: "missing kind", body: "name: bad.yaml\nsource: {type: inline, content: body}\n", want: "file kind is required"},
		{name: "case variant", body: "name: bad.yaml\nkind: Mihomo\nsource: {}\n", want: `file kind "Mihomo"`},
		{name: "legacy config", body: "name: bad.yaml\nkind: mihomo\nsource: {}\nconfig:\n  groups: []\n", want: "config.groups"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			specPath := filepath.Join(dir, "file.yaml")
			require.NoError(t, os.WriteFile(specPath, []byte(test.body), 0o644))

			code, _, stderr := runCLI(t,
				[]string{"--data-dir", filepath.Join(dir, "data"), "file", "render", specPath},
				"",
			)

			require.Equal(t, 1, code, stderr)
			require.Contains(t, stderr, test.want)
		})
	}
}

func TestFileRenderStoredSpec(t *testing.T) {
	dataDir := t.TempDir()
	engine := sandrone.NewWithFS(afero.NewBasePathFs(afero.NewOsFs(), dataDir))
	spec := sandrone.FileSpec{
		Name:   "stored.yaml",
		Kind:   sandrone.FileKindStatic,
		Source: sandrone.FileSource{Type: "inline", Content: "stored: true\n"},
	}
	require.NoError(t, engine.PutFile(context.Background(), spec))

	code, stdout, stderr := runCLI(t,
		[]string{"--data-dir", dataDir, "file", "render", "stored.yaml"},
		"",
	)

	require.Equal(t, 0, code, stderr)
	require.Empty(t, stderr)
	require.Contains(t, stdout, "stored: true")
}

func TestValidateMapsInputURLToRemoteParseRequest(t *testing.T) {
	rec := &recordingEngine{validateResult: &sandrone.ValidateResult{OK: true}}
	code, stdout, stderr := runCLI(t,
		[]string{"validate", "--input-url", "https://example.com/sub", "--user-agent", "ua", "--proxy", "http://127.0.0.1:8080", "--remote-timeout", "7s"},
		"stdin should not be read",
		WithEngineFactory(func(string) engine { return rec }),
	)
	require.Equal(t, 0, code, stderr)
	require.NotEmpty(t, stdout)
	require.Len(t, rec.parseRequests, 1)
	req := rec.parseRequests[0]
	require.Empty(t, req.Format)
	require.Empty(t, req.Content)
	require.NotNil(t, req.Remote)
	require.Equal(t, "https://example.com/sub", req.Remote.URL)
	require.Equal(t, "ua", req.Remote.UserAgent)
	require.Equal(t, "http://127.0.0.1:8080", req.Remote.Proxy)
	require.Equal(t, 7000, req.Remote.TimeoutMS)
}

func TestInspectCommandOutputsCapabilities(t *testing.T) {
	code, stdout, stderr := runCLI(t,
		[]string{"inspect"},
		"",
	)
	require.Equal(t, 0, code, stderr)
	require.Contains(t, stdout, "capabilities")
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

func TestServeCommandTreeIncludesSubcommands(t *testing.T) {
	cmd := NewRootCommand(WithEnv(map[string]string{}))
	serve, _, err := cmd.Find([]string{"serve"})
	require.NoError(t, err)
	require.Equal(t, "serve", serve.Name())
	for _, name := range []string{"http", "mcp", "all"} {
		child, _, err := serve.Find([]string{name})
		require.NoError(t, err)
		require.Equal(t, name, child.Name())
	}
}

func TestServeMCPHelpDocumentsSingleManagementSwitch(t *testing.T) {
	for _, command := range []string{"mcp", "all"} {
		t.Run(command, func(t *testing.T) {
			help := runHelp(t, "serve", command)

			require.Contains(t, help, "--allow-management-tools")
		})
	}
}

func TestServeHTTPPassesFlagsToRuntime(t *testing.T) {
	dataDir := t.TempDir()
	stopErr := errors.New("stop after runtime")
	var got app.Config

	code, _, stderr := runCLI(t,
		[]string{"--data-dir", dataDir, "serve", "--listen", "127.0.0.1:0", "--token", "secret", "http"},
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
}

func TestServeHTTPUsesTokenEnv(t *testing.T) {
	dataDir := t.TempDir()
	stopErr := errors.New("stop after runtime")
	var got app.Config

	code, _, stderr := runCLI(t,
		[]string{"--data-dir", dataDir, "serve", "http"},
		"",
		WithEnv(map[string]string{EnvToken: "env-secret"}),
		WithRuntimeFactory(func(cfg app.Config) (*app.Runtime, error) {
			got = cfg
			return nil, stopErr
		}),
	)

	require.Equal(t, 1, code)
	require.Contains(t, stderr, stopErr.Error())
	require.Equal(t, "env-secret", got.HTTP.Token)
}

func TestServeHTTPUsesWebUIStaticDirEnv(t *testing.T) {
	dataDir := t.TempDir()
	staticDir := t.TempDir()
	stopErr := errors.New("stop after runtime")
	var got app.Config

	code, _, stderr := runCLI(t,
		[]string{"--data-dir", dataDir, "serve", "http"},
		"",
		WithEnv(map[string]string{EnvWebUIStaticDir: staticDir}),
		WithRuntimeFactory(func(cfg app.Config) (*app.Runtime, error) {
			got = cfg
			return nil, stopErr
		}),
	)

	require.Equal(t, 1, code)
	require.Contains(t, stderr, stopErr.Error())
	require.Equal(t, staticDir, got.WebUI.StaticDir)
}

func TestServeHTTPWebUIStaticDirFlagOverridesEnv(t *testing.T) {
	dataDir := t.TempDir()
	envStaticDir := t.TempDir()
	flagStaticDir := t.TempDir()
	stopErr := errors.New("stop after runtime")
	var got app.Config

	code, _, stderr := runCLI(t,
		[]string{"--data-dir", dataDir, "serve", "--webui-static-dir", flagStaticDir, "http"},
		"",
		WithEnv(map[string]string{EnvWebUIStaticDir: envStaticDir}),
		WithRuntimeFactory(func(cfg app.Config) (*app.Runtime, error) {
			got = cfg
			return nil, stopErr
		}),
	)

	require.Equal(t, 1, code)
	require.Contains(t, stderr, stopErr.Error())
	require.Equal(t, flagStaticDir, got.WebUI.StaticDir)
}

func TestServeHTTPUsesDefaultListen(t *testing.T) {
	dataDir := t.TempDir()
	stopErr := errors.New("stop after runtime")
	var got app.Config

	code, _, stderr := runCLI(t,
		[]string{"--data-dir", dataDir, "serve", "http"},
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

func TestServePassesLogLevelToRuntime(t *testing.T) {
	dataDir := t.TempDir()
	stopErr := errors.New("stop after runtime")
	var got app.Config

	code, _, stderr := runCLI(t,
		[]string{"--data-dir", dataDir, "serve", "--log-level", "debug", "http"},
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

func TestServeRejectsInvalidLogLevel(t *testing.T) {
	dataDir := t.TempDir()

	code, stdout, stderr := runCLI(t,
		[]string{"--data-dir", dataDir, "serve", "--log-level", "trace", "http"},
		"",
	)

	require.Equal(t, 1, code)
	require.Empty(t, stdout)
	require.Contains(t, stderr, "unsupported log level")
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
	parseRequests   []sandrone.ParseRequest
	renderRequests  []sandrone.RenderRequest
	convertRequests []sandrone.ConvertRequest
	probeRequests   []sandrone.ProbeRequest
	parseResult     *sandrone.ParseResult
	renderResult    *sandrone.RenderResult
	convertResult   *sandrone.RenderResult
	probeResult     *sandrone.ProbeResult
	validateResult  *sandrone.ValidateResult
	inspectResult   *sandrone.InspectResult
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

func (e *recordingEngine) Probe(_ context.Context, req sandrone.ProbeRequest) (*sandrone.ProbeResult, error) {
	e.probeRequests = append(e.probeRequests, req)
	return e.probeResult, nil
}

func (e *recordingEngine) GetFile(context.Context, sandrone.FileRequest) (*sandrone.FileResult, error) {
	return nil, nil
}

func (e *recordingEngine) ValidateFile(context.Context, sandrone.FileRequest) (*sandrone.ValidateResult, error) {
	return &sandrone.ValidateResult{OK: true}, nil
}

func (e *recordingEngine) ValidateNodes(_ context.Context, req sandrone.ParseRequest) (*sandrone.ValidateResult, error) {
	e.parseRequests = append(e.parseRequests, req)
	if e.validateResult != nil {
		return e.validateResult, nil
	}
	return &sandrone.ValidateResult{OK: true}, nil
}

func (e *recordingEngine) Inspect(context.Context, sandrone.InspectRequest) (*sandrone.InspectResult, error) {
	if e.inspectResult != nil {
		return e.inspectResult, nil
	}
	return &sandrone.InspectResult{Capabilities: map[string]any{}}, nil
}
