package file

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
	"github.com/kuuvahki-labs/sandrone/internal/processor"
)

// TemplateParams substitutes variables in the file content. The default
// delimiters are {{ and }}; configure Open / Close to change them.
// Variables come from Vars first, then RequestInfo.Meta keys.
type TemplateParams struct {
	Vars  map[string]string `json:"vars,omitempty" jsonschema:"Explicit substitutions that override request metadata"`
	Open  string            `json:"open,omitempty" jsonschema:"Opening placeholder delimiter" default:"{{"`
	Close string            `json:"close,omitempty" jsonschema:"Closing placeholder delimiter" default:"}}"`
}

type templateProc struct {
	vars  map[string]string
	open  string
	close string
}

func buildTemplate(spec domain.ProcessorSpec) (domain.FileProcessor, error) {
	var params TemplateParams
	if err := processor.UnmarshalParams(spec, &params); err != nil {
		return nil, err
	}
	open := params.Open
	if open == "" {
		open = "{{"
	}
	close := params.Close
	if close == "" {
		close = "}}"
	}
	return &templateProc{vars: params.Vars, open: open, close: close}, nil
}

func (p *templateProc) Name() string { return "template" }

func (p *templateProc) ApplyFile(_ context.Context, in domain.FileProcessInput) (domain.FileProcessOutput, error) {
	merged := make(map[string]string, len(p.vars)+len(in.Request.Meta))
	for k, v := range in.Request.Meta {
		merged[k] = v
	}
	for k, v := range p.vars {
		merged[k] = v
	}
	body, err := substitute(in.File.Content, p.open, p.close, merged)
	if err != nil {
		return domain.FileProcessOutput{File: in.File}, &domain.AppError{
			Code:      domain.CodeFileProcessorFailed,
			Message:   err.Error(),
			Processor: p.Name(),
		}
	}
	doc := in.File
	doc.Content = body
	return domain.FileProcessOutput{File: doc}, nil
}

// substitute scans content and replaces {{key}} occurrences with the value
// from vars. Unknown keys cause an error so silent typos do not slip into
// generated files.
func substitute(content []byte, open, close string, vars map[string]string) ([]byte, error) {
	if len(content) == 0 {
		return content, nil
	}
	src := string(content)
	out := bytes.Buffer{}
	for {
		start := strings.Index(src, open)
		if start < 0 {
			out.WriteString(src)
			break
		}
		out.WriteString(src[:start])
		rest := src[start+len(open):]
		end := strings.Index(rest, close)
		if end < 0 {
			return nil, fmt.Errorf("unterminated template delimiter at offset %d", start)
		}
		key := strings.TrimSpace(rest[:end])
		value, ok := vars[key]
		if !ok {
			return nil, fmt.Errorf("template variable %q undefined", key)
		}
		out.WriteString(value)
		src = rest[end+len(close):]
	}
	return out.Bytes(), nil
}
