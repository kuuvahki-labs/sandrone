package service

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

type singBoxFileDriver struct{}

func (singBoxFileDriver) Descriptor() typedFileDescriptor {
	return typedFileDescriptor{
		Kind:             domain.FileKindSingBox,
		MediaType:        "application/json",
		Syntax:           "json",
		DefaultExtension: ".json",
		NodeRenderFormat: "sing-box-outbounds",
		DefaultBase: []byte(`{
  "log": { "level": "info" },
  "inbounds": [],
  "outbounds": [],
  "route": {
    "rule_set": [],
    "rules": []
  }
}`),
	}
}

func (singBoxFileDriver) ValidateSettings(raw json.RawMessage) error {
	_, err := decodeSingBoxFileSettings(raw)
	return err
}

func (singBoxFileDriver) Compile(_ context.Context, in typedFileCompileInput) ([]byte, error) {
	settings, err := decodeSingBoxFileSettings(in.Settings)
	if err != nil {
		return nil, err
	}
	var doc map[string]any
	if err := json.Unmarshal(in.Base, &doc); err != nil {
		return nil, domain.WrapError(domain.CodeInvalidArgument, `file kind "sing-box" base: parse JSON`, err)
	}
	if doc == nil {
		doc = map[string]any{}
	}
	var renderedDoc map[string]any
	if err := json.Unmarshal(in.RenderedNodes, &renderedDoc); err != nil {
		return nil, domain.WrapError(domain.CodeInvalidArgument, `file kind "sing-box": parse rendered nodes`, err)
	}
	nodeOutbounds := anyList(renderedDoc["outbounds"])
	nodeEndpoints := anyList(renderedDoc["endpoints"])
	names := namesFromSingBoxOutbounds(nodeOutbounds, nodeEndpoints)
	if settings.Groups == nil {
		doc["outbounds"] = singBoxOutbounds("basic", names, nodeOutbounds)
	} else {
		doc["outbounds"] = singBoxOutboundsWithGroups(settings.Groups, names, nodeOutbounds)
	}
	if len(nodeEndpoints) > 0 {
		doc["endpoints"] = nodeEndpoints
	}
	route := mapValue(doc["route"])
	if settings.RuleSets == nil {
		route["rule_set"] = singBoxRuleSets("default")
	} else {
		route["rule_set"] = configMapList(settings.RuleSets)
	}
	if settings.Rules == nil {
		route["rules"] = singBoxRules()
	} else {
		route["rules"] = configMapList(settings.Rules)
	}
	if final, ok := route["final"].(string); !ok || strings.TrimSpace(final) == "" {
		route["final"] = "Proxy"
	}
	doc["route"] = route
	out, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, domain.WrapError(domain.CodeInvalidArgument, `file kind "sing-box": encode config`, err)
	}
	return out, nil
}
