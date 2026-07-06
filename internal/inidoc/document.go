// Package inidoc provides a lossless, format-neutral INI document model.
package inidoc

import (
	"bytes"
	"fmt"
	"strings"
)

var utf8BOM = []byte{0xef, 0xbb, 0xbf}

// Error describes invalid INI input. Section is populated when the error can
// be attributed to a logical section.
type Error struct {
	Section string
	Line    int
	Message string
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	location := ""
	if e.Line > 0 {
		location = fmt.Sprintf(" at line %d", e.Line)
	}
	if e.Section != "" {
		return fmt.Sprintf("ini section %q%s: %s", e.Section, location, e.Message)
	}
	return fmt.Sprintf("ini%s: %s", location, e.Message)
}

type line struct {
	text   string
	eol    string
	number int
}

type section struct {
	name   string
	header line
	body   []line
}

// Document stores an INI file as physical lines so untouched text can be
// serialized byte-for-byte.
type Document struct {
	bom      bool
	newline  string
	preamble []line
	sections []section
}

// Parse reads an INI document without normalizing its contents.
func Parse(body []byte) (*Document, error) {
	doc := &Document{newline: "\n"}
	if bytes.HasPrefix(body, utf8BOM) {
		doc.bom = true
		body = body[len(utf8BOM):]
	}
	lines, newline := splitLines(body)
	if newline != "" {
		doc.newline = newline
	}

	for _, current := range lines {
		name, isHeader, err := parseHeader(current)
		if err != nil {
			return nil, err
		}
		if isHeader {
			doc.sections = append(doc.sections, section{name: name, header: current})
			continue
		}
		if len(doc.sections) == 0 {
			doc.preamble = append(doc.preamble, current)
			continue
		}
		last := len(doc.sections) - 1
		doc.sections[last].body = append(doc.sections[last].body, current)
	}
	return doc, nil
}

func splitLines(body []byte) ([]line, string) {
	lines := make([]line, 0, bytes.Count(body, []byte{'\n'})+1)
	newline := ""
	start := 0
	lineNumber := 1
	for i, value := range body {
		if value != '\n' {
			continue
		}
		end := i
		eol := "\n"
		if end > start && body[end-1] == '\r' {
			end--
			eol = "\r\n"
		}
		if newline == "" {
			newline = eol
		}
		lines = append(lines, line{text: string(body[start:end]), eol: eol, number: lineNumber})
		start = i + 1
		lineNumber++
	}
	if start < len(body) {
		lines = append(lines, line{text: string(body[start:]), number: lineNumber})
	}
	return lines, newline
}

func parseHeader(current line) (string, bool, error) {
	trimmed := strings.TrimSpace(current.text)
	if !strings.HasPrefix(trimmed, "[") {
		return "", false, nil
	}
	if !strings.HasSuffix(trimmed, "]") || len(trimmed) < 2 {
		return "", false, &Error{Line: current.number, Message: "invalid section header"}
	}
	name := strings.TrimSpace(trimmed[1 : len(trimmed)-1])
	if name == "" {
		return "", false, &Error{Line: current.number, Message: "empty section header"}
	}
	if strings.ContainsAny(name, "[]\r\n") {
		return "", false, &Error{Line: current.number, Message: "invalid section header"}
	}
	return name, true, nil
}

// SectionLines returns the ordered logical body of every matching physical
// section. Matching trims the requested name and ignores case.
func (d *Document) SectionLines(name string) []string {
	if d == nil {
		return nil
	}
	target := strings.TrimSpace(name)
	var out []string
	for _, candidate := range d.sections {
		if !sectionNameEqual(candidate.name, target) {
			continue
		}
		if out == nil {
			out = []string{}
		}
		for _, current := range candidate.body {
			out = append(out, current.text)
		}
	}
	return out
}

// ReplaceSection replaces all matching physical sections with one section at
// the first occurrence. A missing section is appended at EOF.
func (d *Document) ReplaceSection(name string, lines []string) {
	if d == nil {
		return
	}
	target := strings.TrimSpace(name)
	if target == "" {
		return
	}

	first := -1
	filtered := make([]section, 0, len(d.sections))
	for _, candidate := range d.sections {
		if sectionNameEqual(candidate.name, target) {
			if first < 0 {
				first = len(filtered)
				filtered = append(filtered, candidate)
			}
			continue
		}
		filtered = append(filtered, candidate)
	}
	if first < 0 {
		d.appendSection(target, lines)
		return
	}

	finalNewline := d.endsWithNewline()
	hasFollowing := first+1 < len(filtered)
	replacement := &filtered[first]
	switch {
	case len(lines) > 0 || hasFollowing:
		if replacement.header.eol == "" {
			replacement.header.eol = d.newline
		}
	case finalNewline:
		replacement.header.eol = d.newline
	default:
		replacement.header.eol = ""
	}
	replacement.body = d.makeLines(lines, hasFollowing || finalNewline)
	d.sections = filtered
}

func (d *Document) appendSection(name string, body []string) {
	finalNewline := d.endsWithNewline()
	d.ensureTrailingNewline()
	header := line{text: "[" + name + "]"}
	if len(body) > 0 || finalNewline {
		header.eol = d.newline
	}
	d.sections = append(d.sections, section{
		name:   name,
		header: header,
		body:   d.makeLines(body, finalNewline),
	})
}

func (d *Document) makeLines(text []string, terminalNewline bool) []line {
	if len(text) == 0 {
		return nil
	}
	out := make([]line, len(text))
	for i, value := range text {
		out[i] = line{text: value, eol: d.newline}
	}
	if !terminalNewline {
		out[len(out)-1].eol = ""
	}
	return out
}

func (d *Document) deleteSections(name string) {
	target := strings.TrimSpace(name)
	out := d.sections[:0]
	for _, candidate := range d.sections {
		if !sectionNameEqual(candidate.name, target) {
			out = append(out, candidate)
		}
	}
	d.sections = out
}

func (d *Document) endsWithNewline() bool {
	if len(d.sections) > 0 {
		last := d.sections[len(d.sections)-1]
		if len(last.body) > 0 {
			return last.body[len(last.body)-1].eol != ""
		}
		return last.header.eol != ""
	}
	if len(d.preamble) > 0 {
		return d.preamble[len(d.preamble)-1].eol != ""
	}
	return false
}

func (d *Document) ensureTrailingNewline() {
	if len(d.sections) > 0 {
		last := &d.sections[len(d.sections)-1]
		if len(last.body) > 0 {
			if last.body[len(last.body)-1].eol == "" {
				last.body[len(last.body)-1].eol = d.newline
			}
			return
		}
		if last.header.eol == "" {
			last.header.eol = d.newline
		}
		return
	}
	if len(d.preamble) > 0 && d.preamble[len(d.preamble)-1].eol == "" {
		d.preamble[len(d.preamble)-1].eol = d.newline
	}
}

func sectionNameEqual(left, right string) bool {
	return strings.EqualFold(strings.TrimSpace(left), strings.TrimSpace(right))
}

// Bytes serializes the current document while retaining all untouched lines.
func (d *Document) Bytes() []byte {
	if d == nil {
		return nil
	}
	var out bytes.Buffer
	if d.bom {
		out.Write(utf8BOM)
	}
	writeLines := func(lines []line) {
		for _, current := range lines {
			out.WriteString(current.text)
			out.WriteString(current.eol)
		}
	}
	writeLines(d.preamble)
	for _, candidate := range d.sections {
		writeLines([]line{candidate.header})
		writeLines(candidate.body)
	}
	return out.Bytes()
}
