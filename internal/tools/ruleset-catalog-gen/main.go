package main

import (
	"compress/gzip"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	metaCubeXMetaRawBase   = "https://raw.githubusercontent.com/MetaCubeX/meta-rules-dat/meta/"
	metaCubeXSingRawBase   = "https://raw.githubusercontent.com/MetaCubeX/meta-rules-dat/sing/"
	shadowrocketRawBase    = "https://raw.githubusercontent.com/blackmatrix7/ios_rule_script/master/"
	shadowrocketRulesPath  = "rule/Shadowrocket"
	shadowrocketRuleSet    = "RULE-SET"
	shadowrocketDomainSet  = "DOMAIN-SET"
	shadowrocketUsageTitle = "#### 使用说明"
)

type catalogItem struct {
	Name          string `json:"name"`
	URL           string `json:"url"`
	RuleKind      string `json:"rule_kind"`
	ReferenceType string `json:"reference_type,omitempty"`
}

type catalogSnapshot struct {
	Mihomo       []catalogItem `json:"mihomo"`
	SingBox      []catalogItem `json:"sing-box"`
	Shadowrocket []catalogItem `json:"shadowrocket"`
}

type catalogInputs struct {
	MetaCubeXMeta    []string
	MetaCubeXSing    []string
	ShadowrocketRoot string
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	flags := flag.NewFlagSet("ruleset-catalog-gen", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	var output, metaPaths, singPaths, shadowrocketRoot string
	flags.StringVar(&output, "output", "", "output catalog.json.gz path")
	flags.StringVar(&metaPaths, "metacubex-meta-paths", "", "MetaCubeX meta branch git ls-tree path list")
	flags.StringVar(&singPaths, "metacubex-sing-paths", "", "MetaCubeX sing branch git ls-tree path list")
	flags.StringVar(&shadowrocketRoot, "shadowrocket-root", "", "checked-out blackmatrix7/ios_rule_script repository root")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected positional arguments: %s", strings.Join(flags.Args(), " "))
	}
	for _, required := range []struct{ name, value string }{
		{"output", output}, {"metacubex-meta-paths", metaPaths},
		{"metacubex-sing-paths", singPaths},
		{"shadowrocket-root", shadowrocketRoot},
	} {
		if strings.TrimSpace(required.value) == "" {
			return fmt.Errorf("-%s is required", required.name)
		}
	}

	meta, err := readTreePaths(metaPaths)
	if err != nil {
		return fmt.Errorf("read MetaCubeX tree paths: %w", err)
	}
	sing, err := readTreePaths(singPaths)
	if err != nil {
		return fmt.Errorf("read MetaCubeX sing tree paths: %w", err)
	}
	catalog, err := generateCatalog(catalogInputs{
		MetaCubeXMeta:    meta,
		MetaCubeXSing:    sing,
		ShadowrocketRoot: shadowrocketRoot,
	})
	if err != nil {
		return err
	}
	return writeOutput(output, catalog)
}

func readTreePaths(name string) ([]string, error) {
	body, err := os.ReadFile(name)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(string(body), "\n")
	paths := make([]string, 0, len(lines))
	for _, line := range lines {
		if line = strings.TrimSuffix(line, "\r"); line != "" {
			paths = append(paths, line)
		}
	}
	return paths, nil
}

