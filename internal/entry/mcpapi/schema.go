package mcpapi

import (
	"encoding/json"

	"github.com/google/jsonschema-go/jsonschema"

	"github.com/kuuvahki-labs/sandrone/internal/agentcatalog"
)

func convertInputSchema() *jsonschema.Schema {
	return closedObject(map[string]*jsonschema.Schema{
		"from_format":       parseFormatSchema(),
		"to_format":         renderFormatSchema(),
		"content":           stringSchema(),
		"remote":            remoteInputSchema(),
		"parse_processors":  arraySchema(processorSpecSchema()),
		"render_processors": arraySchema(processorSpecSchema()),
		"options": closedObject(map[string]*jsonschema.Schema{
			"format": renderFormatSchema(),
		}, nil),
		"meta": stringMapSchema(),
	}, []string{"to_format"},
		&jsonschema.Schema{
			Required: []string{"content", "from_format"},
			Not:      &jsonschema.Schema{Required: []string{"remote"}},
		},
		&jsonschema.Schema{
			Required: []string{"remote"},
			Not:      &jsonschema.Schema{Required: []string{"content"}},
		},
	)
}

func getFileInputSchema() *jsonschema.Schema {
	return closedObject(map[string]*jsonschema.Schema{
		"file":   stringSchema(),
		"mode":   defaultedSchema(enumSchema("spec", "source", "render"), "render"),
		"target": stringSchema(),
		"args":   stringMapSchema(),
	}, []string{"file"})
}

func subscriptionPreviewInputSchema() *jsonschema.Schema {
	return closedObject(map[string]*jsonschema.Schema{
		"name": stringSchema(),
		"args": stringMapSchema(),
	}, []string{"name"})
}

func subscriptionRenderInputSchema() *jsonschema.Schema {
	return closedObject(map[string]*jsonschema.Schema{
		"name":   stringSchema(),
		"format": renderFormatSchema(),
		"args":   stringMapSchema(),
	}, []string{"name", "format"})
}

func subscriptionTrafficInputSchema() *jsonschema.Schema {
	return closedObject(map[string]*jsonschema.Schema{
		"name":    stringSchema(),
		"refresh": {Type: "boolean"},
	}, []string{"name"})
}

func validateFileInputSchema() *jsonschema.Schema {
	return closedObject(map[string]*jsonschema.Schema{
		"file":   stringSchema(),
		"spec":   agentcatalog.FileSpecSchema(false),
		"target": stringSchema(),
		"args":   stringMapSchema(),
	}, nil, exactlyOneOf("file", "spec")...)
}

func probeInputSchema() *jsonschema.Schema {
	return closedObject(map[string]*jsonschema.Schema{
		"input":             nodeInputSchema(),
		"method":            enumSchema("tcp_connect", "udp_ntp", "url_test"),
		"core":              enumSchema("mihomo", "sing-box"),
		"url":               stringSchema(),
		"ntp_server":        stringSchema(),
		"expected_status":   stringSchema(),
		"timeout_ms":        boundedIntegerSchema(0, nil),
		"attempts":          boundedIntegerSchema(0, nil),
		"concurrency":       boundedIntegerSchema(0, nil),
		"cache_ttl_seconds": boundedIntegerSchema(0, nil),
		"meta":              stringMapSchema(),
	}, []string{"input"})
}

func putSubscriptionInputSchema() *jsonschema.Schema {
	return agentcatalog.SubscriptionSchema()
}

func deleteSubscriptionInputSchema() *jsonschema.Schema {
	return closedObject(map[string]*jsonschema.Schema{
		"name": stringSchema(),
	}, []string{"name"})
}

func putFileInputSchema() *jsonschema.Schema {
	return agentcatalog.FileSpecSchema(true)
}

func deleteFileInputSchema() *jsonschema.Schema {
	return closedObject(map[string]*jsonschema.Schema{
		"name": stringSchema(),
	}, []string{"name"})
}

func listResourcesInputSchema() *jsonschema.Schema {
	maximum := float64(maxResourceListLimit)
	return closedObject(map[string]*jsonschema.Schema{
		"kind":   enumSchema("subscription", "file"),
		"cursor": stringSchema(),
		"limit":  defaultedSchema(boundedIntegerSchema(1, &maximum), defaultResourceListLimit),
	}, nil)
}

