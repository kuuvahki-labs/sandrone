package cli

import (
	"bytes"
	"crypto/rand"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/kuuvahki-labs/sandrone/internal/jsonvalue"
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
	return writeAtomicOutput(outputPath, body, 0o644)
}

// Stage the complete file in the destination directory. A failed write or rename
// leaves an existing output intact; each output is replaced independently.
func writeAtomicOutput(outputPath string, body []byte, mode os.FileMode) error {
	if info, err := os.Stat(outputPath); err == nil {
		if !info.Mode().IsRegular() {
			return fmt.Errorf("output path is not a regular file: %s", outputPath)
		}
		// Do not make an existing private result more permissive.
		mode &= info.Mode().Perm()
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return err
	}
	// Open with the final mode so the kernel applies the caller's umask. Chmod
	// after CreateTemp would bypass it and could expose newly written credentials.
	file, err := os.OpenFile(filepath.Join(filepath.Dir(outputPath), ".sandrone-output-"+rand.Text()), os.O_RDWR|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(file.Name()) }()
	if _, err := file.Write(body); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(file.Name(), outputPath)
}

func writeJSONOutput(outputPath string, stdout io.Writer, value any) error {
	body, err := json.Marshal(value, jsontext.WithIndent("  "))
	if err != nil {
		return err
	}
	body = append(body, '\n')
	return writeOutput(outputPath, stdout, body)
}

func writeSensitiveJSONOutput(outputPath string, stdout io.Writer, value any) error {
	body, err := json.Marshal(value, jsontext.WithIndent("  "))
	if err != nil {
		return err
	}
	body = append(body, '\n')
	if outputPath == "" || outputPath == "-" {
		_, err = stdout.Write(body)
		return err
	}
	return writeAtomicOutput(outputPath, body, 0o600)
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
	if err := json.Unmarshal(trimmed, out, jsonvalue.PreserveNumbers, json.RejectUnknownMembers(true)); err != nil {
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
