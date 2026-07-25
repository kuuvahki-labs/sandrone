package inidoc

import (
	"bytes"
	"strings"
)

// Model is the JSON-shaped, ordered representation used by consumers that
// need to inspect or construct an INI document without collapsing duplicate
// sections or non-assignment lines.
type Model struct {
	BOM             bool           `json:"bom" jsonschema:"Whether the document starts with a UTF-8 BOM"`
	Newline         string         `json:"newline" jsonschema:"Document newline; must be LF or CRLF"`
	TrailingNewline bool           `json:"trailing_newline" jsonschema:"Whether the rendered document ends with its configured newline"`
	Preamble        []string       `json:"preamble" jsonschema:"Ordered lines before the first section"`
	Sections        []ModelSection `json:"sections" jsonschema:"Ordered physical sections; duplicate names are preserved"`
}

// ModelSection preserves a physical section and its ordered body lines.
type ModelSection struct {
	Name  string   `json:"name" jsonschema:"Canonical non-empty section name"`
	Lines []string `json:"lines" jsonschema:"Ordered raw body lines without newline characters"`
}

// ParseModel parses body into an ordered model. Per-line newline differences
// are intentionally normalized to the document's first observed newline.
func ParseModel(body []byte) (Model, error) {
	doc, err := Parse(body)
	if err != nil {
		return Model{}, err
	}
	model := Model{
		BOM:             doc.bom,
		Newline:         doc.newline,
		TrailingNewline: doc.endsWithNewline(),
		Preamble:        []string{},
		Sections:        []ModelSection{},
	}
	if len(doc.preamble) > 0 {
		model.Preamble = make([]string, len(doc.preamble))
		for index, current := range doc.preamble {
			model.Preamble[index] = current.text
		}
	}
	if len(doc.sections) > 0 {
		model.Sections = make([]ModelSection, len(doc.sections))
		for index, candidate := range doc.sections {
			model.Sections[index] = ModelSection{
				Name:  candidate.name,
				Lines: sectionBodyText(candidate),
			}
		}
	}
	return model, nil
}

// RenderModel validates and renders an ordered model using canonical section
// headers and a single document newline.
func RenderModel(model Model) ([]byte, error) {
	if model.Newline != "\n" && model.Newline != "\r\n" {
		return nil, &Error{Message: `model newline must be "\n" or "\r\n"`}
	}
	if model.Preamble == nil || model.Sections == nil {
		return nil, &Error{Message: "model preamble and sections must be arrays"}
	}

	lines := make([]string, 0, len(model.Preamble)+len(model.Sections))
	for index, value := range model.Preamble {
		if err := validateModelLine(value, "", index+1); err != nil {
			return nil, err
		}
		lines = append(lines, value)
	}
	for _, candidate := range model.Sections {
		name := strings.TrimSpace(candidate.Name)
		if name == "" || name != candidate.Name || strings.ContainsAny(name, "[]\r\n") {
			return nil, &Error{Section: candidate.Name, Message: "invalid model section name"}
		}
		if candidate.Lines == nil {
			return nil, &Error{Section: name, Message: "model section lines must be an array"}
		}
		lines = append(lines, "["+name+"]")
		for index, value := range candidate.Lines {
			if err := validateModelLine(value, name, index+1); err != nil {
				return nil, err
			}
			lines = append(lines, value)
		}
	}
	if model.TrailingNewline && len(lines) == 0 {
		return nil, &Error{Message: "model trailing_newline requires document content"}
	}

	var out bytes.Buffer
	if model.BOM {
		out.Write(utf8BOM)
	}
	for index, value := range lines {
		out.WriteString(value)
		if index+1 < len(lines) || model.TrailingNewline {
			out.WriteString(model.Newline)
		}
	}
	return out.Bytes(), nil
}

func validateModelLine(value, section string, lineNumber int) error {
	if strings.ContainsAny(value, "\r\n") {
		return &Error{Section: section, Line: lineNumber, Message: "model line must not contain a newline"}
	}
	_, header, err := parseHeader(line{text: value, number: lineNumber})
	if err != nil {
		return &Error{Section: section, Line: lineNumber, Message: "model line resembles an invalid section header"}
	}
	if header {
		return &Error{Section: section, Line: lineNumber, Message: "model line must not contain a section header"}
	}
	return nil
}
