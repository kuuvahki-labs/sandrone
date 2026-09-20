package main

import (
	"bytes"
	"compress/gzip"
	"encoding/json/v2"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGenerationScriptUsesLockedSourcesAndRefreshesThem(t *testing.T) {
	root := t.TempDir()
	metaRepository := filepath.Join(root, "meta-rules-dat")
	initGitRepository(t, metaRepository, "meta")
	writeGitFile(t, metaRepository, "geo/geosite/cn.mrs", "")
	commitGitFiles(t, metaRepository, "meta catalog")
	lockedMetaRevision := gitRevision(t, metaRepository)
	writeGitFile(t, metaRepository, "geo/geosite/latest.mrs", "")
	commitGitFiles(t, metaRepository, "new meta catalog")
	latestMetaRevision := gitRevision(t, metaRepository)
	runGit(t, "-C", metaRepository, "checkout", "--orphan", "sing")
	runGit(t, "-C", metaRepository, "rm", "-rf", ".")
	writeGitFile(t, metaRepository, "geo/geosite/cn.srs", "")
	commitGitFiles(t, metaRepository, "sing catalog")
	lockedSingRevision := gitRevision(t, metaRepository)
	writeGitFile(t, metaRepository, "geo/geosite/latest.srs", "")
	commitGitFiles(t, metaRepository, "new sing catalog")
	latestSingRevision := gitRevision(t, metaRepository)

	blackmatrixRepository := filepath.Join(root, "ios_rule_script")
	initGitRepository(t, blackmatrixRepository, "stable")
	writeShadowrocketCategory(t, blackmatrixRepository, "Stale")
	commitGitFiles(t, blackmatrixRepository, "stable catalog")
	runGit(t, "-C", blackmatrixRepository, "checkout", "-b", "master")
	require.NoError(t, os.RemoveAll(filepath.Join(blackmatrixRepository, "rule")))
	writeShadowrocketCategory(t, blackmatrixRepository, "Live")
	commitGitFiles(t, blackmatrixRepository, "master catalog")
	lockedShadowrocketRevision := gitRevision(t, blackmatrixRepository)
	writeShadowrocketCategory(t, blackmatrixRepository, "Latest")
	commitGitFiles(t, blackmatrixRepository, "new master catalog")
	latestShadowrocketRevision := gitRevision(t, blackmatrixRepository)
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
			cacheDirectory := filepath.Join(root, "cache-"+test.name)
			environment := []string{
				"GIT_CONFIG_GLOBAL=" + gitConfig,
				"GIT_CONFIG_NOSYSTEM=1",
				"RULESET_CATALOG_CACHE_DIR=" + cacheDirectory,
			}
			if test.mirror != "" {
				environment = append(environment, "RULESET_CATALOG_GITHUB_MIRROR="+test.mirror)
			}

			lockedSources := filepath.Join(root, "locked-"+test.name+".lock")
			writeSourcesLock(t, lockedSources, lockedMetaRevision, lockedSingRevision, lockedShadowrocketRevision)
			lockedOutput := filepath.Join(root, "locked-output-"+test.name)
			runCatalogScript(t, repositoryRoot, append(environment,
				"RULESET_CATALOG_SOURCES_FILE="+lockedSources,
			), "generate", lockedOutput)
			catalog := readCatalog(t, filepath.Join(lockedOutput, "catalog.json.gz"))
			require.Equal(t, []string{"geosite-cn"}, itemNames(catalog.Mihomo))
			require.Equal(t, []string{"geosite-cn"}, itemNames(catalog.SingBox))
			require.Equal(t, []string{"Live/Live"}, itemNames(catalog.Shadowrocket))

			refreshedSources := filepath.Join(root, "refreshed-"+test.name+".lock")
			runCatalogScript(t, repositoryRoot, environment, "update-sources", refreshedSources)
			wantSources := sourcesLockBody(latestMetaRevision, latestSingRevision, latestShadowrocketRevision)
			body, err := os.ReadFile(refreshedSources)
			require.NoError(t, err)
			require.Equal(t, wantSources, string(body))

			latestOutput := filepath.Join(root, "latest-output-"+test.name)
			runCatalogScript(t, repositoryRoot, append(environment,
				"RULESET_CATALOG_SOURCES_FILE="+refreshedSources,
			), "generate", latestOutput)
			catalog = readCatalog(t, filepath.Join(latestOutput, "catalog.json.gz"))
			require.Equal(t, []string{"geosite-cn", "geosite-latest"}, itemNames(catalog.Mihomo))
			require.Equal(t, []string{"geosite-cn", "geosite-latest"}, itemNames(catalog.SingBox))
			require.Equal(t, []string{"Latest/Latest", "Live/Live"}, itemNames(catalog.Shadowrocket))

			offlineOutput := filepath.Join(root, "offline-output-"+test.name)
			runCatalogScript(t, repositoryRoot, []string{
				"GIT_CONFIG_GLOBAL=" + filepath.Join(root, "missing-gitconfig"),
				"GIT_CONFIG_NOSYSTEM=1",
				"RULESET_CATALOG_CACHE_DIR=" + cacheDirectory,
				"RULESET_CATALOG_GITHUB_MIRROR=https://offline.invalid",
				"RULESET_CATALOG_SOURCES_FILE=" + refreshedSources,
			}, "generate", offlineOutput)
			require.Equal(t,
				readCatalog(t, filepath.Join(latestOutput, "catalog.json.gz")),
				readCatalog(t, filepath.Join(offlineOutput, "catalog.json.gz")),
			)
		})
	}
}

