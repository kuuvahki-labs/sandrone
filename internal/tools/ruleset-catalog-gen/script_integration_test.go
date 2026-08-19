package main

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGenerationScriptTracksBlackmatrixMaster(t *testing.T) {
	root := t.TempDir()
	metaRepository := filepath.Join(root, "meta-rules-dat")
	initGitRepository(t, metaRepository, "meta")
	writeGitFile(t, metaRepository, "geo/geosite/cn.mrs", "")
	commitGitFiles(t, metaRepository, "meta catalog")
	runGit(t, "-C", metaRepository, "checkout", "--orphan", "sing")
	runGit(t, "-C", metaRepository, "rm", "-rf", ".")
	writeGitFile(t, metaRepository, "geo/geosite/cn.srs", "")
	commitGitFiles(t, metaRepository, "sing catalog")

	blackmatrixRepository := filepath.Join(root, "ios_rule_script")
	initGitRepository(t, blackmatrixRepository, "stable")
	writeShadowrocketCategory(t, blackmatrixRepository, "Stale")
	commitGitFiles(t, blackmatrixRepository, "stable catalog")
	runGit(t, "-C", blackmatrixRepository, "checkout", "-b", "master")
	require.NoError(t, os.RemoveAll(filepath.Join(blackmatrixRepository, "rule")))
	writeShadowrocketCategory(t, blackmatrixRepository, "Live")
	commitGitFiles(t, blackmatrixRepository, "master catalog")
	runGit(t, "-C", blackmatrixRepository, "checkout", "stable")

	gitConfig := filepath.Join(root, "gitconfig")
	runGit(t, "config", "--file", gitConfig,
		"url.file://"+filepath.ToSlash(metaRepository)+".insteadOf",
		"https://github.com/MetaCubeX/meta-rules-dat.git")
	runGit(t, "config", "--file", gitConfig,
		"url.file://"+filepath.ToSlash(blackmatrixRepository)+".insteadOf",
		"https://github.com/blackmatrix7/ios_rule_script.git")
	mirror := "https://mirror.example/https://github.com"
	runGit(t, "config", "--file", gitConfig, "--add",
		"url.file://"+filepath.ToSlash(metaRepository)+".insteadOf",
		mirror+"/MetaCubeX/meta-rules-dat.git")
	runGit(t, "config", "--file", gitConfig, "--add",
		"url.file://"+filepath.ToSlash(blackmatrixRepository)+".insteadOf",
		mirror+"/blackmatrix7/ios_rule_script.git")

	repositoryRoot, err := filepath.Abs("../../..")
	require.NoError(t, err)
	for _, test := range []struct {
		name   string
		mirror string
	}{
		{name: "default GitHub"},
		{name: "configured mirror", mirror: mirror + "/"},
	} {
		t.Run(test.name, func(t *testing.T) {
			outputDirectory := filepath.Join(root, "output-"+test.name)
			command := exec.Command(
				"bash",
				filepath.Join(repositoryRoot, "scripts/generate-ruleset-catalog.sh"),
				outputDirectory,
			)
			command.Dir = repositoryRoot
			command.Env = append(os.Environ(),
				"GIT_CONFIG_GLOBAL="+gitConfig,
				"GIT_CONFIG_NOSYSTEM=1",
			)
			if test.mirror != "" {
				command.Env = append(command.Env, "RULESET_CATALOG_GITHUB_MIRROR="+test.mirror)
			}
			output, err := command.CombinedOutput()
			require.NoError(t, err, string(output))

			body, err := os.ReadFile(filepath.Join(outputDirectory, "catalog.json.gz"))
			require.NoError(t, err)
			reader, err := gzip.NewReader(bytes.NewReader(body))
			require.NoError(t, err)
			decodedBody, err := io.ReadAll(reader)
			require.NoError(t, err)
			require.NoError(t, reader.Close())
			var catalog catalogSnapshot
			require.NoError(t, json.Unmarshal(decodedBody, &catalog))
			require.Equal(t, []string{"Live/Live"}, itemNames(catalog.Shadowrocket))
		})
	}
}

func initGitRepository(t *testing.T, repository, branch string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(repository, 0o755))
	runGit(t, "init", "--quiet", "-b", branch, repository)
}

func writeGitFile(t *testing.T, repository, name, body string) {
	t.Helper()
	filename := filepath.Join(repository, filepath.FromSlash(name))
	require.NoError(t, os.MkdirAll(filepath.Dir(filename), 0o755))
	require.NoError(t, os.WriteFile(filename, []byte(body), 0o644))
}

func writeShadowrocketCategory(t *testing.T, repository, name string) {
	t.Helper()
	writeGitFile(t, repository, "rule/Shadowrocket/"+name+"/README.md",
		"#### 使用说明\n- "+name+".list，请使用RULE-SET。\n")
	writeGitFile(t, repository, "rule/Shadowrocket/"+name+"/"+name+".list",
		"DOMAIN-SUFFIX,example.com\n")
}

func commitGitFiles(t *testing.T, repository, message string) {
	t.Helper()
	runGit(t, "-C", repository, "add", "-A")
	runGit(t, "-C", repository,
		"-c", "user.name=Sandrone Test",
		"-c", "user.email=sandrone-test@example.invalid",
		"commit", "--quiet", "-m", message)
}

func runGit(t *testing.T, arguments ...string) {
	t.Helper()
	command := exec.Command("git", arguments...)
	output, err := command.CombinedOutput()
	require.NoError(t, err, string(output))
}