func generateCatalog(inputs catalogInputs) (catalogSnapshot, error) {
	mihomo := make([]catalogItem, 0)
	for _, artifactPath := range inputs.MetaCubeXMeta {
		if item, ok := metaCubeXItem(artifactPath); ok {
			mihomo = append(mihomo, item)
		}
	}
	singBox := make([]catalogItem, 0)
	for _, artifactPath := range inputs.MetaCubeXSing {
		if item, ok := metaCubeXSingItem(artifactPath); ok {
			singBox = append(singBox, item)
		}
	}
	shadowrocket := make([]catalogItem, 0)
	if strings.TrimSpace(inputs.ShadowrocketRoot) != "" {
		var err error
		shadowrocket, err = shadowrocketItems(inputs.ShadowrocketRoot)
		if err != nil {
			return catalogSnapshot{}, err
		}
	}

	var err error
	if mihomo, err = sortAndDeduplicate("mihomo", mihomo); err != nil {
		return catalogSnapshot{}, err
	}
	if singBox, err = sortAndDeduplicate("sing-box", singBox); err != nil {
		return catalogSnapshot{}, err
	}
	if shadowrocket, err = sortAndDeduplicate("shadowrocket", shadowrocket); err != nil {
		return catalogSnapshot{}, err
	}
	if len(mihomo) == 0 {
		return catalogSnapshot{}, errors.New("mihomo catalog has no items")
	}
	if len(singBox) == 0 {
		return catalogSnapshot{}, errors.New("sing-box catalog has no items")
	}
	if len(shadowrocket) == 0 {
		return catalogSnapshot{}, errors.New("shadowrocket catalog has no items")
	}
	return catalogSnapshot{Mihomo: mihomo, SingBox: singBox, Shadowrocket: shadowrocket}, nil
}

type shadowrocketReference struct {
	Filename      string
	ReferenceType string
}

func shadowrocketItems(repositoryRoot string) ([]catalogItem, error) {
	rulesRoot := filepath.Join(repositoryRoot, filepath.FromSlash(shadowrocketRulesPath))
	resolvedRulesRoot, err := filepath.EvalSymlinks(rulesRoot)
	if err != nil {
		return nil, fmt.Errorf("resolve shadowrocket rule subtree: %w", err)
	}
	items := make([]catalogItem, 0)
	err = filepath.WalkDir(rulesRoot, func(filename string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || entry.Name() != "README.md" {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("shadowrocket README %s must be a regular file", filename)
		}
		references, err := shadowrocketREADMEReferences(filename)
		if err != nil {
			return err
		}
		for _, reference := range references {
			item, err := shadowrocketItem(rulesRoot, resolvedRulesRoot, filepath.Dir(filename), reference)
			if err != nil {
				return err
			}
			items = append(items, item)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("build shadowrocket catalog: %w", err)
	}
	return sortAndDeduplicate("shadowrocket", items)
}

func shadowrocketREADMEReferences(filename string) ([]shadowrocketReference, error) {
	body, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	inUsage := false
	references := make([]shadowrocketReference, 0)
	for lineNumber, line := range strings.Split(string(body), "\n") {
		line = strings.TrimSpace(strings.TrimPrefix(line, "\ufeff"))
		if strings.HasPrefix(line, "#") {
			inUsage = line == shadowrocketUsageTitle
			continue
		}
		if !inUsage || !strings.Contains(line, ".list") {
			continue
		}
		reference, ok := parseShadowrocketUsageLine(line)
		if !ok {
			return nil, fmt.Errorf("%s:%d: unsupported Shadowrocket usage entry %q", filename, lineNumber+1, line)
		}
		references = append(references, reference)
	}
	return references, nil
}

func parseShadowrocketUsageLine(line string) (shadowrocketReference, bool) {
	const marker = "，请使用"
	line = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "-"))
	separator := strings.Index(line, marker)
	if separator < 1 {
		return shadowrocketReference{}, false
	}
	filename := strings.TrimSpace(line[:separator])
	referenceType := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(line[separator+len(marker):]), "。"))
	if path.Ext(filename) != ".list" || (referenceType != shadowrocketRuleSet && referenceType != shadowrocketDomainSet) {
		return shadowrocketReference{}, false
	}
	return shadowrocketReference{Filename: filename, ReferenceType: referenceType}, true
}