func TestGenerationScriptRejectsInvalidSourceLocks(t *testing.T) {
	repositoryRoot, err := filepath.Abs("../../..")
	require.NoError(t, err)
	revision := strings.Repeat("a", 40)
	for _, test := range []struct {
		name      string
		body      string
		wantError string
	}{
		{
			name: "malformed revision",
			body: "METACUBEX_META_REVISION=not-a-revision\n" +
				"METACUBEX_SING_REVISION=" + revision + "\n" +
				"SHADOWROCKET_REVISION=" + revision + "\n",
			wantError: "must be a lowercase 40-character Git object ID",
		},
		{
			name: "missing source",
			body: "METACUBEX_META_REVISION=" + revision + "\n" +
				"SHADOWROCKET_REVISION=" + revision + "\n",
			wantError: "missing METACUBEX_SING_REVISION",
		},
		{
			name: "unknown source",
			body: "METACUBEX_META_REVISION=" + revision + "\n" +
				"METACUBEX_SING_REVISION=" + revision + "\n" +
				"SHADOWROCKET_REVISION=" + revision + "\n" +
				"UNEXPECTED_REVISION=" + revision + "\n",
			wantError: "unsupported rule-set source UNEXPECTED_REVISION",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			sources := filepath.Join(t.TempDir(), "sources.lock")
			require.NoError(t, os.WriteFile(sources, []byte(test.body), 0o644))
			command := exec.Command(
				"bash",
				filepath.Join(repositoryRoot, "scripts/generate-ruleset-catalog.sh"),
				"generate",
				filepath.Join(t.TempDir(), "output"),
			)
			command.Dir = repositoryRoot
			command.Env = append(os.Environ(), "RULESET_CATALOG_SOURCES_FILE="+sources)
			output, err := command.CombinedOutput()
			require.Error(t, err, string(output))
			require.Contains(t, string(output), test.wantError)
		})
	}
}

func runCatalogScript(t *testing.T, repositoryRoot string, environment []string, arguments ...string) {
	t.Helper()
	command := exec.Command(
		"bash",
		append([]string{filepath.Join(repositoryRoot, "scripts/generate-ruleset-catalog.sh")}, arguments...)...,
	)
	command.Dir = repositoryRoot
	command.Env = append(os.Environ(), environment...)
	output, err := command.CombinedOutput()
	require.NoError(t, err, string(output))
}

func readCatalog(t *testing.T, filename string) catalogSnapshot {
	t.Helper()
	body, err := os.ReadFile(filename)
	require.NoError(t, err)
	reader, err := gzip.NewReader(bytes.NewReader(body))
	require.NoError(t, err)
	decodedBody, err := io.ReadAll(reader)
	require.NoError(t, err)
	require.NoError(t, reader.Close())
	var catalog catalogSnapshot
	require.NoError(t, json.Unmarshal(decodedBody, &catalog))
	return catalog
}

func writeSourcesLock(t *testing.T, filename, meta, sing, shadowrocket string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filename, []byte(sourcesLockBody(meta, sing, shadowrocket)), 0o644))
}

func sourcesLockBody(meta, sing, shadowrocket string) string {
	return "METACUBEX_META_REVISION=" + meta + "\n" +
		"METACUBEX_SING_REVISION=" + sing + "\n" +
		"SHADOWROCKET_REVISION=" + shadowrocket + "\n"
}

func gitRevision(t *testing.T, repository string) string {
	t.Helper()
	command := exec.Command("git", "-C", repository, "rev-parse", "HEAD")
	output, err := command.CombinedOutput()
	require.NoError(t, err, string(output))
	return strings.TrimSpace(string(output))
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