func inspectCapabilitiesInputSchema() *jsonschema.Schema {
	return closedObject(map[string]*jsonschema.Schema{
		"kind": enumSchema("format", "processor", "file_kind"),
		"name": stringSchema(),
	}, nil)
}

func processorSpecSchema() *jsonschema.Schema {
	return closedObject(map[string]*jsonschema.Schema{
		"type":   stringSchema(),
		"stage":  enumSchema("nodes", "file"),
		"name":   stringSchema(),
		"params": openObjectSchema(),
	}, []string{"type"})
}

func remoteInputSchema() *jsonschema.Schema {
	return closedObject(map[string]*jsonschema.Schema{
		"url":               stringSchema(),
		"user_agent":        stringSchema(),
		"proxy":             stringSchema(),
		"timeout_ms":        boundedIntegerSchema(0, nil),
		"cache_ttl_seconds": boundedIntegerSchema(0, nil),
	}, []string{"url"})
}

func nodeInputSchema() *jsonschema.Schema {
	return closedObject(map[string]*jsonschema.Schema{
		"name":              stringSchema(),
		"type":              nodeInputTypeSchema(),
		"ref":               openObjectSchema(),
		"format":            parseFormatSchema(),
		"target":            stringSchema(),
		"nodes":             arraySchema(openObjectSchema()),
		"content":           stringSchema(),
		"path":              stringSchema(),
		"url":               stringSchema(),
		"user_agent":        stringSchema(),
		"proxy":             stringSchema(),
		"timeout_ms":        boundedIntegerSchema(0, nil),
		"cache_ttl_seconds": boundedIntegerSchema(0, nil),
		"required":          &jsonschema.Schema{Type: "boolean"},
		"meta":              stringMapSchema(),
	}, []string{"name", "type"})
}

func closedObject(properties map[string]*jsonschema.Schema, required []string, oneOf ...*jsonschema.Schema) *jsonschema.Schema {
	return &jsonschema.Schema{
		Type:                 "object",
		Properties:           properties,
		Required:             required,
		OneOf:                oneOf,
		AdditionalProperties: falseSchema(),
	}
}

func openObjectSchema() *jsonschema.Schema {
	return &jsonschema.Schema{Type: "object"}
}

func stringSchema() *jsonschema.Schema {
	return &jsonschema.Schema{Type: "string"}
}

func stringMapSchema() *jsonschema.Schema {
	return &jsonschema.Schema{
		Type:                 "object",
		AdditionalProperties: stringSchema(),
	}
}

func arraySchema(items *jsonschema.Schema) *jsonschema.Schema {
	return &jsonschema.Schema{Type: "array", Items: items}
}

func enumSchema(values ...string) *jsonschema.Schema {
	enum := make([]any, len(values))
	for index, value := range values {
		enum[index] = value
	}
	return &jsonschema.Schema{Type: "string", Enum: enum}
}

func parseFormatSchema() *jsonschema.Schema {
	return enumSchema("uri", "uri-list", "base64", "mihomo", "sing-box", "json-nodes")
}

func renderFormatSchema() *jsonschema.Schema {
	return enumSchema("base64", "mihomo-proxies", "shadowrocket-proxies", "sing-box-outbounds", "json-nodes", "uri-list")
}

func nodeInputTypeSchema() *jsonschema.Schema {
	return enumSchema("inline_nodes", "inline", "local", "remote", "ref", "subscription")
}

func boundedIntegerSchema(minimum float64, maximum *float64) *jsonschema.Schema {
	return &jsonschema.Schema{
		Type:    "integer",
		Minimum: &minimum,
		Maximum: maximum,
	}
}

func exactlyOneOf(left, right string) []*jsonschema.Schema {
	return []*jsonschema.Schema{
		{Required: []string{left}, Not: &jsonschema.Schema{Required: []string{right}}},
		{Required: []string{right}, Not: &jsonschema.Schema{Required: []string{left}}},
	}
}

func falseSchema() *jsonschema.Schema {
	return &jsonschema.Schema{Not: &jsonschema.Schema{}}
}

func defaultedSchema(schema *jsonschema.Schema, value any) *jsonschema.Schema {
	body, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	schema.Default = body
	return schema
}