func shadowrocketItem(rulesRoot, resolvedRulesRoot, readmeDirectory string, reference shadowrocketReference) (catalogItem, error) {
	relativeFilename := filepath.Clean(filepath.FromSlash(reference.Filename))
	if filepath.IsAbs(relativeFilename) || relativeFilename == ".." || strings.HasPrefix(relativeFilename, ".."+string(filepath.Separator)) {
		return catalogItem{}, fmt.Errorf("shadowrocket README reference escapes its category: %q", reference.Filename)
	}
	artifactFilename := filepath.Join(readmeDirectory, relativeFilename)
	artifactPath, err := filepath.Rel(rulesRoot, artifactFilename)
	if err != nil || artifactPath == ".." || strings.HasPrefix(artifactPath, ".."+string(filepath.Separator)) {
		return catalogItem{}, fmt.Errorf("shadowrocket README reference escapes rule subtree: %q", reference.Filename)
	}
	artifactPath = filepath.ToSlash(artifactPath)
	if strings.HasSuffix(reference.Filename, "_Domain.list") && reference.ReferenceType != shadowrocketDomainSet {
		return catalogItem{}, fmt.Errorf("shadowrocket artifact %q must use %s", reference.Filename, shadowrocketDomainSet)
	}
	artifactInfo, err := os.Lstat(artifactFilename)
	if err != nil {
		return catalogItem{}, fmt.Errorf("inspect Shadowrocket artifact %s: %w", artifactPath, err)
	}
	if !artifactInfo.Mode().IsRegular() {
		return catalogItem{}, fmt.Errorf("shadowrocket artifact %s must be a regular file", artifactPath)
	}
	resolvedArtifactFilename, err := filepath.EvalSymlinks(artifactFilename)
	if err != nil {
		return catalogItem{}, fmt.Errorf("resolve Shadowrocket artifact %s: %w", artifactPath, err)
	}
	resolvedArtifactPath, err := filepath.Rel(resolvedRulesRoot, resolvedArtifactFilename)
	if err != nil || resolvedArtifactPath == ".." || strings.HasPrefix(resolvedArtifactPath, ".."+string(filepath.Separator)) {
		return catalogItem{}, fmt.Errorf("shadowrocket artifact %q escapes rule subtree", reference.Filename)
	}
	body, err := os.ReadFile(artifactFilename)
	if err != nil {
		return catalogItem{}, fmt.Errorf("read Shadowrocket artifact %s: %w", artifactPath, err)
	}
	ruleKind, err := classifyShadowrocketArtifact(artifactPath, reference.ReferenceType, string(body))
	if err != nil {
		return catalogItem{}, err
	}
	urlPath := path.Join(shadowrocketRulesPath, artifactPath)
	return catalogItem{
		Name:          strings.TrimSuffix(artifactPath, path.Ext(artifactPath)),
		URL:           shadowrocketRawBase + escapePath(urlPath),
		RuleKind:      ruleKind,
		ReferenceType: reference.ReferenceType,
	}, nil
}

func classifyShadowrocketArtifact(artifactPath, referenceType, body string) (string, error) {
	hasRules := false
	ipOnly := true
	for lineNumber, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(strings.TrimPrefix(line, "\ufeff"))
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		hasRules = true
		ruleType, typed := shadowrocketRuleType(line)
		switch referenceType {
		case shadowrocketDomainSet:
			if typed || strings.Contains(line, ",") {
				return "", fmt.Errorf("shadowrocket %s:%d is typed but README declares %s", artifactPath, lineNumber+1, shadowrocketDomainSet)
			}
		case shadowrocketRuleSet:
			if !typed {
				return "", fmt.Errorf("shadowrocket %s:%d is untyped but README declares %s", artifactPath, lineNumber+1, shadowrocketRuleSet)
			}
			if !shadowrocketIPRuleType(ruleType) {
				ipOnly = false
			}
		default:
			return "", fmt.Errorf("shadowrocket %s has invalid reference type %q", artifactPath, referenceType)
		}
	}
	if !hasRules {
		return "", fmt.Errorf("shadowrocket artifact %s has no meaningful rules", artifactPath)
	}
	if referenceType == shadowrocketDomainSet {
		return "domain", nil
	}
	if ipOnly {
		return "ip", nil
	}
	return "mixed", nil
}

func shadowrocketRuleType(line string) (string, bool) {
	separator := strings.IndexByte(line, ',')
	if separator < 1 {
		return "", false
	}
	ruleType := strings.TrimSpace(line[:separator])
	for _, character := range ruleType {
		if (character < 'A' || character > 'Z') && (character < '0' || character > '9') && character != '-' {
			return "", false
		}
	}
	return ruleType, ruleType != ""
}

