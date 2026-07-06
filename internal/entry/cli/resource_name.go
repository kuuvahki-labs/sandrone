package cli

import (
	"fmt"
	"strings"
)

func validateRequiredPublicResourceName(label string, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("%s is required", label)
	}
	if strings.Contains(name, "/") || strings.Contains(name, "\\") || name == "." || name == ".." {
		return fmt.Errorf("%s must be a single path segment", label)
	}
	return nil
}
