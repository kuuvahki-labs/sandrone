// Package cli implements the Cobra command-line entrypoint for Sandrone.
package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/kuuvahki-labs/sandrone/pkg/sandrone"
)

const (
	inputFormatsHelp  = "input formats: uri, uri-list, base64, mihomo, sing-box, json-nodes"
	targetFormatsHelp = "target formats: base64, json-nodes, mihomo-proxies, shadowrocket-proxies, sing-box-outbounds, uri-list"
	probeMethodsHelp  = "probe methods: tcp-connect, udp-ntp, url-test"
)

type remoteInputFlags struct {
	inputURL      string
	userAgent     string
	proxy         string
	remoteTimeout time.Duration
}

func (f remoteInputFlags) remoteInput() *sandrone.RemoteInput {
	if f.inputURL == "" {
		return nil
	}
	remote := &sandrone.RemoteInput{
		URL:       f.inputURL,
		UserAgent: f.userAgent,
		Proxy:     f.proxy,
	}
	if f.remoteTimeout > 0 {
		remote.TimeoutMS = int(f.remoteTimeout / time.Millisecond)
	}
	return remote
}

func addRemoteInputFlags(flags *pflag.FlagSet, remote *remoteInputFlags) {
	flags.StringVar(&remote.inputURL, "input-url", "", "remote input URL to fetch instead of local input")
	flags.StringVar(&remote.userAgent, "user-agent", "", "HTTP User-Agent for remote input fetch")
	flags.StringVar(&remote.proxy, "proxy", "", "HTTP, HTTPS, or SOCKS proxy URL for remote input fetch")
	flags.DurationVar(&remote.remoteTimeout, "remote-timeout", 0, "remote input fetch timeout as Go duration, for example 3s or 500ms")
}

func rejectInputAndInputURL(input string, inputChanged bool, remote *sandrone.RemoteInput) error {
	if remote == nil {
		return nil
	}
	if inputChanged || (input != "" && input != "-") {
		return fmt.Errorf("--input and --input-url are mutually exclusive")
	}
	return nil
}

