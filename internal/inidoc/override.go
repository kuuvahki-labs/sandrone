package inidoc

import "strings"

type overrideOperation uint8

const (
	overrideMerge overrideOperation = iota
	overridePrepend
	overrideAppend
	overrideReplace
	overrideDelete
)

// Override applies patch sections to base in source order.
func Override(base, patch []byte) ([]byte, error) {
	doc, err := Parse(base)
	if err != nil {
		return nil, err
	}
	patchDoc, err := Parse(patch)
	if err != nil {
		return nil, err
	}

	for _, patchSection := range patchDoc.sections {
		name, operation, err := parseOverrideSection(patchSection.name, patchSection.header.number)
		if err != nil {
			return nil, err
		}
		patchLines := sectionBodyText(patchSection)
		switch operation {
		case overridePrepend:
			doc.ReplaceSection(name, append(patchLines, doc.SectionLines(name)...))
		case overrideAppend:
			doc.ReplaceSection(name, append(doc.SectionLines(name), patchLines...))
		case overrideReplace:
			doc.ReplaceSection(name, patchLines)
		case overrideDelete:
			for _, current := range patchSection.body {
				if !isIgnorable(current.text) {
					return nil, &Error{
						Section: name,
						Line:    current.number,
						Message: "delete section must not contain effective content",
					}
				}
			}
			doc.deleteSections(name)
		default:
			doc.ReplaceSection(name, mergeSectionLines(doc.SectionLines(name), patchLines))
		}
	}
	return doc.Bytes(), nil
}

func parseOverrideSection(raw string, lineNumber int) (string, overrideOperation, error) {
	name := strings.TrimSpace(raw)
	if strings.HasPrefix(name, "<") && strings.HasSuffix(name, ">") {
		literal := strings.TrimSpace(name[1 : len(name)-1])
		if literal == "" {
			return "", 0, &Error{Line: lineNumber, Message: "empty escaped section name"}
		}
		return literal, overrideMerge, nil
	}

	prefix := strings.HasPrefix(name, "+")
	suffix := overrideMerge
	switch {
	case strings.HasSuffix(name, "+"):
		suffix = overrideAppend
	case strings.HasSuffix(name, "!"):
		suffix = overrideReplace
	case strings.HasSuffix(name, "-"):
		suffix = overrideDelete
	}
	if prefix && suffix != overrideMerge {
		return "", 0, &Error{Line: lineNumber, Message: "ambiguous section override operator"}
	}
	operation := suffix
	if prefix {
		operation = overridePrepend
		name = strings.TrimSpace(strings.TrimPrefix(name, "+"))
	} else if suffix != overrideMerge {
		name = strings.TrimSpace(name[:len(name)-1])
	}
	if name == "" {
		return "", 0, &Error{Line: lineNumber, Message: "empty section override target"}
	}
	return name, operation, nil
}

func sectionBodyText(candidate section) []string {
	out := make([]string, len(candidate.body))
	for i, current := range candidate.body {
		out[i] = current.text
	}
	return out
}

func mergeSectionLines(base, patch []string) []string {
	out := append([]string(nil), base...)
	for _, patchLine := range patch {
		if key, ok := assignmentKey(patchLine); ok {
			first := -1
			filtered := make([]string, 0, len(out))
			for _, current := range out {
				currentKey, assignment := assignmentKey(current)
				if assignment && currentKey == key {
					if first < 0 {
						first = len(filtered)
						filtered = append(filtered, patchLine)
					}
					continue
				}
				filtered = append(filtered, current)
			}
			if first < 0 {
				filtered = append(filtered, patchLine)
			}
			out = filtered
			continue
		}

		trimmed := strings.TrimSpace(patchLine)
		duplicate := false
		for _, current := range out {
			if strings.TrimSpace(current) == trimmed {
				duplicate = true
				break
			}
		}
		if !duplicate {
			out = append(out, patchLine)
		}
	}
	return out
}

func assignmentKey(value string) (string, bool) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, ";") {
		return "", false
	}
	separator := strings.IndexByte(value, '=')
	if separator < 0 {
		return "", false
	}
	key := strings.TrimSpace(value[:separator])
	return key, key != "" && !strings.ContainsRune(key, ',')
}

func isIgnorable(value string) bool {
	trimmed := strings.TrimSpace(value)
	return trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, ";")
}
