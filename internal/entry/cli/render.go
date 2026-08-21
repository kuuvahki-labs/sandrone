package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/kuuvahki-labs/sandrone/pkg/sandrone"
)

type renderFlags struct {
	args         []string
	refresh      bool
	output       string
	reportOutput string
}

func newRenderCommand(cfg *config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "render",
		Short: "Render declared resources to their final output",
		Long: `Render executes a stored Subscription or a stored/local FileSpec and writes
the final client-facing body. Use convert for one-shot node format conversion,
or diagnose when you need pipeline stages, warnings, and probe traces.`,
		Args: cobra.NoArgs,
	}
	cmd.AddCommand(
		newRenderSubscriptionCommand(cfg),
		newRenderFileCommand(cfg),
	)
	return cmd
}

func newRenderSubscriptionCommand(cfg *config) *cobra.Command {
	var format string
	var flags renderFlags
	cmd := &cobra.Command{
		Use:   "subscription <name>",
		Short: "Render a stored Subscription to a node format",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, positional []string) error {
			if err := validateRequiredPublicResourceName("subscription name", positional[0]); err != nil {
				return err
			}
			format = strings.TrimSpace(format)
			if format == "" {
				return fmt.Errorf("--format is required")
			}
			requestArgs, err := parseRenderArgs(flags.args)
			if err != nil {
				return err
			}
			if err := validateOutputPaths(flags.output, flags.reportOutput); err != nil {
				return err
			}
			eng, err := cfg.newEngine(cmd.Context())
			if err != nil {
				return err
			}
			result, err := eng.RenderSubscriptionRequest(cmd.Context(), sandrone.SubscriptionRenderRequest{
				Name: positional[0], Format: format,
				Request: sandrone.RequestInfo{Args: requestArgs}, Refresh: flags.refresh,
			})
			if err != nil {
				return err
			}
			return writeRenderResult(cfg, flags, result.Body, result.Report)
		},
	}
	cmd.Flags().StringVar(&format, "format", "", targetFormatsHelp)
	addRenderFlags(cmd, &flags)
	return cmd
}

func newRenderFileCommand(cfg *config) *cobra.Command {
	var flags renderFlags
	cmd := &cobra.Command{
		Use:   "file <name-or-spec.json>",
		Short: "Render a stored or local FileSpec",
		Long: `Render a complete client file from a stored FileSpec name or a local JSON
FileSpec. Safe resource names are looked up in the Store first; an existing
local .json path is used as a fallback when that stored resource is absent.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, positional []string) error {
			requestArgs, err := parseRenderArgs(flags.args)
			if err != nil {
				return err
			}
			if err := validateOutputPaths(flags.output, flags.reportOutput); err != nil {
				return err
			}
			eng, err := cfg.newEngine(cmd.Context())
			if err != nil {
				return err
			}
			result, err := renderFile(cmd.Context(), eng, positional[0], sandrone.FileRequest{
				Request: sandrone.RequestInfo{Args: requestArgs}, Refresh: flags.refresh,
			})
			if err != nil {
				return err
			}
			return writeRenderResult(cfg, flags, result.Content, result.Report)
		},
	}
	addRenderFlags(cmd, &flags)
	return cmd
}

func addRenderFlags(cmd *cobra.Command, flags *renderFlags) {
	cmd.Flags().StringArrayVar(&flags.args, "arg", nil, "request argument as key=value; may be repeated")
	cmd.Flags().BoolVar(&flags.refresh, "refresh", false, "bypass the saved resource render cache")
	cmd.Flags().StringVar(&flags.output, "output", "", "output file path, or stdout when empty or -")
	cmd.Flags().StringVar(&flags.reportOutput, "report-output", "", "write the complete render report as pretty JSON to a file")
}

func parseRenderArgs(values []string) (map[string]string, error) {
	if len(values) == 0 {
		return map[string]string{}, nil
	}
	args := make(map[string]string, len(values))
	for _, value := range values {
		key, item, ok := strings.Cut(value, "=")
		key = strings.TrimSpace(key)
		if !ok || key == "" {
			return nil, fmt.Errorf("--arg must use a non-empty key=value")
		}
		args[key] = item
	}
	return args, nil
}

func renderFile(ctx context.Context, eng engine, arg string, req sandrone.FileRequest) (*sandrone.FileResult, error) {
	if isStoreNameCandidate(arg) {
		req.Name = arg
		result, err := eng.GetFile(ctx, req)
		if err == nil {
			return result, nil
		}
		if !errors.Is(err, os.ErrNotExist) || !isLocalSpecPath(arg) {
			return nil, err
		}
	}
	spec, err := readFileSpec(arg)
	if err != nil {
		return nil, err
	}
	req.Name = ""
	req.Spec = spec
	return eng.GetFile(ctx, req)
}

func writeRenderResult(cfg *config, flags renderFlags, body []byte, report sandrone.Report) error {
	if err := writeOutput(flags.output, cfg.stdout, body); err != nil {
		return err
	}
	if flags.reportOutput != "" {
		return writeReportOutput(flags.reportOutput, report)
	}
	return nil
}
