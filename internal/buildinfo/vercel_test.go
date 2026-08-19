package buildinfo

import (
	"compress/gzip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVerifyVercelAssetsRequiresWebAndCatalog(t *testing.T) {
	root := newVercelAssetFixture(t)
	verify := filepath.Join(root, "scripts", "verify-vercel-assets.sh")

	output, err := runCommand(root, "sh", verify)
	if err == nil || !strings.Contains(string(output), "static/index.html is missing or empty") {
		t.Fatalf("missing Web UI check: err=%v\n%s", err, output)
	}

	writeTestFile(t, filepath.Join(root, "internal", "entry", "webui", "static", "index.html"), "<!doctype html>\n")
	output, err = runCommand(root, "sh", verify)
	if err == nil || !strings.Contains(string(output), "catalog.json.gz is missing or empty") {
		t.Fatalf("missing catalog check: err=%v\n%s", err, output)
	}

	writeTestFile(t, filepath.Join(root, "internal", "service", "catalog_builtin", "catalog.json.gz"), "not gzip\n")
	output, err = runCommand(root, "sh", verify)
	if err == nil || !strings.Contains(string(output), "catalog.json.gz is not valid gzip") {
		t.Fatalf("invalid catalog check: err=%v\n%s", err, output)
	}
}

func TestBuildVercelAssetsGeneratesAndVerifiesBothAssets(t *testing.T) {
	root := newVercelAssetFixture(t)
	makeLog := filepath.Join(root, "make.log")
	catalogFixture := filepath.Join(root, "catalog-fixture.json.gz")
	writeTestGzip(t, catalogFixture, []byte(`{"mihomo":[],"sing-box":[],"shadowrocket":[]}`))

	makeStub := filepath.Join(root, "make-stub.sh")
	writeTestFile(t, makeStub, `#!/bin/sh
set -eu
printf '%s\n' "$*" > "$MAKE_LOG"
mkdir -p internal/entry/webui/static internal/service/catalog_builtin
printf '%s\n' '<!doctype html>' > internal/entry/webui/static/index.html
cp "$CATALOG_FIXTURE" internal/service/catalog_builtin/catalog.json.gz
`)
	if err := os.Chmod(makeStub, 0o700); err != nil {
		t.Fatal(err)
	}

	output, err := runCommandEnv(root, []string{
		"MAKE=" + makeStub,
		"MAKE_LOG=" + makeLog,
		"CATALOG_FIXTURE=" + catalogFixture,
	}, "sh", filepath.Join(root, "scripts", "build-vercel-assets.sh"))
	if err != nil {
		t.Fatalf("build Vercel assets: %v\n%s", err, output)
	}
	logBody, err := os.ReadFile(makeLog)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := strings.TrimSpace(string(logBody)), "ruleset-catalog build-webui"; got != want {
		t.Fatalf("make targets = %q, want %q", got, want)
	}
}

func TestVercelWorkflowUsesPrebuiltAssetPipeline(t *testing.T) {
	root := filepath.Join("..", "..")
	workflow, err := os.ReadFile(filepath.Join(root, ".github", "workflows", "ci.yml"))
	if err != nil {
		t.Fatal(err)
	}
	content := string(workflow)
	for _, want := range []string{
		"  vercel:\n",
		"name: Vercel deployment",
		"if: github.ref_type == 'tag' || (github.event_name == 'push' && github.ref == 'refs/heads/main')",
		"needs:\n      - go\n      - web",
		"group: vercel-${{ github.ref }}",
		"VERCEL_ENVIRONMENT: ${{ github.ref_type == 'tag' && 'production' || 'preview' }}",
		"VERCEL_ORG_ID: ${{ secrets.VERCEL_ORG_ID }}",
		"VERCEL_PROJECT_ID: ${{ secrets.VERCEL_PROJECT_ID }}",
		"VERCEL_TOKEN: ${{ secrets.VERCEL_TOKEN }}",
		"npm install --global \"vercel@${VERCEL_CLI_VERSION}\"",
		"./scripts/build-vercel-assets.sh",
		"./scripts/verify-vercel-assets.sh",
		"vercel build --standalone --prod",
		"vercel build --standalone --token",
		"vercel deploy --prebuilt --prod",
		"vercel deploy --prebuilt --archive=tgz",
	} {
		if !strings.Contains(content, want) {
			t.Errorf("Vercel workflow does not contain %q", want)
		}
	}
	buildAssetsAt := strings.Index(content, "./scripts/build-vercel-assets.sh")
	verifyAssetsAt := strings.Index(content, "./scripts/verify-vercel-assets.sh")
	vercelBuildAt := strings.Index(content, "vercel build --standalone --prod")
	vercelDeployAt := strings.Index(content, "vercel deploy --prebuilt --prod")
	if buildAssetsAt < 0 || !(buildAssetsAt < verifyAssetsAt && verifyAssetsAt < vercelBuildAt && vercelBuildAt < vercelDeployAt) {
		t.Error("Vercel workflow must generate, verify, build, and deploy in order")
	}
	if strings.Contains(content, "vercel@latest") {
		t.Error("Vercel workflow must pin the CLI version")
	}
	if strings.Contains(content, "github.ref == 'refs/heads/main' && 'production'") {
		t.Error("Vercel workflow must reserve Production deployments for tags")
	}
	if strings.Contains(content, "vercel build --prod --token") || strings.Contains(content, "vercel build --token") {
		t.Error("Vercel workflow must inline Go bootstrap output with standalone builds")
	}
}

func newVercelAssetFixture(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	fixture := t.TempDir()
	for _, name := range []string{
		filepath.Join("scripts", "build-vercel-assets.sh"),
		filepath.Join("scripts", "verify-vercel-assets.sh"),
	} {
		body, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			t.Fatal(err)
		}
		target := filepath.Join(fixture, name)
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, body, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	return fixture
}

func writeTestFile(t *testing.T, name, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(name), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(name, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func writeTestGzip(t *testing.T, name string, body []byte) {
	t.Helper()
	file, err := os.Create(name)
	if err != nil {
		t.Fatal(err)
	}
	compressed := gzip.NewWriter(file)
	if _, err := compressed.Write(body); err != nil {
		t.Fatal(err)
	}
	if err := compressed.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}
