package agentcatalog

import "github.com/google/jsonschema-go/jsonschema"

// SubscriptionSchema returns the closed wire schema for a stored subscription.
func SubscriptionSchema() *jsonschema.Schema {
	return closedObject(map[string]*jsonschema.Schema{
		"name":         stringSchema(),
		"display_name": stringSchema(),
		"type":         enumSchema("remote", "local", "collection"),
		"format":       subscriptionFormatSchema(),
		"content":      stringSchema(),
		"remote":       remoteInputSchema(),
		"inputs":       arraySchema(nodeInputSchema()),
		"processors":   arraySchema(processorSpecSchema()),
		"nodes":        arraySchema(openObjectSchema()),
		"meta":         stringMapSchema(),
	}, []string{"name", "type"})
}

// FileSpecSchema returns the closed wire schema for a stored file.
func FileSpecSchema(requireName bool) *jsonschema.Schema {
	required := []string{"kind", "source"}
	if requireName {
		required = []string{"name", "kind", "source"}
	}
	return closedObject(map[string]*jsonschema.Schema{
		"name":         stringSchema(),
		"display_name": stringSchema(),
		"kind":         enumSchema("static", "mihomo", "sing-box", "shadowrocket"),
		"source":       fileSourceSchema(),
		"config":       fileConfigSchema(),
		"processors":   arraySchema(processorSpecSchema()),
		"meta":         stringMapSchema(),
	}, required)
}

func processorSpecSchema() *jsonschema.Schema {
	return closedObject(map[string]*jsonschema.Schema{
		"type":   stringSchema(),
		"stage":  enumSchema("nodes", "file"),
		"name":   stringSchema(),
		"params": openObjectSchema(),
	}, []string{"type"})
}

func fileConfigSchema() *jsonschema.Schema {
	return closedObject(map[string]*jsonschema.Schema{
		"subscriptions": arraySchema(stringSchema()),
		"settings":      openObjectSchema(),
	}, nil)
}

func fileSourceSchema() *jsonschema.Schema {
	return closedObject(map[string]*jsonschema.Schema{
		"type":    enumSchema("inline", "remote"),
		"content": stringSchema(),
		"remote":  remoteInputSchema(),
	}, nil)
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
		"required":          {Type: "boolean"},
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

func subscriptionFormatSchema() *jsonschema.Schema {
	return enumSchema("auto", "uri", "uri-list", "base64", "mihomo", "sing-box", "json-nodes")
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

func falseSchema() *jsonschema.Schema {
	return &jsonschema.Schema{Not: &jsonschema.Schema{}}
}
