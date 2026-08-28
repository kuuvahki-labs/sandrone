package cli

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/kuuvahki-labs/sandrone/pkg/sandrone"
)

type diagnoseFailedError struct{}

func (diagnoseFailedError) Error() string { return "diagnosis failed" }

type diagnoseNodeFlags struct {
	format         string
	processorsPath string
	output         string
}

func newDiagnoseCommand(cfg *config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "diagnose",
		Short: "Diagnose recognized inputs through their declared pipeline",
		Long: `Diagnose identifies nodes, Subscription definitions, and FileSpecs and
executes the pipeline declared by that input. It never inserts a probe step;
network probing occurs only when a probe processor or script explicitly asks
for it. Diagnostic JSON can contain complete nodes, generated files, and probe
reports and must be treated as sensitive data.`,
		Args: cobra.NoArgs,
	}
	cmd.AddCommand(
		newDiagnoseInputCommand(cfg),
		newDiagnoseURLCommand(cfg),
		newDiagnoseSubscriptionCommand(cfg),
		newDiagnoseFileCommand(cfg),
	)
	return cmd
}

func newDiagnoseInputCommand(cfg *config) *cobra.Command {
	var kind string
	var flags diagnoseNodeFlags
	cmd := &cobra.Command{
		Use:   "input <path|->",
		Short: "Diagnose a local input or stdin",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			kind = strings.ToLower(strings.TrimSpace(kind))
			switch sandrone.DiagnoseInputKind(kind) {
			case sandrone.DiagnoseInputAuto, sandrone.DiagnoseInputNodes, sandrone.DiagnoseInputSubscription, sandrone.DiagnoseInputFile:
			default:
				return fmt.Errorf("--kind must be auto, nodes, subscription, or file")
			}
			if flags.format != "" && kind != string(sandrone.DiagnoseInputAuto) && kind != string(sandrone.DiagnoseInputNodes) {
				return fmt.Errorf("--format is only supported for nodes input")
			}
			if flags.processorsPath != "" && kind != string(sandrone.DiagnoseInputAuto) && kind != string(sandrone.DiagnoseInputNodes) {
				return fmt.Errorf("--processors is only supported for nodes input")
			}
			if args[0] == "-" && flags.processorsPath == "-" {
				return fmt.Errorf("input and --processors cannot both read from stdin")
			}
			body, err := readInput(args[0], cfg.stdin)
			if err != nil {
				return err
			}
			processors, err := readProcessorSpecs(flags.processorsPath, cfg.stdin)
			if err != nil {
				return err
			}
			return runDiagnose(cmd, cfg, sandrone.DiagnoseRequest{
				Kind: sandrone.DiagnoseInputKind(kind), Name: args[0], Format: flags.format,
				Content: body, Processors: processors,
			}, flags.output)
		},
	}
	cmd.Flags().StringVar(&kind, "kind", "auto", "input kind: auto, nodes, subscription, or file")
	addDiagnoseNodeFlags(cmd, &flags)
	return cmd
}

func newDiagnoseURLCommand(cfg *config) *cobra.Command {
	var flags diagnoseNodeFlags
	var userAgent string
	var proxyURL string
	var remoteTimeout time.Duration
	cmd := &cobra.Command{
		Use:   "url <url>",
		Short: "Diagnose a remote node document",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			processors, err := readProcessorSpecs(flags.processorsPath, cfg.stdin)
			if err != nil {
				return err
			}
			remote := &sandrone.RemoteInput{URL: args[0], UserAgent: userAgent, Proxy: proxyURL}
			if remoteTimeout > 0 {
				remote.TimeoutMS = int(remoteTimeout / time.Millisecond)
			}
			return runDiagnose(cmd, cfg, sandrone.DiagnoseRequest{
				Kind: sandrone.DiagnoseInputNodes, Name: args[0], Format: flags.format,
				Remote: remote, Processors: processors,
			}, flags.output)
		},
	}
	addDiagnoseNodeFlags(cmd, &flags)
	cmd.Flags().StringVar(&userAgent, "user-agent", "", "HTTP User-Agent for the remote fetch")
	cmd.Flags().StringVar(&proxyURL, "proxy", "", "HTTP, HTTPS, or SOCKS proxy URL for the remote fetch")
	cmd.Flags().DurationVar(&remoteTimeout, "remote-timeout", 0, "remote fetch timeout as a Go duration")
	return cmd
}

