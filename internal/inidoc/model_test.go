package inidoc_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/inidoc"
)

func TestParseModelPreservesOrderedINISemantics(t *testing.T) {
	body := append([]byte{0xef, 0xbb, 0xbf}, []byte(
		"# generated\r\n"+
			"\r\n"+
			"[General]\r\n"+
			"dns-server = 1.1.1.1\r\n"+
			"[Rule]\r\n"+
			"DOMAIN,example.com,Proxy\r\n"+
			"[Rule]\r\n",
	)...)

	model, err := inidoc.ParseModel(body)

	require.NoError(t, err)
	require.Equal(t, inidoc.Model{
		BOM:             true,
		Newline:         "\r\n",
		TrailingNewline: true,
		Preamble:        []string{"# generated", ""},
		Sections: []inidoc.ModelSection{
			{Name: "General", Lines: []string{"dns-server = 1.1.1.1"}},
			{Name: "Rule", Lines: []string{"DOMAIN,example.com,Proxy"}},
			{Name: "Rule", Lines: []string{}},
		},
	}, model)
}

func TestRenderModelProducesCanonicalINI(t *testing.T) {
	body, err := inidoc.RenderModel(inidoc.Model{
		BOM:             true,
		Newline:         "\n",
		TrailingNewline: false,
		Preamble:        []string{"# generated", ""},
		Sections: []inidoc.ModelSection{
			{Name: "General", Lines: []string{"dns-server = 1.1.1.1"}},
			{Name: "Rule", Lines: []string{"DOMAIN,example.com,Proxy"}},
			{Name: "Rule", Lines: []string{}},
		},
	})

	require.NoError(t, err)
	require.Equal(t,
		append([]byte{0xef, 0xbb, 0xbf}, []byte(
			"# generated\n"+
				"\n"+
				"[General]\n"+
				"dns-server = 1.1.1.1\n"+
				"[Rule]\n"+
				"DOMAIN,example.com,Proxy\n"+
				"[Rule]",
		)...),
		body,
	)
}

func TestParseAndRenderModelNormalizeMixedNewlines(t *testing.T) {
	model, err := inidoc.ParseModel([]byte("[General]\r\nkey = value\n[Rule]"))
	require.NoError(t, err)
	require.Equal(t, "\r\n", model.Newline)
	require.False(t, model.TrailingNewline)

	body, err := inidoc.RenderModel(model)

	require.NoError(t, err)
	require.Equal(t, "[General]\r\nkey = value\r\n[Rule]", string(body))
}

func TestParseModelReturnsCompleteEmptyDocumentShape(t *testing.T) {
	model, err := inidoc.ParseModel(nil)

	require.NoError(t, err)
	require.Equal(t, "\n", model.Newline)
	require.NotNil(t, model.Preamble)
	require.Empty(t, model.Preamble)
	require.NotNil(t, model.Sections)
	require.Empty(t, model.Sections)
}

func TestRenderModelRejectsInvalidDocuments(t *testing.T) {
	tests := []struct {
		name  string
		model inidoc.Model
	}{
		{name: "invalid newline", model: inidoc.Model{Newline: "\r"}},
		{name: "empty section name", model: inidoc.Model{Newline: "\n", Sections: []inidoc.ModelSection{{Name: " "}}}},
		{name: "section name with brackets", model: inidoc.Model{Newline: "\n", Sections: []inidoc.ModelSection{{Name: "Bad[Name"}}}},
		{name: "section name with newline", model: inidoc.Model{Newline: "\n", Sections: []inidoc.ModelSection{{Name: "Bad\nName"}}}},
		{name: "preamble line with newline", model: inidoc.Model{Newline: "\n", Preamble: []string{"bad\nline"}}},
		{name: "section line with carriage return", model: inidoc.Model{Newline: "\n", Sections: []inidoc.ModelSection{{Name: "General", Lines: []string{"bad\rline"}}}}},
		{name: "preamble creates section", model: inidoc.Model{Newline: "\n", Preamble: []string{"[General]"}}},
		{name: "body creates section", model: inidoc.Model{Newline: "\n", Sections: []inidoc.ModelSection{{Name: "General", Lines: []string{"[Rule]"}}}}},
		{name: "missing preamble", model: inidoc.Model{Newline: "\n", Sections: []inidoc.ModelSection{}}},
		{name: "missing sections", model: inidoc.Model{Newline: "\n", Preamble: []string{}}},
		{name: "missing section lines", model: inidoc.Model{Newline: "\n", Preamble: []string{}, Sections: []inidoc.ModelSection{{Name: "General"}}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := inidoc.RenderModel(tt.model)
			require.Error(t, err)
		})
	}
}