func newConvertCommand(cfg *config) *cobra.Command {
	var fromFormat string
	var toFormat string
	var input string
	var output string
	var reportOutput string
	var remote remoteInputFlags

	cmd := &cobra.Command{
		Use:   "convert --to <fmt> (--input <path|-> | --input-url <url>)",
		Short: "Convert input nodes directly to a target format",
		Long: `Convert reads nodes in one format and renders them directly to another format.

Use --input - to read from stdin, or use --input-url to fetch a remote
config input. When --input-url is used, --from can be omitted so that the
service auto-detects the source format. Omit --output to write the rendered
body to stdout. Use --report-output with a file path to write the complete
conversion report as pretty JSON.

json-nodes is the normalized node IR, useful for debugging or exporting parsed
nodes. mihomo-proxies and sing-box-outbounds are structured node fragments;
shadowrocket-proxies is a native [Proxy] section. Use file render when you
need a complete client configuration file.

` + inputFormatsHelp + `
` + targetFormatsHelp,
		Example: `  sandrone convert --from uri-list --to mihomo-proxies --input sub.txt
  sandrone convert --to json-nodes --input-url https://example.com/sub --remote-timeout 5s
  sandrone convert --from base64 --to sing-box-outbounds --input - --output out.json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			remoteInput := remote.remoteInput()
			if remoteInput == nil && fromFormat == "" {
				return fmt.Errorf("--from is required unless --input-url is set")
			}
			if toFormat == "" {
				return fmt.Errorf("--to is required")
			}
			if err := rejectInputAndInputURL(input, cmd.Flags().Changed("input"), remoteInput); err != nil {
				return err
			}
			if err := validateOutputPaths(output, reportOutput); err != nil {
				return err
			}
			var body []byte
			if remoteInput == nil {
				var err error
				body, err = readInput(input, cfg.stdin)
				if err != nil {
					return err
				}
			}
			engine := cfg.engineFactory(cfg.dataDir)
			rendered, err := engine.Convert(cmd.Context(), sandrone.ConvertRequest{
				FromFormat: fromFormat,
				ToFormat:   toFormat,
				Content:    body,
				Remote:     remoteInput,
			})
			if err != nil {
				return err
			}
			if err := writeOutput(output, cfg.stdout, rendered.Body); err != nil {
				return err
			}
			if reportOutput != "" {
				return writeReportOutput(reportOutput, rendered.Report)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&fromFormat, "from", "", inputFormatsHelp+"; required unless --input-url is set")
	cmd.Flags().StringVar(&toFormat, "to", "", targetFormatsHelp)
	cmd.Flags().StringVar(&input, "input", "-", "input file path, or - to read from stdin")
	addRemoteInputFlags(cmd.Flags(), &remote)
	cmd.Flags().StringVar(&output, "output", "", "output file path, or stdout when empty or -")
	cmd.Flags().StringVar(&reportOutput, "report-output", "", "write the complete conversion report as pretty JSON to a file")
	return cmd
}

func newProbeCommand(cfg *config) *cobra.Command {
	var method string
	var format string
	var input string
	var output string
	var core string
	var url string
	var ntpServer string
	var expectedStatus string
	var timeout time.Duration
	var attempts int
	var concurrency int
	var cacheTTL int
	var remote remoteInputFlags

	cmd := &cobra.Command{
		Use:   "probe [--format <fmt>] (--input <path|-> | --input-url <url>)",
		Short: "Probe node health",
		Long: `Probe reads nodes and checks whether each node is alive.

--format selects the input node format; uri-list is the default and is a
good fit for URI lists. Use base64 for encoded node lists. json-nodes is useful
when replaying normalized nodes from convert --to json-nodes. When --input-url
is used, --format may be omitted so that the service auto-detects the source.

The default method is url-test through the sing-box core. url-test asks a
client core to run an HTTP URL health check. --core, --url, and
--expected-status are only used by url-test. --ntp-server is only used by
udp-ntp. tcp-connect does not use a client core.

Use Go duration values such as 3s or 500ms for --timeout. Set --attempts or
--concurrency to 0 to use the service default. Set --cache-ttl to 0 to use the
service default; caching is disabled when both values are 0. Use
--remote-timeout for remote input fetches.

` + probeMethodsHelp,
		Example: `  sandrone probe --format uri-list --method url-test --input sub.txt --timeout 3s --concurrency 10
  sandrone probe --method url-test --input-url https://example.com/sub --remote-timeout 5s
  sandrone probe --format uri-list --method udp-ntp --input nodes.txt --ntp-server time.apple.com --timeout 3s --cache-ttl 300
  sandrone probe --format json-nodes --method url-test --core sing-box --input nodes.json --url https://cp.cloudflare.com --expected-status 204 --timeout 5s`,
		RunE: func(cmd *cobra.Command, args []string) error {
			remoteInput := remote.remoteInput()
			if err := rejectInputAndInputURL(input, cmd.Flags().Changed("input"), remoteInput); err != nil {
				return err
			}
			resolvedFormat := format
			if remoteInput != nil && !cmd.Flags().Changed("format") {
				resolvedFormat = ""
			}
			nodeInput := sandrone.NodeInput{
				Name:   "cli",
				Type:   "inline",
				Format: resolvedFormat,
			}
			if remoteInput != nil {
				nodeInput = sandrone.NodeInput{
					Name:      "cli",
					Type:      "remote",
					URL:       remoteInput.URL,
					Format:    resolvedFormat,
					UserAgent: remoteInput.UserAgent,
					Proxy:     remoteInput.Proxy,
					TimeoutMS: remoteInput.TimeoutMS,
				}
			} else {
				body, err := readInput(input, cfg.stdin)
				if err != nil {
					return err
				}
				nodeInput.Content = string(body)
			}
			req := sandrone.ProbeRequest{
				Input:           nodeInput,
				Method:          normalizeProbeMethod(method),
				Core:            core,
				URL:             url,
				NTPServer:       ntpServer,
				ExpectedStatus:  expectedStatus,
				Attempts:        attempts,
				Concurrency:     concurrency,
				CacheTTLSeconds: cacheTTL,
			}
			if timeout > 0 {
				req.TimeoutMS = int(timeout / time.Millisecond)
			}
			engine := cfg.engineFactory(cfg.dataDir)
			probed, err := engine.Probe(cmd.Context(), req)
			if err != nil {
				return err
			}
			out, err := json.MarshalIndent(probed, "", "  ")
			if err != nil {
				return err
			}
			out = append(out, '\n')
			if err := writeOutput(output, cfg.stdout, out); err != nil {
				return err
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&method, "method", "url-test", probeMethodsHelp)
	cmd.Flags().StringVar(&format, "format", "uri-list", "input format; defaults to uri-list; examples: uri-list, base64, json-nodes")
	cmd.Flags().StringVar(&input, "input", "-", "input file path, or - to read from stdin")
	addRemoteInputFlags(cmd.Flags(), &remote)
	cmd.Flags().StringVar(&output, "output", "", "output file path, or stdout when empty or -")
	cmd.Flags().StringVar(&core, "core", "sing-box", "core name for url-test or udp-ntp, for example mihomo or sing-box")
	cmd.Flags().StringVar(&url, "url", "", "HTTP URL target for url-test, for example https://cp.cloudflare.com")
	cmd.Flags().StringVar(&ntpServer, "ntp-server", "", "NTP server for udp-ntp, for example time.apple.com")
	cmd.Flags().StringVar(&expectedStatus, "expected-status", "", "expected HTTP status or range for url-test, for example 204 or 200-299")
	cmd.Flags().DurationVar(&timeout, "timeout", 0, "per-node timeout as Go duration, for example 3s or 500ms")
	cmd.Flags().IntVar(&attempts, "attempts", 0, "per-node attempts; 0 uses service default")
	cmd.Flags().IntVar(&concurrency, "concurrency", 0, "maximum concurrent probes; 0 uses service default")
	cmd.Flags().IntVar(&cacheTTL, "cache-ttl", 0, "cache TTL in seconds; 0 uses the service default")
	return cmd
}

func normalizeProbeMethod(method string) sandrone.ProbeMethod {
	method = strings.ToLower(strings.TrimSpace(method))
	method = strings.ReplaceAll(method, "-", "_")
	return sandrone.ProbeMethod(method)
}

func newValidateCommand(cfg *config) *cobra.Command {
	var format string
	var input string
	var fileTarget string
	var output string
	var remote remoteInputFlags
	cmd := &cobra.Command{
		Use:   "validate ((--format <fmt> --input <path|->) | --input-url <url> | --file <name-or-spec-path>)",
		Short: "Validate nodes or a file spec through the service flow",
		Long: `Validate checks nodes or a FileSpec without rendering the final output.

There are three modes:

  1. Node validation from local input: pass --format and --input.
  2. Node validation from remote input: pass --input-url and optional remote
     fetch flags; --format may be omitted for auto-detect.
  3. FileSpec validation: pass --file with a stored file name or a local
     .json, .yaml, or .yml FileSpec path.

When --file is set, --format is not required. The result is written as JSON to
stdout.

` + inputFormatsHelp + ``,
		Example: `  sandrone validate --format uri-list --input nodes.txt
  sandrone validate --input-url https://example.com/sub --remote-timeout 5s
  sandrone validate --format json-nodes --input nodes.json
  sandrone validate --file mihomo.yaml`,
		RunE: func(cmd *cobra.Command, args []string) error {
			engine := cfg.engineFactory(cfg.dataDir)
			remoteInput := remote.remoteInput()
			if fileTarget != "" {
				if remoteInput != nil {
					return fmt.Errorf("--file and --input-url are mutually exclusive")
				}
				result, err := validateFile(cmd.Context(), engine, fileTarget, sandrone.FileRequest{})
				if err != nil {
					return err
				}
				out, err := json.MarshalIndent(result, "", "  ")
				if err != nil {
					return err
				}
				out = append(out, '\n')
				return writeOutput(output, cfg.stdout, out)
			}
			if remoteInput == nil && format == "" {
				return fmt.Errorf("--format is required unless --file or --input-url is set")
			}
			if err := rejectInputAndInputURL(input, cmd.Flags().Changed("input"), remoteInput); err != nil {
				return err
			}
			req := sandrone.ParseRequest{Format: format, Remote: remoteInput}
			if remoteInput == nil {
				body, err := readInput(input, cfg.stdin)
				if err != nil {
					return err
				}
				req.Content = body
			}
			result, err := engine.ValidateNodes(cmd.Context(), req)
			if err != nil {
				return err
			}
			out, err := json.MarshalIndent(result, "", "  ")
			if err != nil {
				return err
			}
			out = append(out, '\n')
			return writeOutput(output, cfg.stdout, out)
		},
	}
	cmd.Flags().StringVar(&format, "format", "", inputFormatsHelp+"; required unless --file or --input-url is set")
	cmd.Flags().StringVar(&input, "input", "-", "input file path, or - to read from stdin")
	addRemoteInputFlags(cmd.Flags(), &remote)
	cmd.Flags().StringVar(&fileTarget, "file", "", "stored file name or local FileSpec path ending in .json, .yaml, or .yml")
	cmd.Flags().StringVar(&output, "output", "", "output file path, or stdout when empty or -")
	return cmd
}

func newInspectCommand(cfg *config) *cobra.Command {
	var output string
	cmd := &cobra.Command{
		Use:   "inspect",
		Short: "Inspect the lightweight runtime summary",
		Long: `Inspect prints the current runtime summary as JSON.

Use it to see available format and processor names, file kinds, probe backends,
and the current store summary for the selected data directory. Use capability
commands for detailed format contracts.`,
		Example: `  sandrone inspect`,
		RunE: func(cmd *cobra.Command, args []string) error {
			engine := cfg.engineFactory(cfg.dataDir)
			result, err := engine.Inspect(cmd.Context())
			if err != nil {
				return err
			}
			out, err := json.MarshalIndent(result, "", "  ")
			if err != nil {
				return err
			}
			out = append(out, '\n')
			return writeOutput(output, cfg.stdout, out)
		},
	}
	cmd.Flags().StringVar(&output, "output", "", "output file path, or stdout when empty or -")
	return cmd
}

func newCapabilityCommand(cfg *config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "capability",
		Short: "Inspect detailed format capability contracts",
	}
	cmd.AddCommand(
		newCapabilityFormatsCommand(cfg),
		newCapabilityFormatCommand(cfg),
	)
	return cmd
}

func newCapabilityFormatsCommand(cfg *config) *cobra.Command {
	var output string
	cmd := &cobra.Command{
		Use:   "formats",
		Short: "List parse and render format capability summaries",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			engine := cfg.engineFactory(cfg.dataDir)
			result, err := engine.ListFormatCapabilities(cmd.Context())
			if err != nil {
				return err
			}
			return writeJSONOutput(output, cfg.stdout, result)
		},
	}
	cmd.Flags().StringVar(&output, "output", "", "output file path, or stdout when empty or -")
	return cmd
}

