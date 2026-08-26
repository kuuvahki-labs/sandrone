package internal_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProductionGoSourcePathOnlyIncludesBackendImplementation(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{path: "cmd/sandrone/main.go", want: true},
		{path: "internal/service/resources.go", want: true},
		{path: "pkg/sandrone/sandrone.go", want: true},
		{path: "internal/service/resources_test.go", want: false},
		{path: "internal/adapter/testdata/generated.go", want: false},
		{path: "web/app/root.tsx", want: false},
		{path: "web/eslint.config.js", want: false},
		{path: "data/files/convert.js", want: false},
		{path: "tools/generate.go", want: false},
	}

	for _, test := range tests {
		t.Run(test.path, func(t *testing.T) {
			if got := isProductionGoSourcePath(test.path); got != test.want {
				t.Fatalf("isProductionGoSourcePath(%q) = %t, want %t", test.path, got, test.want)
			}
		})
	}
}

var productionGoRoots = []string{"cmd", "internal", "pkg"}

func TestProductionGoFilesStayFocused(t *testing.T) {
	const maxLines = 650
	repositoryRoot := ".."

	for _, sourceRoot := range productionGoRoots {
		root := filepath.Join(repositoryRoot, sourceRoot)
		err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			rel, err := filepath.Rel(repositoryRoot, path)
			if err != nil {
				return err
			}
			rel = filepath.ToSlash(rel)
			if d.IsDir() {
				if shouldSkipProductionGoDir(rel) {
					return filepath.SkipDir
				}
				return nil
			}
			if !isProductionGoSourcePath(rel) {
				return nil
			}
			body, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			lines := strings.Count(string(body), "\n")
			if len(body) > 0 && body[len(body)-1] != '\n' {
				lines++
			}
			if lines > maxLines {
				t.Errorf("%s has %d lines; split production Go files above %d lines by responsibility", rel, lines, maxLines)
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walk production Go root %s: %v", sourceRoot, err)
		}
	}
}

func shouldSkipProductionGoDir(path string) bool {
	base := filepath.Base(path)
	return strings.HasPrefix(base, ".") || base == "testdata"
}

func isProductionGoSourcePath(path string) bool {
	if filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") || strings.Contains(path, "/testdata/") {
		return false
	}
	for _, root := range productionGoRoots {
		if strings.HasPrefix(path, root+"/") {
			return true
		}
	}
	return false
}

func TestProcessorDoesNotImportAdapter(t *testing.T) {
	root := "processor"
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if strings.Contains(string(b), "/internal/adapter/") {
			t.Fatalf("%s imports internal/adapter; service should compose adapter and processor dependencies", path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk processor: %v", err)
	}
}
