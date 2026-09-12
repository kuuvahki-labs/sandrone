package buildinfo

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildWebUIReusesCompletePrebuiltAssets(t *testing.T) {
	root, script := newWebUIBuildFixture(t)
	assets := filepath.Join(root, "prebuilt")
	static := filepath.Join(root, "internal", "entry", "webui", "static")
	for name, body := range map[string]string{"index.html": "tested index", "assets/app.js": "tested js", "assets/app.js.gz": "compressed", "assets/app.js.br": "\x00\xffbrotli", ".manifest": "hidden"} {
		writeTestFile(t, filepath.Join(assets, name), body)
	}
	writeTestFile(t, filepath.Join(static, "stale.js"), "stale")
	writeTestFile(t, filepath.Join(static, ".gitkeep"), "")
	bin := filepath.Join(root, "bin")
	pnpm := filepath.Join(bin, "pnpm")
	writeTestFile(t, pnpm, "#!/bin/sh\nprintf called > pnpm-called\nexit 1\n")
	if err := os.Chmod(pnpm, 0o700); err != nil {
		t.Fatal(err)
	}
	output, err := runCommandEnv(root, []string{"WEBUI_PREBUILT_DIR=" + assets, "PATH=" + bin + ":" + os.Getenv("PATH")}, "bash", script)
	if err != nil {
		t.Fatalf("copy: %v\n%s", err, output)
	}
	for _, dir := range []string{root, filepath.Join(root, "web")} {
		if _, err := os.Stat(filepath.Join(dir, "pnpm-called")); !os.IsNotExist(err) {
			t.Fatalf("prebuilt path must not invoke pnpm: %v", err)
		}
	}
	for name, want := range map[string]string{"index.html": "tested index", "assets/app.js": "tested js", "assets/app.js.gz": "compressed", "assets/app.js.br": "\x00\xffbrotli", ".manifest": "hidden", ".gitkeep": ""} {
		body, err := os.ReadFile(filepath.Join(static, name))
		if err != nil || string(body) != want {
			t.Errorf("asset %s = %q: %v", name, body, err)
		}
	}
	if _, err := os.Stat(filepath.Join(static, "stale.js")); !os.IsNotExist(err) {
		t.Errorf("stale asset not removed: %v", err)
	}
}
func TestBuildWebUIInvalidPrebuiltPreservesDestination(t *testing.T) {
	for _, kind := range []string{"missing-index", "empty-index", "same-directory", "inside-destination", "contains-destination"} {
		t.Run(kind, func(t *testing.T) {
			root, script := newWebUIBuildFixture(t)
			static := filepath.Join(root, "internal", "entry", "webui", "static")
			writeTestFile(t, filepath.Join(static, "index.html"), "existing index")
			assets := filepath.Join(root, "prebuilt")
			if err := os.MkdirAll(assets, 0o700); err != nil {
				t.Fatal(err)
			}
			switch kind {
			case "empty-index":
				writeTestFile(t, filepath.Join(assets, "index.html"), "")
			case "same-directory":
				assets = static
			case "inside-destination":
				assets = filepath.Join(static, "nested")
				writeTestFile(t, filepath.Join(assets, "index.html"), "nested")
			case "contains-destination":
				assets = root
				writeTestFile(t, filepath.Join(assets, "index.html"), "ancestor")
			}
			output, err := runCommandEnv(root, []string{"WEBUI_PREBUILT_DIR=" + assets}, "bash", script)
			if err == nil {
				t.Fatalf("invalid assets accepted: %s", output)
			}
			body, err := os.ReadFile(filepath.Join(static, "index.html"))
			if err != nil || string(body) != "existing index" {
				t.Fatalf("destination changed: %q, %v", body, err)
			}
		})
	}
}
func TestBuildWebUIDefaultBuildsWithFrozenLockfile(t *testing.T) {
	root, script := newWebUIBuildFixture(t)
	bin := filepath.Join(root, "bin")
	stub := filepath.Join(bin, "pnpm")
	writeTestFile(t, stub, `#!/bin/sh
printf '%s\n' "$*" >> pnpm.log
if [ "$1" = build ]; then
 mkdir -p build/client
 printf '%s\n' built > build/client/index.html
fi
`)
	if err := os.Chmod(stub, 0o700); err != nil {
		t.Fatal(err)
	}
	output, err := runCommandEnv(root, []string{"PATH=" + bin + ":" + os.Getenv("PATH"), "WEBUI_PREBUILT_DIR="}, "bash", script)
	if err != nil {
		t.Fatalf("build: %v\n%s", err, output)
	}
	body, err := os.ReadFile(filepath.Join(root, "web", "pnpm.log"))
	if err != nil || string(body) != "install --frozen-lockfile\nbuild\n" {
		t.Fatalf("pnpm calls %q: %v", body, err)
	}
	body, err = os.ReadFile(filepath.Join(root, "internal", "entry", "webui", "static", "index.html"))
	if err != nil || strings.TrimSpace(string(body)) != "built" {
		t.Fatalf("embedded index %q: %v", body, err)
	}
}
func newWebUIBuildFixture(t *testing.T) (string, string) {
	t.Helper()
	body, err := os.ReadFile(filepath.Join("..", "..", "scripts", "build-webui.sh"))
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	script := filepath.Join(root, "scripts", "build-webui.sh")
	writeTestFile(t, script, string(body))
	if err := os.MkdirAll(filepath.Join(root, "web"), 0o700); err != nil {
		t.Fatal(err)
	}
	return root, script
}
