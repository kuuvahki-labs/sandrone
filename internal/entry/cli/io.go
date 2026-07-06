package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/kuuvahki-labs/sandrone/pkg/sandrone"
)

func readInput(inputPath string, stdin io.Reader) ([]byte, error) {
	if inputPath == "" || inputPath == "-" {
		return io.ReadAll(stdin)
	}
	return os.ReadFile(inputPath)
}

func writeOutput(outputPath string, stdout io.Writer, body []byte) error {
	if outputPath == "" || outputPath == "-" {
		_, err := stdout.Write(body)
		return err
	}
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(outputPath, body, 0o644)
}

func writeJSONOutput(outputPath string, stdout io.Writer, value any) error {
	body, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	body = append(body, '\n')
	return writeOutput(outputPath, stdout, body)
}

func writeReportOutput(outputPath string, report sandrone.Report) error {
	if outputPath == "" || outputPath == "-" {
		return fmt.Errorf("--report-output requires a file path other than -")
	}
	return writeJSONOutput(outputPath, io.Discard, report)
}

func validateOutputPaths(outputPath, reportOutputPath string) error {
	if reportOutputPath == "" {
		return nil
	}
	if reportOutputPath == "-" {
		return fmt.Errorf("--report-output requires a file path other than -")
	}
	if outputPath == "" || outputPath == "-" {
		return nil
	}

	mainPath, err := filepath.Abs(filepath.Clean(outputPath))
	if err != nil {
		return err
	}
	reportPath, err := filepath.Abs(filepath.Clean(reportOutputPath))
	if err != nil {
		return err
	}
	if mainPath == reportPath {
		return fmt.Errorf("--output and --report-output must refer to different files")
	}
	mainInfo, mainErr := os.Stat(mainPath)
	reportInfo, reportErr := os.Stat(reportPath)
	if mainErr == nil && reportErr == nil && os.SameFile(mainInfo, reportInfo) {
		return fmt.Errorf("--output and --report-output must refer to different files")
	}
	return nil
}

func renderFile(ctx context.Context, engine engine, arg string, req sandrone.FileRequest) (*sandrone.FileResult, error) {
	if isStoreNameCandidate(arg) {
		req.Name = arg
		result, err := engine.GetFile(ctx, req)
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
	return engine.GetFile(ctx, req)
}

func validateFile(ctx context.Context, engine engine, arg string, req sandrone.FileRequest) (*sandrone.ValidateResult, error) {
	if isStoreNameCandidate(arg) {
		req.Name = arg
		result, err := engine.ValidateFile(ctx, req)
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
	return engine.ValidateFile(ctx, req)
}

func readFileSpec(specPath string) (*sandrone.FileSpec, error) {
	body, err := os.ReadFile(specPath)
	if err != nil {
		return nil, err
	}
	var spec sandrone.FileSpec
	switch strings.ToLower(filepath.Ext(specPath)) {
	case ".json":
		decoder := json.NewDecoder(bytes.NewReader(body))
		decoder.UseNumber()
		if err := decoder.Decode(&spec); err != nil {
			return nil, err
		}
	case ".yaml", ".yml":
		var raw any
		if err := yaml.Unmarshal(body, &raw); err != nil {
			return nil, err
		}
		jsonBody, err := json.Marshal(yamlToJSONValue(raw))
		if err != nil {
			return nil, err
		}
		decoder := json.NewDecoder(bytes.NewReader(jsonBody))
		decoder.UseNumber()
		if err := decoder.Decode(&spec); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("file spec must be .json, .yaml, or .yml: %s", specPath)
	}
	return &spec, nil
}

func yamlToJSONValue(v any) any {
	switch value := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(value))
		for key, child := range value {
			out[key] = yamlToJSONValue(child)
		}
		return out
	case map[any]any:
		out := make(map[string]any, len(value))
		for key, child := range value {
			out[fmt.Sprint(key)] = yamlToJSONValue(child)
		}
		return out
	case []any:
		out := make([]any, len(value))
		for i, child := range value {
			out[i] = yamlToJSONValue(child)
		}
		return out
	default:
		return value
	}
}

func isLocalSpecPath(specPath string) bool {
	info, err := os.Stat(specPath)
	if err != nil || info.IsDir() {
		return false
	}
	switch strings.ToLower(filepath.Ext(specPath)) {
	case ".json", ".yaml", ".yml":
		return true
	default:
		return false
	}
}

func isStoreNameCandidate(name string) bool {
	return validateRequiredPublicResourceName("file name", name) == nil
}