func shadowrocketIPRuleType(ruleType string) bool {
	switch ruleType {
	case "GEOIP", "IP-ASN", "IP-CIDR", "IP-CIDR6", "SRC-IP":
		return true
	default:
		return false
	}
}

func metaCubeXItem(artifactPath string) (catalogItem, bool) {
	prefix, ruleKind := "", ""
	switch path.Dir(artifactPath) {
	case "geo/geosite":
		prefix, ruleKind = "geosite", "domain"
	case "geo/geoip":
		prefix, ruleKind = "geoip", "ip"
	default:
		return catalogItem{}, false
	}
	if path.Ext(artifactPath) != ".mrs" {
		return catalogItem{}, false
	}
	return newCatalogItem(prefix, ruleKind, artifactPath, metaCubeXMetaRawBase), true
}

func metaCubeXSingItem(artifactPath string) (catalogItem, bool) {
	prefix, ruleKind := "", ""
	switch path.Dir(artifactPath) {
	case "geo/geosite":
		prefix, ruleKind = "geosite", "domain"
	case "geo/geoip":
		prefix, ruleKind = "geoip", "ip"
	default:
		return catalogItem{}, false
	}
	if path.Ext(artifactPath) != ".srs" {
		return catalogItem{}, false
	}
	return newCatalogItem(prefix, ruleKind, artifactPath, metaCubeXSingRawBase), true
}

func newCatalogItem(prefix, ruleKind, artifactPath, rawBase string) catalogItem {
	stem := strings.TrimSuffix(path.Base(artifactPath), path.Ext(artifactPath))
	return catalogItem{
		Name:     prefix + "-" + stem,
		URL:      rawBase + escapePath(artifactPath),
		RuleKind: ruleKind,
	}
}

func escapePath(value string) string {
	segments := strings.Split(value, "/")
	for index := range segments {
		segments[index] = url.PathEscape(segments[index])
	}
	return strings.Join(segments, "/")
}

func sortAndDeduplicate(target string, items []catalogItem) ([]catalogItem, error) {
	sort.Slice(items, func(i, j int) bool {
		if items[i].Name != items[j].Name {
			return items[i].Name < items[j].Name
		}
		return items[i].URL < items[j].URL
	})
	result := make([]catalogItem, 0, len(items))
	nameURLs := make(map[string]string, len(items))
	seenURLs := make(map[string]bool, len(items))
	for _, item := range items {
		if existing, ok := nameURLs[item.Name]; ok && existing != item.URL {
			return nil, fmt.Errorf("%s catalog name %q maps to multiple URLs", target, item.Name)
		}
		nameURLs[item.Name] = item.URL
		if !seenURLs[item.URL] {
			seenURLs[item.URL] = true
			result = append(result, item)
		}
	}
	return result, nil
}

func writeOutput(name string, catalog catalogSnapshot) (returnErr error) {
	temporary, err := os.CreateTemp(filepath.Dir(name), ".catalog.json.gz-*")
	if err != nil {
		return fmt.Errorf("create rule-set catalog output: %w", err)
	}
	temporaryName := temporary.Name()
	defer func() {
		_ = temporary.Close()
		if returnErr != nil {
			_ = os.Remove(temporaryName)
		}
	}()
	if err := writeGzip(temporary, catalog); err != nil {
		return err
	}
	if err := temporary.Chmod(0o644); err != nil {
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporaryName, name)
}

func writeGzip(output io.Writer, catalog catalogSnapshot) error {
	body, err := json.Marshal(catalog)
	if err != nil {
		return err
	}
	compressed, err := gzip.NewWriterLevel(output, gzip.BestCompression)
	if err != nil {
		return err
	}
	compressed.ModTime = time.Time{}
	compressed.OS = 255
	if _, err := compressed.Write(body); err != nil {
		_ = compressed.Close()
		return err
	}
	return compressed.Close()
}
