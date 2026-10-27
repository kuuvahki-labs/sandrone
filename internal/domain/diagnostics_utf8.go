package domain

import (
	"encoding/json/v2"
	"strings"
)

// WarningsWithValidUTF8 copies diagnostic text into a form that can be reported
// even when the source node contains invalid text. It never changes node data.
func WarningsWithValidUTF8(warnings []Warning) []Warning {
	if warnings == nil {
		return nil
	}
	out := append([]Warning{}, warnings...)
	text := func(value string) string { return strings.ToValidUTF8(value, "\uFFFD") }
	for i := range out {
		warning := &out[i]
		warning.Code = text(warning.Code)
		warning.Message = text(warning.Message)
		warning.Node = text(warning.Node)
		warning.Field = text(warning.Field)
		warning.Source = text(warning.Source)
		warning.Target = text(warning.Target)
		if warning.NodeContext == nil {
			continue
		}
		context := *warning.NodeContext
		context.Format = text(context.Format)
		context.Name = text(context.Name)
		context.Type = NodeType(text(string(context.Type)))
		context.Server = text(context.Server)
		context.RawLine = text(context.RawLine)
		// Opaque diagnostic snapshots are optional. Keep valid ones unchanged;
		// omit unencodable ones rather than altering connection data.
		if _, err := json.Marshal(context.Raw); err != nil {
			context.Raw = nil
		}
		warning.NodeContext = &context
	}
	return out
}