func newCapabilityFormatCommand(cfg *config) *cobra.Command {
	var output string
	cmd := &cobra.Command{
		Use:   "format <parse|render> <format>",
		Short: "Show one exact format capability contract",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			engine := cfg.engineFactory(cfg.dataDir)
			result, err := engine.GetFormatCapability(cmd.Context(), sandrone.FormatCapabilityRequest{
				Direction: sandrone.CapabilityDirection(args[0]),
				Format:    args[1],
			})
			if err != nil {
				return err
			}
			return writeJSONOutput(output, cfg.stdout, result)
		},
	}
	cmd.Flags().StringVar(&output, "output", "", "output file path, or stdout when empty or -")
	return cmd
}

func newFileCommand(cfg *config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "file",
		Short: "Generate complete client files",
		Long: `Generate complete client files from stored or local FileSpec definitions.

Use file render when you need a full mihomo, sing-box, or Shadowrocket configuration file.
For node output only, use convert with targets such as mihomo-proxies,
sing-box-outbounds, or shadowrocket-proxies.`,
		Example: `  sandrone file render ./mihomo-file.yaml --output mihomo.yaml`,
	}
	cmd.AddCommand(newFileRenderCommand(cfg))
	return cmd
}

func newFileRenderCommand(cfg *config) *cobra.Command {
	var output string
	var reportOutput string

	cmd := &cobra.Command{
		Use:   "render <name-or-spec-path>",
		Short: "Render a stored file spec or local YAML/JSON file spec",
		Long: `Render a complete client file from a FileSpec.

<name-or-spec-path> can be a stored file name or a local FileSpec path ending in
.json, .yaml, or .yml. Stored names are tried first for safe relative names; if
the stored file is not found and the local path exists, Sandrone reads the local
FileSpec instead.

The rendered body is written to stdout unless --output is set. Use
--report-output with a file path to write the complete render report as pretty
JSON.`,
		Example: `  sandrone file render mihomo.yaml
  sandrone file render ./mihomo-file.yaml --output mihomo.yaml`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutputPaths(output, reportOutput); err != nil {
				return err
			}
			engine := cfg.engineFactory(cfg.dataDir)
			result, err := renderFile(cmd.Context(), engine, args[0], sandrone.FileRequest{})
			if err != nil {
				return err
			}
			if err := writeOutput(output, cfg.stdout, result.Content); err != nil {
				return err
			}
			if reportOutput != "" {
				return writeReportOutput(reportOutput, result.Report)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&output, "output", "", "output file path, or stdout when empty or -")
	cmd.Flags().StringVar(&reportOutput, "report-output", "", "write the complete render report as pretty JSON to a file")
	return cmd
}
