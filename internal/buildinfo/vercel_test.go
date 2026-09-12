package buildinfo

import (
	"compress/gzip"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestVerifyVercelAssetsRequiresWebAndCatalog(t *testing.T) {
	root := newVercelAssetFixture(t)
	verify := filepath.Join(root, "scripts", "vercel-assets.sh")

	output, err := runCommand(root, "sh", verify, "verify")
	if err == nil || !strings.Contains(string(output), "static/index.html is missing or empty") {
		t.Fatalf("missing Web UI check: err=%v\n%s", err, output)
	}

	writeTestFile(t, filepath.Join(root, "internal", "entry", "webui", "static", "index.html"), "<!doctype html>\n")
	output, err = runCommand(root, "sh", verify, "verify")
	if err == nil || !strings.Contains(string(output), "catalog.json.gz is missing or empty") {
		t.Fatalf("missing catalog check: err=%v\n%s", err, output)
	}

	writeTestFile(t, filepath.Join(root, "internal", "service", "catalog_builtin", "catalog.json.gz"), "not gzip\n")
	output, err = runCommand(root, "sh", verify, "verify")
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
	}, "sh", filepath.Join(root, "scripts", "vercel-assets.sh"), "build")
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

	writeTestFile(t, catalogFixture, "not gzip\n")
	output, err = runCommandEnv(root, []string{
		"MAKE=" + makeStub,
		"MAKE_LOG=" + makeLog,
		"CATALOG_FIXTURE=" + catalogFixture,
	}, "sh", filepath.Join(root, "scripts", "vercel-assets.sh"), "build")
	if err == nil || !strings.Contains(string(output), "catalog.json.gz is not valid gzip") {
		t.Fatalf("build must reject invalid generated assets: err=%v\n%s", err, output)
	}
}

func TestVercelWorkflowUsesPrebuiltAssetPipeline(t *testing.T) {
	job := jobByName(t, readWorkflow(t, "ci.yml"), "vercel")
	if job.If != "github.ref_type == 'tag' || (github.event_name == 'push' && github.ref == 'refs/heads/main')" {
		t.Errorf("Vercel condition = %q", job.If)
	}
	if !slices.Equal(slices.Sorted(slices.Values(job.Needs)), []string{"go", "web"}) {
		t.Errorf("Vercel needs = %v", job.Needs)
	}
	if job.Concurrency.Group != "vercel-${{ github.ref }}" || !job.Concurrency.Cancel {
		t.Errorf("Vercel concurrency = %+v", job.Concurrency)
	}
	requireFields(t, job.Env, map[string]string{
		"VERCEL_ENVIRONMENT": "${{ github.ref_type == 'tag' && 'production' || 'preview' }}",
		"VERCEL_ORG_ID":      "${{ secrets.VERCEL_ORG_ID }}",
		"VERCEL_PROJECT_ID":  "${{ secrets.VERCEL_PROJECT_ID }}",
		"VERCEL_TOKEN":       "${{ secrets.VERCEL_TOKEN }}",
	})
	if job.Env["VERCEL_CLI_VERSION"] == "" || job.Env["VERCEL_CLI_VERSION"] == "latest" {
		t.Error("Vercel CLI version must be pinned")
	}
	stepByRun(t, job, `npm install --global "vercel@${VERCEL_CLI_VERSION}"`)
	build := stepByRun(t, job, "vercel build --standalone")
	requireCommands(t, build.Run, "vercel build --standalone --prod", "vercel build --standalone --token")
	deploy := stepByRun(t, job, "vercel deploy --prebuilt")
	requireCommands(t, deploy.Run, "vercel deploy --prebuilt --prod --archive=tgz", "vercel deploy --prebuilt --archive=tgz")
	stage := 0
	for _, step := range job.Steps {
		if strings.Contains(step.Run, "./scripts/vercel-assets.sh build") {
			stage = 1
		}
		if strings.Contains(step.Run, "vercel build --standalone") {
			if stage != 1 {
				t.Error("deployment build must follow assets")
			}
			stage = 2
		}
		if strings.Contains(step.Run, "vercel deploy --prebuilt") && stage != 2 {
			t.Error("deploy must follow deployment build")
		}
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
		filepath.Join("scripts", "vercel-assets.sh"),
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
