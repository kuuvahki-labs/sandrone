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
		{name: "blank falls back to default", raw: " \t\n", want: "0.1.0"},
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
			"--build-arg VERSION=0.1.0",
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
	if got, want := string(versionOutput), "sandrone version 0.1.0 (deadbeefcafe)\n"; got != want {
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

func TestContainerWorkflowWiresBuildIdentityAndReleasePolicy(t *testing.T) {
	root := filepath.Join("..", "..")
	dockerfile, err := os.ReadFile(filepath.Join(root, "Dockerfile"))
	if err != nil {
		t.Fatal(err)
	}
	workflow, err := os.ReadFile(filepath.Join(root, ".github", "workflows", "ci.yml"))
	if err != nil {
		t.Fatal(err)
	}

	for _, want := range []string{
		`ARG VERSION="dev"`,
		"ARG REVISION",
		`if [ -z "$REVISION" ] && [ "$VERSION" != "dev" ]; then`,
		"org.opencontainers.image.source=https://github.com/kuuvahki-labs/sandrone",
	} {
		if !strings.Contains(string(dockerfile), want) {
			t.Errorf("Dockerfile does not contain %q", want)
		}
	}
	if labelAt, copyAt := strings.LastIndex(string(dockerfile), "LABEL "), strings.LastIndex(string(dockerfile), "COPY --from=web"); labelAt < copyAt {
		t.Error("Dockerfile OCI labels must follow runtime package and asset layers")
	}
	if want := `make image SANDRONE_IMAGE="${IMAGE}:sha-${short_sha}" REVISION="${GITHUB_SHA}"`; !strings.Contains(string(workflow), want) {
		t.Errorf("container workflow does not contain %q", want)
	}
	for _, want := range []string{
		`tags:`,
		`- "v*"`,
		`fetch-depth: 0`,
		`group: container-image-${{ github.event_name == 'push' && 'publish' || github.run_id }}`,
		`queue: max`,
		`git fetch --force --tags origin`,
		`./scripts/container-image-tags.sh`,
	} {
		if !strings.Contains(string(workflow), want) {
			t.Errorf("container workflow does not contain %q", want)
		}
	}
	if strings.Contains(string(workflow), `docker tag "${IMAGE}:sha-${short_sha}" "${IMAGE}:latest"`) {
		t.Error("container workflow must derive latest from the release tag policy")
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
		"--build-arg VERSION=0.1.0",
		"--build-arg REVISION=" + revision,
		"--label org.opencontainers.image.version=0.1.0",
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

	t.Run("main publishes only immutable revision tag", func(t *testing.T) {
		output, err := run(t, "push", "branch", "main", "0.2.0")
		if err != nil {
			t.Fatalf("plan main tags: %v\n%s", err, output)
		}
		want := "example.test/sandrone:sha-" + revision[:12] + "\n"
		if got := string(output); got != want {
			t.Fatalf("main tags = %q, want %q", got, want)
		}
	})

	t.Run("newest release publishes version and latest", func(t *testing.T) {
		output, err := run(t, "push", "tag", "v0.2.0", "0.2.0")
		if err != nil {
			t.Fatalf("plan newest release tags: %v\n%s", err, output)
		}
		want := strings.Join([]string{
			"example.test/sandrone:sha-" + revision[:12],
			"example.test/sandrone:0.2.0",
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
		want := strings.Join([]string{
			"example.test/sandrone:sha-" + revision[:12],
			"example.test/sandrone:0.1.0",
			"",
		}, "\n")
		if got := string(output); got != want {
			t.Fatalf("older release tags = %q, want %q", got, want)
		}
	})

	t.Run("prerelease publishes its version without latest", func(t *testing.T) {
		output, err := run(t, "push", "tag", "v0.3.0-rc.1", "0.3.0-rc.1")
		if err != nil {
			t.Fatalf("plan prerelease tags: %v\n%s", err, output)
		}
		want := strings.Join([]string{
			"example.test/sandrone:sha-" + revision[:12],
			"example.test/sandrone:0.3.0-rc.1",
			"",
		}, "\n")
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

	if output, err := runMake(t, "build-probe-mihomo", "GO=false", "VERSION="+value); err == nil {
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
