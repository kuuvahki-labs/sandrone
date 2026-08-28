package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

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

func writeSensitiveJSONOutput(outputPath string, stdout io.Writer, value any) error {
	body, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	body = append(body, '\n')
	if outputPath == "" || outputPath == "-" {
		_, err = stdout.Write(body)
		return err
	}
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return err
	}
	file, err := os.OpenFile(outputPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	if err := file.Chmod(0o600); err != nil {
		_ = file.Close()
		return err
	}
	if _, err := file.Write(body); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
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

func readFileSpec(specPath string) (*sandrone.FileSpec, error) {
	body, err := os.ReadFile(specPath)
	if err != nil {
		return nil, err
	}
	var spec sandrone.FileSpec
	switch strings.ToLower(filepath.Ext(specPath)) {
	case ".json":
		if err := decodeJSONResourceDefinition(body, &spec); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("file spec must be .json: %s", specPath)
	}
	return &spec, nil
}

func decodeJSONResourceDefinition[T any](body []byte, out *T) error {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		return fmt.Errorf("JSON resource definition is empty")
	}
	decoder := json.NewDecoder(bytes.NewReader(trimmed))
	decoder.UseNumber()
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(out); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		if err == nil {
			return fmt.Errorf("multiple JSON values are not supported")
		}
		return err
	}
	return nil
}

func isLocalSpecPath(specPath string) bool {
	info, err := os.Stat(specPath)
	if err != nil || info.IsDir() {
		return false
	}
	switch strings.ToLower(filepath.Ext(specPath)) {
	case ".json":
		return true
	default:
		return false
	}
}

func isStoreNameCandidate(name string) bool {
	return validateRequiredPublicResourceName("file name", name) == nil
}
