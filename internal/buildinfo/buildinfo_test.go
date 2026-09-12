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
	canonical := canonicalVersion(t)

	tests := []struct {
		name string
		raw  string
		want string
	}{
		{name: "default", raw: "0.1.0", want: "0.1.0"},
		{name: "blank falls back to default", raw: " \t\n", want: canonical},
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

func TestBuildTime(t *testing.T) {
	originalBuildTime := rawBuildTime
	t.Cleanup(func() { rawBuildTime = originalBuildTime })

	rawBuildTime = " 2026-08-30T03:15:42Z "
	if got := BuildTime(); got != "2026-08-30T03:15:42Z" {
		t.Fatalf("BuildTime() = %q, want RFC3339 build time", got)
	}
}

func TestSummaryIncludesBuildTime(t *testing.T) {
	originalVersion := rawVersion
	originalRevision := rawRevision
	originalBuildTime := rawBuildTime
	originalReadBuildInfo := readBuildInfo
	t.Cleanup(func() {
		rawVersion = originalVersion
		rawRevision = originalRevision
		rawBuildTime = originalBuildTime
		readBuildInfo = originalReadBuildInfo
	})
	readBuildInfo = func() (*debug.BuildInfo, bool) { return nil, false }
	rawBuildTime = "2026-08-30T03:15:42Z"

	rawVersion = "dev"
	rawRevision = ""
	if got := Summary(); got != "dev (2026-08-30T03:15:42Z)" {
		t.Fatalf("Summary() = %q, want development build time", got)
	}

	rawVersion = "0.1.0"
	rawRevision = "0123456789abcdef"
	if got := Summary(); got != "0.1.0 (0123456789ab; 2026-08-30T03:15:42Z)" {
		t.Fatalf("Summary() = %q, want release revision and build time", got)
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

func canonicalVersion(t *testing.T) string {
	t.Helper()
	version := strings.TrimSpace(defaultVersion)
	if version == "" {
		t.Fatal("embedded VERSION is empty")
	}
	return version
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

func TestMakeValidatesRFC3339UTCBuildTime(t *testing.T) {
	if output, err := runMake(t, "help", "BUILD_TIME=2026-08-30T03:15:42Z"); err != nil {
		t.Fatalf("make rejected RFC3339 UTC build time: %v\n%s", err, output)
	}
	for _, buildTime := range []string{"", "20260830-031542", "2026-08-30T03:15:42+08:00", "$(whoami)"} {
		t.Run(buildTime, func(t *testing.T) {
			if output, err := runMake(t, "help", "BUILD_TIME="+buildTime); err == nil {
				t.Fatalf("make accepted invalid BUILD_TIME %q:\n%s", buildTime, output)
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
	canonical := canonicalVersion(t)

	t.Run("clean worktree", func(t *testing.T) {
		repo, revision := newMakeFixtureRepo(t)
		output, err := runMakeAt(repo, "image", "DOCKER=echo")
		if err != nil {
			t.Fatalf("make image in clean worktree: %v\n%s", err, output)
		}
		for _, want := range []string{
			"--build-arg VERSION=" + canonical,
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
	buildTime := "2026-08-30T03:15:42Z"

	output, err := runMake(t, "build-check", "BUILD_BIN="+binary, "REVISION="+revision, "BUILD_TIME="+buildTime)
	if err != nil {
		t.Fatalf("make build-check failed: %v\n%s", err, output)
	}
	command := exec.Command(binary, "--version")
	versionOutput, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("built binary --version failed: %v\n%s", err, versionOutput)
	}
	if got, want := string(versionOutput), "sandrone version "+canonicalVersion(t)+" (deadbeefcafe; "+buildTime+")\n"; got != want {
		t.Fatalf("built binary --version = %q, want %q", got, want)
	}
}

func TestMakeBuildWithoutRevisionForcesDevVersion(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "sandrone")
	buildTime := "2026-08-30T03:15:42Z"
	output, err := runMake(
		t,
		"build-check",
		"BUILD_BIN="+binary,
		"VERSION=9.9.9",
		"REVISION=",
		"BUILD_TIME="+buildTime,
	)
	if err != nil {
		t.Fatalf("make build-check failed: %v\n%s", err, output)
	}
	command := exec.Command(binary, "--version")
	versionOutput, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("built binary --version failed: %v\n%s", err, versionOutput)
	}
	if got, want := string(versionOutput), "sandrone version dev ("+buildTime+")\n"; got != want {
		t.Fatalf("built binary --version = %q, want %q", got, want)
	}
}

func TestArtifactTargetsUseCanonicalScript(t *testing.T) {
	for _, target := range []string{"release-artifacts", "snapshot-artifacts"} {
		t.Run(target, func(t *testing.T) {
			repo, revision := newMakeFixtureRepo(t)
			log := filepath.Join(repo, "calls.log")
			for name, body := range map[string]string{
				"build-webui.sh":             "#!/bin/sh\nprintf 'web\\n' >> calls.log\n",
				"build-release-artifacts.sh": "#!/bin/sh\nprintf '%s|%s|%s|%s|%s\\n' \"$ARTIFACT_KIND\" \"$VERSION\" \"$REVISION\" \"$BUILD_TIME\" \"${OUTPUT_DIR-}\" >> calls.log\n",
			} {
				path := filepath.Join(repo, "scripts", name)
				writeTestFile(t, path, body)
				if err := os.Chmod(path, 0o700); err != nil {
					t.Fatal(err)
				}
			}
			args := []string{target, "VERSION=1.2.3", "REVISION=" + revision, "BUILD_TIME=2026-08-30T03:15:42Z"}
			if output, err := runMakeAt(repo, args...); err != nil {
				t.Fatalf("make: %v\n%s", err, output)
			}
			body, err := os.ReadFile(log)
			if err != nil {
				t.Fatal(err)
			}
			want := "web\nrelease|1.2.3|" + revision + "|2026-08-30T03:15:42Z|\n"
			if target == "snapshot-artifacts" {
				want = "web\nsnapshot|dev||2026-08-30T03:15:42Z|" + filepath.Join(repo, "dist", "snapshot") + "\n"
			}
			if string(body) != want {
				t.Fatalf("calls = %q, want %q", body, want)
			}
			if err := os.Remove(log); err != nil {
				t.Fatal(err)
			}
			if output, err := runMakeAt(repo, target, "VERSION=unsafe/version", "REVISION="+revision); err == nil {
				t.Fatalf("unsafe version accepted: %s", output)
			}
			if _, err := os.Stat(log); !os.IsNotExist(err) {
				t.Fatalf("invalid identity must prevent script execution: %v", err)
			}
		})
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
	for _, want := range []string{" VERSION=1.2.3", " REVISION=" + revision, " BUILD_TIME="} {
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
	for _, want := range []string{" VERSION=dev", " REVISION=", " BUILD_TIME="} {
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
		filepath.Join("scripts", "validate-build-identity.sh"),
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
	for _, want := range []string{
		`FROM --platform=$BUILDPLATFORM node:24.17.0-bookworm AS web`,
		`FROM --platform=$BUILDPLATFORM golang:1.27.0-bookworm AS build`,
		`ARG VERSION="dev"`,
		"ARG REVISION",
		"ARG BUILD_TIME",
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

	testCIContracts(t)
}

func TestManualReleaseWorkflowIncrementsPatchAndDispatchesTagCI(t *testing.T) {
	workflow := readWorkflow(t, "release-tag.yml")
	if len(workflow.On) != 1 {
		t.Errorf("manual release triggers = %v", workflow.On)
	}
	dispatch, ok := workflow.On["workflow_dispatch"]
	if !ok {
		t.Fatal("missing manual dispatch")
	}
	if len(dispatch.Content) != 0 {
		t.Error("manual release must not take version inputs")
	}
	requireFields(t, workflow.Permissions, map[string]string{"actions": "write", "contents": "write"})
	if workflow.Concurrency.Group != "create-release-tag" || workflow.Concurrency.Cancel || workflow.Concurrency.Queue != "max" {
		t.Errorf("manual release concurrency = %+v", workflow.Concurrency)
	}
	job := jobByName(t, workflow, "release-tag")
	requireFields(t, stepByAction(t, job, "actions/checkout").With, map[string]string{"fetch-depth": "0"})
	stepByRun(t, job, `if [[ "${GITHUB_REF}" != "refs/heads/main" ]]`)
	resolve := stepByRun(t, job, "./scripts/release.sh next-version")
	requireCommands(t, resolve.Run, `git show-ref --verify --quiet "refs/tags/${tag}"`, `printf '%s\n' "${version}" > internal/buildinfo/VERSION`, `GITHUB_REF_NAME="${tag}" sh ./scripts/release.sh validate-tag`)
	commit := stepByRun(t, job, "git push --atomic")
	requireCommands(t, commit.Run, `git commit -m "chore(release): bump version to ${RELEASE_VERSION}"`, `git tag -a "${RELEASE_TAG}" -m "Release ${RELEASE_TAG}"`, `git push --atomic origin HEAD:refs/heads/main "refs/tags/${RELEASE_TAG}"`)
	dispatchCI := stepByRun(t, job, `gh workflow run ci.yml --ref "${RELEASE_TAG}"`)
	requireFields(t, dispatchCI.Env, map[string]string{"GH_TOKEN": "${{ github.token }}", "RELEASE_TAG": "${{ steps.release.outputs.tag }}"})
}

func TestNextReleaseVersionIncrementsLatestStablePatch(t *testing.T) {
	script, err := filepath.Abs(filepath.Join("..", "..", "scripts", "release.sh"))
	if err != nil {
		t.Fatal(err)
	}

	t.Run("latest stable tag", func(t *testing.T) {
		repo, _ := newGitRepo(t)
		for _, tag := range []string{"v0.9.9", "v1.2.9", "v1.3.0-rc.1", "not-a-version"} {
			runCommandOK(t, repo, "git", "tag", tag)
		}
		output := runCommandOK(t, repo, "sh", script, "next-version")
		if got, want := string(output), "1.2.10\n"; got != want {
			t.Fatalf("next release version = %q, want %q", got, want)
		}
	})

	t.Run("no stable tag", func(t *testing.T) {
		repo, _ := newGitRepo(t)
		runCommandOK(t, repo, "git", "tag", "v1.0.0-rc.1")
		output, err := runCommand(repo, "sh", script, "next-version")
		if err == nil {
			t.Fatalf("missing stable release tag was accepted:\n%s", output)
		}
		if want := "no stable release tag found"; !strings.Contains(string(output), want) {
			t.Fatalf("missing stable tag error = %q, want it to contain %q", output, want)
		}
	})
}

func TestValidateReleaseTagMatchesVersionFile(t *testing.T) {
	script, err := filepath.Abs(filepath.Join("..", "..", "scripts", "release.sh"))
	if err != nil {
		t.Fatal(err)
	}
	versionFile := filepath.Join(t.TempDir(), "VERSION")
	run := func(t *testing.T, version, refName string) ([]byte, error) {
		t.Helper()
		if err := os.WriteFile(versionFile, []byte(version+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		return runCommandEnv(".", []string{
			"GITHUB_REF_NAME=" + refName,
			"VERSION_FILE=" + versionFile,
		}, "sh", script, "validate-tag")
	}

	t.Run("matching tag", func(t *testing.T) {
		output, err := run(t, "0.1.3", "v0.1.3")
		if err != nil {
			t.Fatalf("matching release tag failed: %v\n%s", err, output)
		}
		if got := string(output); got != "" {
			t.Fatalf("matching release tag output = %q, want empty", got)
		}
	})

	t.Run("mismatched tag", func(t *testing.T) {
		output, err := run(t, "0.1.2", "v0.1.3")
		if err == nil {
			t.Fatalf("mismatched release tag was accepted:\n%s", output)
		}
		if want := "release tag v0.1.3 does not match VERSION v0.1.2"; !strings.Contains(string(output), want) {
			t.Fatalf("release tag error = %q, want it to contain %q", output, want)
		}
	})
}

func TestMakeImageInjectsCanonicalVersionAndRevision(t *testing.T) {
	revision := "deadbeefcafe0123456789abcdef0123456789ab"
	buildTime := "2026-08-30T03:15:42Z"
	canonical := canonicalVersion(t)
	output, err := runMake(
		t,
		"image",
		"DOCKER=echo",
		"SANDRONE_IMAGE=example.test/sandrone:test",
		"REVISION="+revision,
		"BUILD_TIME="+buildTime,
	)
	if err != nil {
		t.Fatalf("make image failed: %v\n%s", err, output)
	}
	for _, want := range []string{
		"--build-arg VERSION=" + canonical,
		"--build-arg REVISION=" + revision,
		"--build-arg BUILD_TIME=" + buildTime,
		"--label org.opencontainers.image.version=" + canonical,
		"--label org.opencontainers.image.revision=" + revision,
		"--tag example.test/sandrone:test",
	} {
		if !strings.Contains(string(output), want) {
			t.Errorf("make image output does not contain %q:\n%s", want, output)
		}
	}
}

func TestMakeImageWithoutRevisionUsesDevVersion(t *testing.T) {
	buildTime := "2026-08-30T03:15:42Z"
	output, err := runMake(
		t,
		"image",
		"DOCKER=echo",
		"REVISION=",
		"BUILD_TIME="+buildTime,
	)
	if err != nil {
		t.Fatalf("make image failed: %v\n%s", err, output)
	}
	for _, want := range []string{
		"--build-arg VERSION=dev",
		"--build-arg REVISION=",
		"--build-arg BUILD_TIME=" + buildTime,
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
	script, err := filepath.Abs(filepath.Join("..", "..", "scripts", "release.sh"))
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
		}, "sh", script, "image-tags")
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

	t.Run("tag workflow dispatch publishes version and latest", func(t *testing.T) {
		output, err := run(t, "workflow_dispatch", "tag", "v0.2.0", "0.2.0")
		if err != nil {
			t.Fatalf("plan manually dispatched release tags: %v\n%s", err, output)
		}
		want := strings.Join([]string{
			"example.test/sandrone:v0.2.0",
			"example.test/sandrone:latest",
			"",
		}, "\n")
		if got := string(output); got != want {
			t.Fatalf("manually dispatched release tags = %q, want %q", got, want)
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
		filepath.Join("scripts", "validate-build-identity.sh"),
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
