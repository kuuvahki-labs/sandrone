package buildinfo

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

type workflowSpec struct {
	On          map[string]yaml.Node   `yaml:"on"`
	Permissions map[string]string      `yaml:"permissions"`
	Jobs        map[string]workflowJob `yaml:"jobs"`
	Concurrency workflowConcurrency    `yaml:"concurrency"`
}
type workflowJob struct {
	If          string              `yaml:"if"`
	Needs       []string            `yaml:"needs"`
	Permissions map[string]string   `yaml:"permissions"`
	Env         map[string]string   `yaml:"env"`
	Concurrency workflowConcurrency `yaml:"concurrency"`
	Steps       []workflowStep      `yaml:"steps"`
}
type workflowConcurrency struct {
	Group  string `yaml:"group"`
	Cancel bool   `yaml:"cancel-in-progress"`
	Queue  string `yaml:"queue"`
}
type workflowStep struct {
	Name string            `yaml:"name"`
	ID   string            `yaml:"id"`
	Uses string            `yaml:"uses"`
	Run  string            `yaml:"run"`
	With map[string]string `yaml:"with"`
	Env  map[string]string `yaml:"env"`
}

func readWorkflow(t *testing.T, name string) workflowSpec {
	t.Helper()
	body, err := os.ReadFile(filepath.Join("..", "..", ".github", "workflows", name))
	if err != nil {
		t.Fatal(err)
	}
	var workflow workflowSpec
	if err := yaml.Unmarshal(body, &workflow); err != nil {
		t.Fatal(err)
	}
	return workflow
}
func jobByName(t *testing.T, workflow workflowSpec, name string) workflowJob {
	t.Helper()
	job, ok := workflow.Jobs[name]
	if !ok {
		t.Fatalf("missing job %q", name)
	}
	return job
}
func stepByAction(t *testing.T, job workflowJob, action string) workflowStep {
	t.Helper()
	for _, step := range job.Steps {
		if strings.HasPrefix(step.Uses, action+"@") {
			return step
		}
	}
	t.Fatalf("missing action %q", action)
	return workflowStep{}
}
func stepByRun(t *testing.T, job workflowJob, command string) workflowStep {
	t.Helper()
	for _, step := range job.Steps {
		if strings.Contains(step.Run, command) {
			return step
		}
	}
	t.Fatalf("missing command %q", command)
	return workflowStep{}
}
func requireFields(t *testing.T, got, want map[string]string) {
	t.Helper()
	for key, value := range want {
		if got[key] != value {
			t.Errorf("%s = %q, want %q", key, got[key], value)
		}
	}
}
func requireCommands(t *testing.T, run string, commands ...string) {
	t.Helper()
	for _, command := range commands {
		if !strings.Contains(run, command) {
			t.Errorf("script missing %q", command)
		}
	}
}
func requireReleaseGate(t *testing.T, job workflowJob, group string) {
	t.Helper()
	if job.If != "github.ref_type == 'tag'" {
		t.Errorf("publication condition = %q", job.If)
	}
	needs := slices.Sorted(slices.Values(job.Needs))
	if !slices.Equal(needs, []string{"go", "web"}) {
		t.Errorf("publication needs = %v", needs)
	}
	if job.Concurrency.Group != group || job.Concurrency.Cancel || job.Concurrency.Queue != "max" {
		t.Errorf("publication concurrency = %+v", job.Concurrency)
	}
}
func testCIContracts(t *testing.T) {
	workflow := readWorkflow(t, "ci.yml")
	requireFields(t, workflow.Permissions, map[string]string{"contents": "read"})
	for _, event := range []string{"pull_request", "push", "workflow_dispatch"} {
		if _, ok := workflow.On[event]; !ok {
			t.Errorf("missing trigger %s", event)
		}
	}
	var push struct {
		Branches []string `yaml:"branches"`
		Tags     []string `yaml:"tags"`
	}
	node := workflow.On["push"]
	if err := node.Decode(&push); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(push.Branches, []string{"main"}) || !slices.Equal(push.Tags, []string{"v*"}) {
		t.Errorf("push trigger = %+v", push)
	}
	check := jobByName(t, workflow, "container-check")
	if check.If != "github.ref_type != 'tag'" || len(check.Needs) != 0 || check.Permissions["packages"] == "write" {
		t.Errorf("container check gate/permissions = %+v", check)
	}
	for _, step := range check.Steps {
		if strings.Contains(step.Uses, "login-action") || strings.Contains(step.Uses, "setup-qemu-action") || strings.Contains(step.Run, "docker push") {
			t.Errorf("check must not publish or emulate: %+v", step)
		}
	}
	publish := jobByName(t, workflow, "container-publish")
	requireReleaseGate(t, publish, "container-image-publish")
	requireFields(t, publish.Permissions, map[string]string{"contents": "read", "packages": "write"})
	requireFields(t, stepByAction(t, publish, "actions/checkout").With, map[string]string{"fetch-depth": "0"})
	stepByAction(t, publish, "docker/setup-qemu-action")
	requireFields(t, stepByAction(t, publish, "docker/login-action").With, map[string]string{"registry": "ghcr.io", "username": "${{ github.actor }}", "password": "${{ secrets.GITHUB_TOKEN }}"})
	for name, job := range map[string]workflowJob{"check": check, "publish": publish} {
		t.Run(name, func(t *testing.T) {
			stepByAction(t, job, "docker/setup-buildx-action")
			build := stepByAction(t, job, "docker/build-push-action")
			fields := map[string]string{"context": ".", "platforms": "linux/amd64", "push": "false", "tags": "${{ env.IMAGE }}:ci", "cache-from": "type=gha,scope=sandrone-container", "cache-to": "type=gha,mode=max,scope=sandrone-container"}
			if name == "publish" {
				fields["platforms"] = "linux/amd64,linux/arm64"
				fields["push"] = "true"
				fields["tags"] = "${{ steps.image-metadata.outputs.tags }}"
			}
			requireFields(t, build.With, fields)
			requireCommands(t, build.With["build-args"], "VERSION=${{ steps.image-metadata.outputs.version }}", "REVISION=${{ github.sha }}", "BUILD_TIME=${{ steps.image-metadata.outputs.build_time }}")
			requireCommands(t, build.With["labels"], "org.opencontainers.image.version=${{ steps.image-metadata.outputs.version }}", "org.opencontainers.image.revision=${{ github.sha }}")
			metadata := stepByRun(t, job, "internal/buildinfo/VERSION")
			if metadata.ID != "image-metadata" {
				t.Errorf("metadata id = %q", metadata.ID)
			}
			requireCommands(t, metadata.Run, "GITHUB_OUTPUT", "build_time=", "version=")
			if name == "publish" {
				requireCommands(t, metadata.Run, "git fetch --force --tags origin", "./scripts/release.sh image-tags")
			}
		})
	}
	release := jobByName(t, workflow, "release")
	requireReleaseGate(t, release, "github-release-publish")
	requireFields(t, release.Permissions, map[string]string{"contents": "write"})
	requireFields(t, stepByAction(t, release, "actions/checkout").With, map[string]string{"fetch-depth": "0"})
	requireFields(t, stepByAction(t, release, "actions/setup-go").With, map[string]string{"go-version-file": "go.mod"})
	stepByRun(t, release, "sh ./scripts/release.sh validate-tag")
	stepByRun(t, release, `make release-artifacts REVISION="${GITHUB_SHA}"`)
	publication := stepByRun(t, release, "gh release create")
	requireFields(t, publication.Env, map[string]string{"GH_TOKEN": "${{ secrets.GITHUB_TOKEN }}"})
	requireCommands(t, publication.Run, "dist/sandrone_linux_amd64.tar.gz", "dist/sandrone_linux_arm64.tar.gz", "dist/checksums.txt", `gh release upload "${GITHUB_REF_NAME}" "${artifacts[@]}" --clobber`, `gh release create "${GITHUB_REF_NAME}" "${artifacts[@]}" --verify-tag --generate-notes ${prerelease_flag}`, `prerelease_flag="--prerelease"`, `^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)
	testWebArtifactContract(t, workflow)
}
func testWebArtifactContract(t *testing.T, workflow workflowSpec) {
	t.Helper()
	web := jobByName(t, workflow, "web")
	upload := stepByAction(t, web, "actions/upload-artifact")
	requireFields(t, upload.With, map[string]string{"name": "web-client", "path": "web/build/client", "if-no-files-found": "error", "include-hidden-files": "true"})
	tested := false
	for _, step := range web.Steps {
		if strings.Contains(step.Run, "pnpm test:e2e") {
			tested = true
		}
		if step.Uses == upload.Uses && !tested {
			t.Error("Web assets must be tested before upload")
		}
	}
	for _, name := range []string{"release", "vercel"} {
		job := jobByName(t, workflow, name)
		download := stepByAction(t, job, "actions/download-artifact")
		requireFields(t, download.With, map[string]string{"name": "web-client", "path": "web/build/client", "digest-mismatch": "error"})
		command := "make release-artifacts"
		if name == "vercel" {
			command = "./scripts/vercel-assets.sh build"
		}
		build := stepByRun(t, job, command)
		if build.Env["WEBUI_PREBUILT_DIR"] == "" && job.Env["WEBUI_PREBUILT_DIR"] == "" && !strings.Contains(build.Run, "WEBUI_PREBUILT_DIR=") {
			t.Errorf("%s must consume prebuilt assets", name)
		}
		downloaded := false
		for _, step := range job.Steps {
			if step.Uses == download.Uses {
				downloaded = true
			}
			if strings.Contains(step.Run, command) && !downloaded {
				t.Errorf("%s builds before downloading assets", name)
			}
		}
	}
}