func newDiagnoseSubscriptionCommand(cfg *config) *cobra.Command {
	var output string
	var cacheMode string
	cmd := &cobra.Command{
		Use:   "subscription <name>",
		Short: "Diagnose a stored Subscription and its dependencies",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateRequiredPublicResourceName("subscription name", args[0]); err != nil {
				return err
			}
			cacheMode = strings.ToLower(strings.TrimSpace(cacheMode))
			switch sandrone.DiagnoseCacheMode(cacheMode) {
			case sandrone.DiagnoseCacheModeRefresh, sandrone.DiagnoseCacheModeReuse:
			default:
				return fmt.Errorf("--cache-mode must be refresh or reuse")
			}
			return runDiagnose(cmd, cfg, sandrone.DiagnoseRequest{
				Kind: sandrone.DiagnoseInputSubscription, SubscriptionName: args[0], CacheMode: sandrone.DiagnoseCacheMode(cacheMode),
			}, output)
		},
	}
	cmd.Flags().StringVar(&output, "output", "", "diagnostic JSON path, or stdout when empty or -")
	cmd.Flags().StringVar(&cacheMode, "cache-mode", string(sandrone.DiagnoseCacheModeRefresh), "cache mode: refresh or reuse")
	return cmd
}

func newDiagnoseFileCommand(cfg *config) *cobra.Command {
	var output string
	cmd := &cobra.Command{
		Use:   "file <name-or-spec-path>",
		Short: "Diagnose a stored or local FileSpec",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			arg := args[0]
			eng, err := cfg.newEngine(cmd.Context())
			if err != nil {
				return err
			}
			if isStoreNameCandidate(arg) {
				result, callErr := eng.Diagnose(cmd.Context(), sandrone.DiagnoseRequest{
					Kind: sandrone.DiagnoseInputFile, File: &sandrone.FileRequest{Name: arg},
				})
				if callErr != nil {
					return callErr
				}
				if result.Status != sandrone.DiagnoseStatusFailed || result.Error == nil || result.Error.Code != "file_input_not_found" || !isLocalSpecPath(arg) {
					return writeDiagnoseResult(cfg, output, result)
				}
			}
			spec, err := readFileSpec(arg)
			if err != nil {
				return err
			}
			result, err := eng.Diagnose(cmd.Context(), sandrone.DiagnoseRequest{
				Kind: sandrone.DiagnoseInputFile, Name: arg, File: &sandrone.FileRequest{Spec: spec},
			})
			if err != nil {
				return err
			}
			return writeDiagnoseResult(cfg, output, result)
		},
	}
	cmd.Flags().StringVar(&output, "output", "", "diagnostic JSON path, or stdout when empty or -")
	return cmd
}

func addDiagnoseNodeFlags(cmd *cobra.Command, flags *diagnoseNodeFlags) {
	cmd.Flags().StringVar(&flags.format, "format", "", inputFormatsHelp+"; overrides node format detection")
	cmd.Flags().StringVar(&flags.processorsPath, "processors", "", "JSON ProcessorSpec array path, or - for stdin")
	cmd.Flags().StringVar(&flags.output, "output", "", "diagnostic JSON path, or stdout when empty or -")
}

func readProcessorSpecs(path string, stdin io.Reader) ([]sandrone.ProcessorSpec, error) {
	if path == "" {
		return nil, nil
	}
	if path != "-" {
		switch strings.ToLower(filepath.Ext(path)) {
		case ".json":
		default:
			return nil, fmt.Errorf("processor spec must be .json: %s", path)
		}
	}
	body, err := readInput(path, stdin)
	if err != nil {
		return nil, err
	}
	var specs []sandrone.ProcessorSpec
	if err := decodeJSONResourceDefinition(body, &specs); err != nil {
		return nil, fmt.Errorf("decode processor specs: %w", err)
	}
	return specs, nil
}

func runDiagnose(cmd *cobra.Command, cfg *config, req sandrone.DiagnoseRequest, output string) error {
	eng, err := cfg.newEngine(cmd.Context())
	if err != nil {
		return err
	}
	result, err := eng.Diagnose(cmd.Context(), req)
	if err != nil {
		return err
	}
	return writeDiagnoseResult(cfg, output, result)
}

func writeDiagnoseResult(cfg *config, output string, result *sandrone.DiagnoseResult) error {
	if err := writeSensitiveJSONOutput(output, cfg.stdout, result); err != nil {
		return err
	}
	if result != nil && result.Status == sandrone.DiagnoseStatusFailed {
		return diagnoseFailedError{}
	}
	return nil
}
