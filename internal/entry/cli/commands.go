// Package cli implements the Cobra command-line entrypoint for Sandrone.
package cli

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/kuuvahki-labs/sandrone/pkg/sandrone"
)

const (
	inputFormatsHelp  = "input formats: uri, uri-list, base64, mihomo, sing-box, json-nodes"
	targetFormatsHelp = "target formats: base64, json-nodes, mihomo-proxies, shadowrocket-proxies, sing-box-outbounds, uri-list"
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
shadowrocket-proxies is a native [Proxy] section.

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
			engine, err := cfg.newEngine(cmd.Context())
			if err != nil {
				return err
			}
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
			engine, err := cfg.newEngine(cmd.Context())
			if err != nil {
				return err
			}
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
			engine, err := cfg.newEngine(cmd.Context())
			if err != nil {
				return err
			}
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
			engine, err := cfg.newEngine(cmd.Context())
			if err != nil {
				return err
			}
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
