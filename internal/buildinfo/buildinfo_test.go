package buildinfo

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"strings"
	"testing"
)

func TestVersion(t *testing.T) {
	original := rawVersion
	t.Cleanup(func() { rawVersion = original })

	tests := []struct {
		name string
		raw  string
		want string
	}{
		{name: "default", raw: "0.1.0", want: "0.1.0"},
		{name: "blank falls back to default", raw: " \t\n", want: "0.1.1"},
		{name: "trims whitespace", raw: "  1.2.3\n", want: "1.2.3"},
		{name: "removes leading v", raw: "v1.2.3", want: "1.2.3"},
		{name: "removes only one leading v", raw: "vv1.2.3", want: "v1.2.3"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rawVersion = tt.raw
			if got := Version(); got != tt.want {
				t.Fatalf("Version() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestVersionFormats(t *testing.T) {
	original := rawVersion
	t.Cleanup(func() { rawVersion = original })
	rawVersion = "0.1.0"

	if got := DisplayVersion(); got != "v0.1.0" {
		t.Fatalf("DisplayVersion() = %q, want %q", got, "v0.1.0")
	}
	if got := UserAgent(); got != "sandrone/0.1.0" {
		t.Fatalf("UserAgent() = %q, want %q", got, "sandrone/0.1.0")
	}

	rawVersion = "dev"
	if got := DisplayVersion(); got != "dev" {
		t.Fatalf("DisplayVersion() = %q, want %q", got, "dev")
	}
}

func TestRevisionPrefersInjectedValue(t *testing.T) {
	originalRevision := rawRevision
	originalReadBuildInfo := readBuildInfo
	t.Cleanup(func() {
		rawRevision = originalRevision
		readBuildInfo = originalReadBuildInfo
	})

	rawRevision = " 0123456789abcdef "
	readBuildInfo = func() (*debug.BuildInfo, bool) {
		return &debug.BuildInfo{Settings: []debug.BuildSetting{
			{Key: "vcs.revision", Value: "fedcba9876543210"},
		}}, true
	}

	if got := Revision(); got != "0123456789abcdef" {
		t.Fatalf("Revision() = %q, want %q", got, "0123456789abcdef")
	}
}

func TestRevisionFallsBackToGoBuildInfo(t *testing.T) {
	originalRevision := rawRevision
	originalReadBuildInfo := readBuildInfo
	t.Cleanup(func() {
		rawRevision = originalRevision
		readBuildInfo = originalReadBuildInfo
	})

	rawRevision = ""
	readBuildInfo = func() (*debug.BuildInfo, bool) {
		return &debug.BuildInfo{Settings: []debug.BuildSetting{
			{Key: "vcs.revision", Value: "fedcba9876543210"},
		}}, true
	}

	if got := Revision(); got != "fedcba9876543210" {
		t.Fatalf("Revision() = %q, want %q", got, "fedcba9876543210")
	}
}

func TestDirtyVCSBuildUsesDevWithoutRevision(t *testing.T) {
	originalVersion := rawVersion
	originalRevision := rawRevision
	originalReadBuildInfo := readBuildInfo
	t.Cleanup(func() {
		rawVersion = originalVersion
		rawRevision = originalRevision
		readBuildInfo = originalReadBuildInfo
	})

	rawVersion = ""
	rawRevision = ""
	readBuildInfo = func() (*debug.BuildInfo, bool) {
		return &debug.BuildInfo{Settings: []debug.BuildSetting{
			{Key: "vcs.revision", Value: "0123456789abcdef0123456789abcdef01234567"},
			{Key: "vcs.modified", Value: "true"},
		}}, true
	}

	if got := Version(); got != "dev" {
		t.Fatalf("Version() = %q, want %q", got, "dev")
	}
	if got := Revision(); got != "" {
		t.Fatalf("Revision() = %q, want empty for dirty VCS build", got)
	}
	if got := Summary(); got != "dev" {
		t.Fatalf("Summary() = %q, want %q", got, "dev")
	}
	if got := UserAgent(); got != "sandrone/dev" {
		t.Fatalf("UserAgent() = %q, want %q", got, "sandrone/dev")
	}
}

func TestSummaryIncludesShortRevisionWithoutChangingUserAgent(t *testing.T) {
	originalVersion := rawVersion
	originalRevision := rawRevision
	originalReadBuildInfo := readBuildInfo
	t.Cleanup(func() {
		rawVersion = originalVersion
		rawRevision = originalRevision
		readBuildInfo = originalReadBuildInfo
	})

	rawVersion = "0.1.0"
	rawRevision = "0123456789abcdef"
	readBuildInfo = func() (*debug.BuildInfo, bool) { return nil, false }

	if got := Summary(); got != "0.1.0 (0123456789ab)" {
		t.Fatalf("Summary() = %q, want %q", got, "0.1.0 (0123456789ab)")
	}
	if got := UserAgent(); got != "sandrone/0.1.0" {
		t.Fatalf("UserAgent() = %q, want %q", got, "sandrone/0.1.0")
	}
}

func TestSummaryOmitsUnknownRevision(t *testing.T) {
	originalVersion := rawVersion
	originalRevision := rawRevision
	originalReadBuildInfo := readBuildInfo
	t.Cleanup(func() {
		rawVersion = originalVersion
		rawRevision = originalRevision
		readBuildInfo = originalReadBuildInfo
	})

	rawVersion = "dev"
	rawRevision = ""
	readBuildInfo = func() (*debug.BuildInfo, bool) { return nil, false }

	if got := Summary(); got != "dev" {
		t.Fatalf("Summary() = %q, want %q", got, "dev")
	}
}

func TestDefaultVersionHasSingleCanonicalFile(t *testing.T) {
	content, err := os.ReadFile("VERSION")
	if err != nil {
		t.Fatal(err)
	}
	want := strings.TrimSpace(string(content))
	if want == "" {
		t.Fatal("VERSION is empty")
	}

	originalVersion := rawVersion
	originalRevision := rawRevision
	t.Cleanup(func() {
		rawVersion = originalVersion
		rawRevision = originalRevision
	})
	rawVersion = ""
	rawRevision = "0123456789abcdef"

	if got := Version(); got != want {
		t.Fatalf("Version() = %q, want embedded VERSION %q", got, want)
	}
}

func TestMakeAcceptsSafeBuildVersions(t *testing.T) {
	for _, version := range []string{"v0.1.0", "0.1.0-rc.1+meta"} {
		t.Run(version, func(t *testing.T) {
			output, err := runMake(t, "help", "VERSION="+version)
			if err != nil {
				t.Fatalf("make rejected safe VERSION %q: %v\n%s", version, err, output)
			}
		})
	}
}

func TestMakeAcceptsSafeBuildRevisions(t *testing.T) {
	for _, revision := range []string{
		"0123456789abcdef0123456789abcdef01234567",
		"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
	} {
		t.Run(revision, func(t *testing.T) {
			output, err := runMake(t, "help", "REVISION="+revision)
			if err != nil {
				t.Fatalf("make rejected safe REVISION %q: %v\n%s", revision, err, output)
			}
		})
	}
}

func TestMakeRejectsUnsafeBuildRevisions(t *testing.T) {
	for _, revision := range []string{"123456", "0123456", "012345g", "0123456 ", "sha-0123456", "$HOME", "$(whoami)"} {
		t.Run(revision, func(t *testing.T) {
			if output, err := runMake(t, "help", "REVISION="+revision); err == nil {
				t.Fatalf("make accepted unsafe REVISION %q:\n%s", revision, output)
			}
		})
	}
}

func TestResolveBuildRevisionRequiresCleanGitWorktree(t *testing.T) {
	script, err := filepath.Abs(filepath.Join("..", "..", "scripts", "resolve-build-revision.sh"))
	if err != nil {
		t.Fatal(err)
	}

	t.Run("clean", func(t *testing.T) {
		repo, revision := newGitRepo(t)
		output, err := runCommand(repo, "sh", script)
		if err != nil {
			t.Fatalf("resolve clean revision: %v\n%s", err, output)
		}
		if got := strings.TrimSpace(string(output)); got != revision {
			t.Fatalf("resolved revision = %q, want %q", got, revision)
		}
	})

	t.Run("modified tracked file", func(t *testing.T) {
		repo, _ := newGitRepo(t)
		if err := os.WriteFile(filepath.Join(repo, "tracked.txt"), []byte("dirty\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		output, err := runCommand(repo, "sh", script)
		if err != nil {
			t.Fatalf("resolve dirty revision: %v\n%s", err, output)
		}
		if got := strings.TrimSpace(string(output)); got != "" {
			t.Fatalf("resolved dirty revision = %q, want empty", got)
		}
	})

	t.Run("untracked file", func(t *testing.T) {
		repo, _ := newGitRepo(t)
		if err := os.WriteFile(filepath.Join(repo, "untracked.txt"), []byte("dirty\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		output, err := runCommand(repo, "sh", script)
		if err != nil {
			t.Fatalf("resolve dirty revision: %v\n%s", err, output)
		}
		if got := strings.TrimSpace(string(output)); got != "" {
			t.Fatalf("resolved dirty revision = %q, want empty", got)
		}
	})

	t.Run("not a git repository", func(t *testing.T) {
		output, err := runCommand(t.TempDir(), "sh", script)
		if err != nil {
			t.Fatalf("resolve absent revision: %v\n%s", err, output)
		}
		if got := strings.TrimSpace(string(output)); got != "" {
			t.Fatalf("resolved absent revision = %q, want empty", got)
		}
	})
}

func TestMakeImageDerivesIdentityFromWorktreeState(t *testing.T) {
	t.Run("clean worktree", func(t *testing.T) {
		repo, revision := newMakeFixtureRepo(t)
		output, err := runMakeAt(repo, "image", "DOCKER=echo")
		if err != nil {
			t.Fatalf("make image in clean worktree: %v\n%s", err, output)
		}
		for _, want := range []string{
			"--build-arg VERSION=0.1.1",
			"--build-arg REVISION=" + revision,
		} {
			if !strings.Contains(string(output), want) {
				t.Errorf("clean make image output does not contain %q:\n%s", want, output)
			}
		}
	})

	t.Run("dirty worktree", func(t *testing.T) {
		repo, _ := newMakeFixtureRepo(t)
		if err := os.WriteFile(filepath.Join(repo, "untracked.txt"), []byte("dirty\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		output, err := runMakeAt(repo, "image", "DOCKER=echo")
		if err != nil {
			t.Fatalf("make image in dirty worktree: %v\n%s", err, output)
		}
		for _, want := range []string{
			"--build-arg VERSION=dev",
			"--build-arg REVISION=",
		} {
			if !strings.Contains(string(output), want) {
				t.Errorf("dirty make image output does not contain %q:\n%s", want, output)
			}
		}
	})
}

func TestMakeInjectsBuildRevision(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "sandrone")
	revision := "deadbeefcafe0123456789abcdef0123456789ab"

	output, err := runMake(t, "build-check", "BUILD_BIN="+binary, "REVISION="+revision)
	if err != nil {
		t.Fatalf("make build-check failed: %v\n%s", err, output)
	}
	command := exec.Command(binary, "--version")
	versionOutput, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("built binary --version failed: %v\n%s", err, versionOutput)
	}
	if got, want := string(versionOutput), "sandrone version 0.1.1 (deadbeefcafe)\n"; got != want {
		t.Fatalf("built binary --version = %q, want %q", got, want)
	}
}

func TestMakeBuildWithoutRevisionForcesDevVersion(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "sandrone")
	output, err := runMake(
		t,
		"build-check",
		"BUILD_BIN="+binary,
		"VERSION=9.9.9",
		"REVISION=",
	)
	if err != nil {
		t.Fatalf("make build-check failed: %v\n%s", err, output)
	}
	command := exec.Command(binary, "--version")
	versionOutput, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("built binary --version failed: %v\n%s", err, versionOutput)
	}
	if got, want := string(versionOutput), "sandrone version dev\n"; got != want {
		t.Fatalf("built binary --version = %q, want %q", got, want)
	}
}

func TestArtifactTargetsUseCanonicalScript(t *testing.T) {
	makefile, err := os.ReadFile(filepath.Join("..", "..", "Makefile"))
	if err != nil {
		t.Fatal(err)
	}
	content := string(makefile)
	validatedLine := "VALIDATED_TARGETS := help check ci fmt fmt-check vet test test-webui test-webui-e2e build build-bin build-check build-webui image lint ruleset-catalog release-artifacts snapshot-artifacts"
	if !strings.Contains(content, validatedLine) {
		t.Errorf("Makefile does not validate artifact targets")
	}
	for target, wantRecipe := range map[string]string{
		"release-artifacts":  "release-artifacts: build-webui\n\tARTIFACT_KIND=release VERSION=\"$(BUILD_VERSION)\" REVISION=\"$(BUILD_REVISION)\" ./scripts/build-release-artifacts.sh\n",
		"snapshot-artifacts": "snapshot-artifacts: build-webui\n\tARTIFACT_KIND=snapshot VERSION=dev REVISION=\"\" OUTPUT_DIR=\"$(CURDIR)/dist/snapshot\" ./scripts/build-release-artifacts.sh\n",
	} {
		if count := strings.Count(content, wantRecipe); count != 1 {
			t.Errorf("Makefile contains %d canonical %s recipes, want 1", count, target)
		}
	}
}

func TestBuildReleaseArtifactsProducesCanonicalArchive(t *testing.T) {
	repo, script, makeLog := newReleaseArtifactFixture(t)
	outputDir := filepath.Join(repo, "output")
	revision := "0123456789abcdef0123456789abcdef01234567"
	output, err := runCommandEnv(repo, []string{
		"PATH=" + os.Getenv("PATH"),
		"VERSION=1.2.3",
		"REVISION=" + revision,
		"RELEASE_TARGETS=linux/arm64",
		"OUTPUT_DIR=" + outputDir,
		"MAKE=" + script,
		"MAKE_LOG=" + makeLog,
	}, "sh", filepath.Join(repo, "scripts", "build-release-artifacts.sh"))
	if err != nil {
		t.Fatalf("build release artifacts: %v\n%s", err, output)
	}

	archiveName := "sandrone_linux_arm64.tar.gz"
	archive := filepath.Join(outputDir, archiveName)
	listing, err := runCommand(repo, "tar", "-tzf", archive)
	if err != nil {
		t.Fatalf("list release archive: %v\n%s", err, listing)
	}
	if got, want := strings.Fields(string(listing)), []string{"sandrone", "LICENSE"}; strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("archive entries = %q, want %q", got, want)
	}
	checksumOutput, err := runCommand(outputDir, "sha256sum", "-c", "checksums.txt")
	if err != nil {
		t.Fatalf("verify release checksums: %v\n%s", err, checksumOutput)
	}
	if got, want := string(checksumOutput), archiveName+": OK\n"; got != want {
		t.Fatalf("checksum output = %q, want %q", got, want)
	}
	entries, err := os.ReadDir(outputDir)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := len(entries), 2; got != want {
		t.Fatalf("output file count = %d, want %d", got, want)
	}
	makeCalls, err := os.ReadFile(makeLog)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(makeCalls)), "\n")
	if got, want := len(lines), 2; got != want {
		t.Fatalf("make call count = %d, want %d:\n%s", got, want, makeCalls)
	}
	if got, want := lines[0], "|||ruleset-catalog"; got != want {
		t.Errorf("ruleset catalog call = %q, want %q", got, want)
	}
	if !strings.HasPrefix(lines[1], "0|linux|arm64|build-check BUILD_BIN=") {
		t.Errorf("build call does not use the linux/arm64 static environment:\n%s", lines[1])
	}
	for _, want := range []string{" VERSION=1.2.3", " REVISION=" + revision} {
		if !strings.Contains(lines[1], want) {
			t.Errorf("build call does not contain %q:\n%s", want, lines[1])
		}
	}
}

func TestBuildSnapshotArtifactsAllowsUnknownRevision(t *testing.T) {
	repo, script, makeLog := newReleaseArtifactFixture(t)
	outputDir := filepath.Join(repo, "snapshot")
	output, err := runCommandEnv(repo, []string{
		"PATH=" + os.Getenv("PATH"),
		"ARTIFACT_KIND=snapshot",
		"VERSION=dev",
		"REVISION=",
		"RELEASE_TARGETS=linux/arm64",
		"OUTPUT_DIR=" + outputDir,
		"MAKE=" + script,
		"MAKE_LOG=" + makeLog,
	}, "sh", filepath.Join(repo, "scripts", "build-release-artifacts.sh"))
	if err != nil {
		t.Fatalf("build snapshot artifacts: %v\n%s", err, output)
	}

	if _, err := os.Stat(filepath.Join(outputDir, "sandrone_linux_arm64.tar.gz")); err != nil {
		t.Fatalf("snapshot archive: %v", err)
	}
	makeCalls, err := os.ReadFile(makeLog)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(makeCalls)), "\n")
	if got, want := len(lines), 2; got != want {
		t.Fatalf("make call count = %d, want %d:\n%s", got, want, makeCalls)
	}
	for _, want := range []string{" VERSION=dev", " REVISION="} {
		if !strings.Contains(lines[1], want) {
			t.Errorf("snapshot build call does not contain %q:\n%s", want, lines[1])
		}
	}
}

func TestBuildReleaseArtifactsRejectsUnknownRevision(t *testing.T) {
	repo, script, makeLog := newReleaseArtifactFixture(t)
	output, err := runCommandEnv(repo, []string{
		"PATH=" + os.Getenv("PATH"),
		"ARTIFACT_KIND=release",
		"VERSION=dev",
		"REVISION=",
		"RELEASE_TARGETS=linux/arm64",
		"OUTPUT_DIR=" + filepath.Join(repo, "output"),
		"MAKE=" + script,
		"MAKE_LOG=" + makeLog,
	}, "sh", filepath.Join(repo, "scripts", "build-release-artifacts.sh"))
	if err == nil {
		t.Fatalf("release artifacts accepted an empty revision:\n%s", output)
	}
	if want := "release artifacts require REVISION"; !strings.Contains(string(output), want) {
		t.Fatalf("missing error %q:\n%s", want, output)
	}
}

func TestBuildReleaseArtifactsRejectsUnsupportedTarget(t *testing.T) {
	repo, script, makeLog := newReleaseArtifactFixture(t)
	outputDir := filepath.Join(repo, "output")
	output, err := runCommandEnv(repo, []string{
		"PATH=" + os.Getenv("PATH"),
		"VERSION=1.2.3",
		"REVISION=0123456789abcdef0123456789abcdef01234567",
		"RELEASE_TARGETS=darwin/arm64",
		"OUTPUT_DIR=" + outputDir,
		"MAKE=" + script,
		"MAKE_LOG=" + makeLog,
	}, "sh", filepath.Join(repo, "scripts", "build-release-artifacts.sh"))
	if err == nil {
		t.Fatalf("unsupported release target succeeded:\n%s", output)
	}
	if want := "unsupported release target darwin/arm64"; !strings.Contains(string(output), want) {
		t.Fatalf("unsupported target output does not contain %q:\n%s", want, output)
	}
	if entries, readErr := os.ReadDir(outputDir); readErr == nil && len(entries) != 0 {
		t.Fatalf("unsupported target left %d final output files", len(entries))
	} else if readErr != nil && !os.IsNotExist(readErr) {
		t.Fatal(readErr)
	}
}

func newReleaseArtifactFixture(t *testing.T) (repo, makeScript, makeLog string) {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	repo = t.TempDir()
	for _, name := range []string{
		"LICENSE",
		filepath.Join("scripts", "build-release-artifacts.sh"),
		filepath.Join("scripts", "validate-build-revision.sh"),
		filepath.Join("scripts", "validate-build-version.sh"),
	} {
		content, readErr := os.ReadFile(filepath.Join(root, name))
		if readErr != nil {
			t.Fatal(readErr)
		}
		target := filepath.Join(repo, name)
		if mkdirErr := os.MkdirAll(filepath.Dir(target), 0o700); mkdirErr != nil {
			t.Fatal(mkdirErr)
		}
		if writeErr := os.WriteFile(target, content, 0o700); writeErr != nil {
			t.Fatal(writeErr)
		}
	}
	makeLog = filepath.Join(repo, "make.log")
	makeScript = filepath.Join(repo, "fake-make")
	fakeMake := `#!/bin/sh
set -eu
printf '%s|%s|%s|%s\n' "${CGO_ENABLED-}" "${GOOS-}" "${GOARCH-}" "$*" >>"$MAKE_LOG"
if [ "$1" = build-check ]; then
  build_bin=
  for argument do
    case "$argument" in
      BUILD_BIN=*) build_bin=${argument#BUILD_BIN=} ;;
    esac
  done
  [ -n "$build_bin" ]
  mkdir -p "$(dirname "$build_bin")"
  printf '%s\n' sandrone-binary >"$build_bin"
  chmod 0755 "$build_bin"
fi
`
	if err := os.WriteFile(makeScript, []byte(fakeMake), 0o700); err != nil {
		t.Fatal(err)
	}
	return repo, makeScript, makeLog
}

func workflowTriggerBlock(t *testing.T, workflow string) string {
	t.Helper()

	const startMarker = "on:\n"
	const endMarker = "\npermissions:\n"
	if count := strings.Count(workflow, startMarker); count != 1 {
		t.Fatalf("workflow contains %d top-level on blocks, want 1", count)
	}
	start := strings.Index(workflow, startMarker)
	end := strings.Index(workflow[start+len(startMarker):], endMarker)
	if end < 0 {
		t.Fatal("workflow on block is not followed by permissions")
	}
	return strings.TrimRight(workflow[start:start+len(startMarker)+end], "\n")
}

func workflowNamedStep(t *testing.T, workflow, name string) string {
	t.Helper()

	marker := "      - name: " + name + "\n"
	if count := strings.Count(workflow, marker); count != 1 {
		t.Fatalf("workflow contains %d %q steps, want 1", count, name)
	}
	start := strings.Index(workflow, marker)
	body := workflow[start+len(marker):]
	body = workflowIndentedBody(body, 6)
	return strings.TrimRight(marker+body, "\n")
}

func workflowIndentedBody(body string, parentIndent int) string {
	offset := 0
	for _, line := range strings.Split(body, "\n") {
		if line != "" {
			indent := len(line) - len(strings.TrimLeft(line, " "))
			if indent <= parentIndent {
				return strings.TrimRight(body[:offset], "\n")
			}
		}
		offset += len(line) + 1
	}
	return strings.TrimRight(body, "\n")
}

func workflowNamedJob(t *testing.T, workflow, name string) string {
	t.Helper()

	marker := "  " + name + ":\n"
	if count := strings.Count(workflow, marker); count != 1 {
		t.Fatalf("workflow contains %d %q jobs, want 1", count, name)
	}
	start := strings.Index(workflow, marker)
	body := workflow[start+len(marker):]
	body = workflowIndentedBody(body, 2)
	return strings.TrimRight(marker+body, "\n")
}

func TestBuildMetadataContracts(t *testing.T) {
	root := filepath.Join("..", "..")
	dockerfile, err := os.ReadFile(filepath.Join(root, "Dockerfile"))
	if err != nil {
		t.Fatal(err)
	}
	dockerignore, err := os.ReadFile(filepath.Join(root, ".dockerignore"))
	if err != nil {
		t.Fatal(err)
	}
	workflow, err := os.ReadFile(filepath.Join(root, ".github", "workflows", "ci.yml"))
	if err != nil {
		t.Fatal(err)
	}
	buildReference, err := os.ReadFile(filepath.Join(root, "docs", "reference", "build-info.md"))
	if err != nil {
		t.Fatal(err)
	}

	for _, want := range []string{
		"sandrone_linux_amd64.tar.gz",
		"sandrone_linux_arm64.tar.gz",
		"checksums.txt",
		"sha256sum -c checksums.txt",
		"linux/amd64",
		"linux/arm64",
		"发布版本不允许加号",
		"最多 127 个字符",
		"只有推送与规范版本匹配的 `v<version>` Git tag 才会发布",
		"稳定版本才同时更新 `latest`",
		"预发布 tag 只发布自己的同名 tag",
		"pull request、`main` 和手动 CI 只验证 `linux/amd64`",
		"`v<version>` tag 构建并发布",
		"`linux/amd64` 和 `linux/arm64` 的 GHCR manifest",
		"`$BUILDPLATFORM`",
		"`GOOS`/`GOARCH`",
		"GitHub Actions 缓存",
		"随后并行发布",
	} {
		if !strings.Contains(string(buildReference), want) {
			t.Errorf("build reference does not contain %q", want)
		}
	}

	for _, want := range []string{
		`FROM --platform=$BUILDPLATFORM node:24.17.0-bookworm AS web`,
		`FROM --platform=$BUILDPLATFORM golang:1.25.11-bookworm AS build`,
		`ARG VERSION="dev"`,
		"ARG REVISION",
		"ARG TARGETOS",
		"ARG TARGETARCH",
		`CGO_ENABLED=0 GOOS="$TARGETOS" GOARCH="$TARGETARCH"`,
		`COPY --from=web /src/web/build/client ./internal/entry/webui/static`,
		`if [ -z "$REVISION" ] && [ "$VERSION" != "dev" ]; then`,
		"org.opencontainers.image.source=https://github.com/kuuvahki-labs/sandrone",
	} {
		if !strings.Contains(string(dockerfile), want) {
			t.Errorf("Dockerfile does not contain %q", want)
		}
	}
	if strings.Contains(string(dockerfile), `COPY --from=web --chown=sandrone:sandrone /src/web/build/client /app/static`) {
		t.Error("Dockerfile must not duplicate embedded Web UI assets in the runtime image")
	}
	if !strings.Contains(string(dockerignore), "internal/entry/webui/static") {
		t.Error("Docker context must exclude ignored host Web UI assets before copying the current web build")
	}
	if labelAt, copyAt := strings.LastIndex(string(dockerfile), "LABEL "), strings.LastIndex(string(dockerfile), "COPY --from=web"); labelAt < copyAt {
		t.Error("Dockerfile OCI labels must follow runtime package and asset layers")
	}

	workflowText := string(workflow)
	for _, want := range []string{
		`uses: actions/checkout@v7`,
		`uses: actions/setup-go@v7`,
		`uses: actions/setup-node@v7`,
		`uses: pnpm/action-setup@v6`,
		`uses: docker/setup-qemu-action@v4`,
		`uses: docker/setup-buildx-action@v4`,
		`uses: docker/login-action@v4`,
		`uses: docker/build-push-action@v7`,
	} {
		if !strings.Contains(workflowText, want) {
			t.Errorf("workflow does not contain Node.js 24 action %q", want)
		}
	}
	for _, forbidden := range []string{
		`uses: actions/checkout@v4`,
		`uses: actions/setup-go@v5`,
		`uses: actions/setup-node@v4`,
		`uses: pnpm/action-setup@v4`,
		`uses: docker/setup-qemu-action@v3`,
		`uses: docker/setup-buildx-action@v3`,
		`uses: docker/login-action@v3`,
		`uses: docker/build-push-action@v6`,
	} {
		if strings.Contains(workflowText, forbidden) {
			t.Errorf("workflow still contains Node.js 20 action %q", forbidden)
		}
	}
	if got, want := workflowTriggerBlock(t, workflowText), `on:
  pull_request:
  push:
    branches:
      - main
    tags:
      - "v*"
  workflow_dispatch:`; got != want {
		t.Errorf("workflow trigger block =\n%s\nwant:\n%s", got, want)
	}
	if strings.Contains(workflowText, "  build-webui:\n") {
		t.Error("workflow must not duplicate the embedded Web UI build outside the container and release jobs")
	}
	if strings.Contains(workflowText, "  container-image:\n") {
		t.Error("workflow must split ordinary container validation from release publication")
	}

	containerCheckJob := workflowNamedJob(t, workflowText, "container-check")
	if got, want := workflowNamedStep(t, containerCheckJob, "Resolve image version"), `      - name: Resolve image version
        id: image-metadata
        shell: bash
        run: |
          version="$(tr -d '\r\n' < internal/buildinfo/VERSION)"
          echo "version=${version}" >> "${GITHUB_OUTPUT}"`; got != want {
		t.Errorf("Resolve image version step =\n%s\nwant:\n%s", got, want)
	}
	if got, want := workflowNamedStep(t, containerCheckJob, "Build container image"), `      - name: Build container image
        uses: docker/build-push-action@v7
        with:
          context: .
          platforms: linux/amd64
          push: false
          tags: ${{ env.IMAGE }}:ci
          cache-from: type=gha,scope=sandrone-container
          cache-to: type=gha,mode=max,scope=sandrone-container
          build-args: |
            VERSION=${{ steps.image-metadata.outputs.version }}
            REVISION=${{ github.sha }}
          labels: |
            org.opencontainers.image.version=${{ steps.image-metadata.outputs.version }}
            org.opencontainers.image.revision=${{ github.sha }}`; got != want {
		t.Errorf("container check build step =\n%s\nwant:\n%s", got, want)
	}
	for _, want := range []string{
		`if: github.event_name != 'push' || github.ref_type != 'tag'`,
		`uses: docker/setup-buildx-action@v4`,
	} {
		if !strings.Contains(containerCheckJob, want) {
			t.Errorf("container check workflow does not contain %q", want)
		}
	}
	for _, forbidden := range []string{"needs:", "packages: write", "setup-qemu-action", "login-action", "make image", "docker push"} {
		if strings.Contains(containerCheckJob, forbidden) {
			t.Errorf("container check workflow must not contain %q", forbidden)
		}
	}

	containerPublishJob := workflowNamedJob(t, workflowText, "container-publish")
	if got, want := workflowNamedStep(t, containerPublishJob, "Resolve image metadata"), `      - name: Resolve image metadata
        id: image-metadata
        shell: bash
        run: |
          version="$(tr -d '\r\n' < internal/buildinfo/VERSION)"
          git fetch --force --tags origin
          tags="$(./scripts/container-image-tags.sh)"
          {
            echo "version=${version}"
            echo 'tags<<EOF'
            echo "${tags}"
            echo EOF
          } >> "${GITHUB_OUTPUT}"`; got != want {
		t.Errorf("Resolve image metadata step =\n%s\nwant:\n%s", got, want)
	}
	if got, want := workflowNamedStep(t, containerPublishJob, "Log in to GHCR"), `      - name: Log in to GHCR
        uses: docker/login-action@v4
        with:
          registry: ghcr.io
          username: ${{ github.actor }}
          password: ${{ secrets.GITHUB_TOKEN }}`; got != want {
		t.Errorf("Log in to GHCR step =\n%s\nwant:\n%s", got, want)
	}
	if got, want := workflowNamedStep(t, containerPublishJob, "Build and publish container image"), `      - name: Build and publish container image
        uses: docker/build-push-action@v7
        with:
          context: .
          platforms: linux/amd64,linux/arm64
          push: true
          tags: ${{ steps.image-metadata.outputs.tags }}
          cache-from: type=gha,scope=sandrone-container
          cache-to: type=gha,mode=max,scope=sandrone-container
          build-args: |
            VERSION=${{ steps.image-metadata.outputs.version }}
            REVISION=${{ github.sha }}
          labels: |
            org.opencontainers.image.version=${{ steps.image-metadata.outputs.version }}
            org.opencontainers.image.revision=${{ github.sha }}`; got != want {
		t.Errorf("container publish build step =\n%s\nwant:\n%s", got, want)
	}
	if got, want := workflowNamedStep(t, containerPublishJob, "Set up QEMU"), `      - name: Set up QEMU
        uses: docker/setup-qemu-action@v4`; got != want {
		t.Errorf("Set up QEMU step =\n%s\nwant:\n%s", got, want)
	}
	for _, forbidden := range []string{"make image", "docker tag", "docker push", "sha-"} {
		if strings.Contains(containerPublishJob, forbidden) {
			t.Errorf("container workflow must not contain %q", forbidden)
		}
	}
	for _, want := range []string{
		`if: github.event_name == 'push' && github.ref_type == 'tag'`,
		`uses: docker/setup-buildx-action@v4`,
		`needs:`,
		`- go`,
		`- web`,
		`packages: write`,
		`fetch-depth: 0`,
		`group: container-image-publish`,
		`cancel-in-progress: false`,
		`queue: max`,
	} {
		if !strings.Contains(containerPublishJob, want) {
			t.Errorf("container workflow does not contain %q", want)
		}
	}

	releaseJob := workflowNamedJob(t, workflowText, "release")
	for _, want := range []string{
		`if: github.event_name == 'push' && github.ref_type == 'tag'`,
		"concurrency:\n      group: github-release-publish\n      cancel-in-progress: false\n      queue: max",
		"needs:\n      - go\n      - web",
		"permissions:\n      contents: write",
		`uses: actions/checkout@v7`,
		`fetch-depth: 0`,
		`uses: actions/setup-go@v7`,
		`go-version-file: go.mod`,
		`uses: pnpm/action-setup@v6`,
		`version: 11.5.2`,
		`uses: actions/setup-node@v7`,
		`node-version-file: .node-version`,
		`make release-artifacts REVISION="${GITHUB_SHA}"`,
		`GH_TOKEN: ${{ secrets.GITHUB_TOKEN }}`,
		`artifacts=(`,
		`"dist/sandrone_linux_amd64.tar.gz"`,
		`"dist/sandrone_linux_arm64.tar.gz"`,
		`"dist/checksums.txt"`,
		`if gh release view "${GITHUB_REF_NAME}" >/dev/null 2>&1; then`,
		`gh release upload "${GITHUB_REF_NAME}" "${artifacts[@]}" --clobber`,
		`if [[ ! "${GITHUB_REF_NAME}" =~ ^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$ ]]; then`,
		`prerelease_flag="--prerelease"`,
		`gh release create "${GITHUB_REF_NAME}" "${artifacts[@]}" --verify-tag --generate-notes ${prerelease_flag}`,
	} {
		if !strings.Contains(releaseJob, want) {
			t.Errorf("release job does not contain %q", want)
		}
	}
	if strings.Contains(releaseJob, `run: make build-webui`) {
		t.Error("release job must let make release-artifacts build the Web UI exactly once")
	}
	if strings.Contains(releaseJob, "container-publish") {
		t.Error("GitHub Release must run in parallel with container publication after the shared Go/Web gates")
	}
}

func TestMakeImageInjectsCanonicalVersionAndRevision(t *testing.T) {
	revision := "deadbeefcafe0123456789abcdef0123456789ab"
	output, err := runMake(
		t,
		"image",
		"DOCKER=echo",
		"SANDRONE_IMAGE=example.test/sandrone:test",
		"REVISION="+revision,
	)
	if err != nil {
		t.Fatalf("make image failed: %v\n%s", err, output)
	}
	for _, want := range []string{
		"--build-arg VERSION=0.1.1",
		"--build-arg REVISION=" + revision,
		"--label org.opencontainers.image.version=0.1.1",
		"--label org.opencontainers.image.revision=" + revision,
		"--tag example.test/sandrone:test",
	} {
		if !strings.Contains(string(output), want) {
			t.Errorf("make image output does not contain %q:\n%s", want, output)
		}
	}
}

func TestMakeImageWithoutRevisionUsesDevVersion(t *testing.T) {
	output, err := runMake(
		t,
		"image",
		"DOCKER=echo",
		"REVISION=",
	)
	if err != nil {
		t.Fatalf("make image failed: %v\n%s", err, output)
	}
	for _, want := range []string{
		"--build-arg VERSION=dev",
		"--build-arg REVISION=",
		"--label org.opencontainers.image.version=dev",
		"--label org.opencontainers.image.revision=",
		"--tag ghcr.io/kuuvahki-labs/sandrone:local",
	} {
		if !strings.Contains(string(output), want) {
			t.Errorf("make image output does not contain %q:\n%s", want, output)
		}
	}
}

func TestMakeImageUsesSharedImageVariable(t *testing.T) {
	revision := "deadbeefcafe0123456789abcdef0123456789ab"
	output, err := runMake(
		t,
		"image",
		"DOCKER=echo",
		"SANDRONE_IMAGE=example.test/sandrone:custom",
		"REVISION="+revision,
	)
	if err != nil {
		t.Fatalf("make image failed: %v\n%s", err, output)
	}
	if want := "--tag example.test/sandrone:custom"; !strings.Contains(string(output), want) {
		t.Fatalf("make image output does not contain %q:\n%s", want, output)
	}
}

func TestContainerImageTagsFollowReleasePolicy(t *testing.T) {
	script, err := filepath.Abs(filepath.Join("..", "..", "scripts", "container-image-tags.sh"))
	if err != nil {
		t.Fatal(err)
	}
	repo, revision := newGitRepo(t)
	runCommandOK(t, repo, "git", "tag", "v0.1.0")
	runCommandOK(t, repo, "git", "tag", "v0.2.0")
	runCommandOK(t, repo, "git", "tag", "v0.3.0-rc.1")
	versionFile := filepath.Join(repo, "VERSION")

	run := func(t *testing.T, event, refType, refName, version string) ([]byte, error) {
		t.Helper()
		if err := os.WriteFile(versionFile, []byte(version+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		return runCommandEnv(repo, []string{
			"IMAGE=example.test/sandrone",
			"GITHUB_SHA=" + revision,
			"GITHUB_EVENT_NAME=" + event,
			"GITHUB_REF_TYPE=" + refType,
			"GITHUB_REF_NAME=" + refName,
			"VERSION_FILE=" + versionFile,
		}, "sh", script)
	}

	t.Run("main publishes nothing", func(t *testing.T) {
		output, err := run(t, "push", "branch", "main", "0.2.0")
		if err != nil {
			t.Fatalf("plan main tags: %v\n%s", err, output)
		}
		if got := string(output); got != "" {
			t.Fatalf("main tags = %q, want empty", got)
		}
	})

	t.Run("newest release publishes version and latest", func(t *testing.T) {
		output, err := run(t, "push", "tag", "v0.2.0", "0.2.0")
		if err != nil {
			t.Fatalf("plan newest release tags: %v\n%s", err, output)
		}
		want := strings.Join([]string{
			"example.test/sandrone:v0.2.0",
			"example.test/sandrone:latest",
			"",
		}, "\n")
		if got := string(output); got != want {
			t.Fatalf("newest release tags = %q, want %q", got, want)
		}
	})

	t.Run("older release cannot replace latest", func(t *testing.T) {
		output, err := run(t, "push", "tag", "v0.1.0", "0.1.0")
		if err != nil {
			t.Fatalf("plan older release tags: %v\n%s", err, output)
		}
		want := "example.test/sandrone:v0.1.0\n"
		if got := string(output); got != want {
			t.Fatalf("older release tags = %q, want %q", got, want)
		}
	})

	t.Run("prerelease publishes its version without latest", func(t *testing.T) {
		output, err := run(t, "push", "tag", "v0.3.0-rc.1", "0.3.0-rc.1")
		if err != nil {
			t.Fatalf("plan prerelease tags: %v\n%s", err, output)
		}
		want := "example.test/sandrone:v0.3.0-rc.1\n"
		if got := string(output); got != want {
			t.Fatalf("prerelease tags = %q, want %q", got, want)
		}
	})

	t.Run("tag must match canonical version", func(t *testing.T) {
		output, err := run(t, "push", "tag", "v0.1.0", "0.2.0")
		if err == nil {
			t.Fatalf("mismatched release tag was accepted:\n%s", output)
		}
	})

	t.Run("release version rejects build metadata", func(t *testing.T) {
		output, err := run(t, "push", "tag", "v0.2.0+build", "0.2.0+build")
		if err == nil {
			t.Fatalf("release version with build metadata was accepted:\n%s", output)
		}
		if want := "release VERSION must contain only ASCII letters, digits, dots, and hyphens"; !strings.Contains(string(output), want) {
			t.Fatalf("release version error = %q, want it to contain %q", output, want)
		}
	})

	t.Run("release version accepts 127 characters", func(t *testing.T) {
		version := strings.Repeat("a", 127)
		output, err := run(t, "push", "tag", "v"+version, version)
		if err != nil {
			t.Fatalf("127-character release version was rejected: %v\n%s", err, output)
		}
		want := "example.test/sandrone:v" + version + "\n"
		if got := string(output); got != want {
			t.Fatalf("127-character release tags = %q, want %q", got, want)
		}
	})

	t.Run("release version rejects 128 characters", func(t *testing.T) {
		version := strings.Repeat("a", 128)
		output, err := run(t, "push", "tag", "v"+version, version)
		if err == nil {
			t.Fatalf("128-character release version was accepted:\n%s", output)
		}
		if want := "release VERSION must be at most 127 characters"; !strings.Contains(string(output), want) {
			t.Fatalf("release version error = %q, want it to contain %q", output, want)
		}
	})

	t.Run("non-push event publishes nothing", func(t *testing.T) {
		output, err := run(t, "pull_request", "branch", "feature", "0.2.0")
		if err != nil {
			t.Fatalf("plan pull request tags: %v\n%s", err, output)
		}
		if got := string(output); got != "" {
			t.Fatalf("pull request tags = %q, want empty", got)
		}
	})
}

func TestComposeDefaultsToPublishedLatestWithoutLocalBuild(t *testing.T) {
	compose, err := os.ReadFile(filepath.Join("..", "..", "docker-compose.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	content := string(compose)
	if want := `image: ${SANDRONE_IMAGE:-ghcr.io/kuuvahki-labs/sandrone:latest}`; !strings.Contains(content, want) {
		t.Errorf("docker-compose.yaml does not contain %q", want)
	}
	if strings.Contains(content, "\n    build:\n") {
		t.Error("published-image Compose config must not build the local worktree")
	}
}

func TestMakeRejectsUnsafeBuildVersions(t *testing.T) {
	for _, version := range []string{"v0. 1.0", "v0.1.0 ", "v0.1.0\"", "v0.1.0;next", "$HOME", "(v0.1.0)", "v版本"} {
		t.Run(version, func(t *testing.T) {
			if output, err := runMake(t, "help", "VERSION="+version); err == nil {
				t.Fatalf("make accepted unsafe VERSION %q:\n%s", version, output)
			}
		})
	}
}

func TestMakeVersionCannotExecuteMakeFunction(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "make-function-ran")
	value := "$(shell touch " + marker + ")"

	if output, err := runMake(t, "help", "VERSION="+value); err == nil {
		t.Errorf("make accepted executable VERSION:\n%s", output)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Errorf("VERSION executed a Make function; marker stat error = %v", err)
	}
}

func TestMakeVersionCannotInjectRecipeCommands(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "recipe-command-ran")
	value := "v0.1.0\"; touch \"" + marker + "\"; #"

	if output, err := runMake(t, "build-check", "GO=false", "VERSION="+value); err == nil {
		t.Errorf("make accepted injectable VERSION:\n%s", output)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Errorf("VERSION injected a recipe command; marker stat error = %v", err)
	}
}

func runMake(t *testing.T, args ...string) ([]byte, error) {
	t.Helper()
	return runMakeAt(filepath.Join("..", ".."), args...)
}

func runMakeAt(dir string, args ...string) ([]byte, error) {
	return runCommand(dir, "make", args...)
}

func newGitRepo(t *testing.T) (string, string) {
	t.Helper()
	repo := t.TempDir()
	runCommandOK(t, repo, "git", "init", "--quiet")
	runCommandOK(t, repo, "git", "config", "user.name", "Sandrone Test")
	runCommandOK(t, repo, "git", "config", "user.email", "test@example.invalid")
	if err := os.WriteFile(filepath.Join(repo, "tracked.txt"), []byte("clean\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runCommandOK(t, repo, "git", "add", "tracked.txt")
	runCommandOK(t, repo, "git", "commit", "--quiet", "-m", "test")
	revision := strings.TrimSpace(string(runCommandOK(t, repo, "git", "rev-parse", "HEAD")))
	return repo, revision
}

func newMakeFixtureRepo(t *testing.T) (string, string) {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	repo := t.TempDir()
	for _, name := range []string{
		"Makefile",
		filepath.Join("internal", "buildinfo", "VERSION"),
		filepath.Join("scripts", "resolve-build-revision.sh"),
		filepath.Join("scripts", "validate-build-revision.sh"),
		filepath.Join("scripts", "validate-build-version.sh"),
	} {
		content, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			t.Fatal(err)
		}
		target := filepath.Join(repo, name)
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, content, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	runCommandOK(t, repo, "git", "init", "--quiet")
	runCommandOK(t, repo, "git", "config", "user.name", "Sandrone Test")
	runCommandOK(t, repo, "git", "config", "user.email", "test@example.invalid")
	runCommandOK(t, repo, "git", "add", ".")
	runCommandOK(t, repo, "git", "commit", "--quiet", "-m", "test")
	revision := strings.TrimSpace(string(runCommandOK(t, repo, "git", "rev-parse", "HEAD")))
	return repo, revision
}

func runCommandOK(t *testing.T, dir, name string, args ...string) []byte {
	t.Helper()
	output, err := runCommand(dir, name, args...)
	if err != nil {
		t.Fatalf("%s %v: %v\n%s", name, args, err, output)
	}
	return output
}

func runCommand(dir, name string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	return cmd.CombinedOutput()
}

func runCommandEnv(dir string, env []string, name string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), env...)
	return cmd.CombinedOutput()
}
