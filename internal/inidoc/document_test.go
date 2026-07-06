package inidoc

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParsePreservesDocumentBytesAndAggregatesDuplicateSections(t *testing.T) {
	body := []byte("\xEF\xBB\xBF; preamble\r\n\r\n[ Proxy ]\r\nAlpha = one\r\n# note\r\n[Other]\r\nunknown record\r\n[pRoXy]\r\nBeta=two")

	doc, err := Parse(body)

	require.NoError(t, err)
	require.Equal(t, body, doc.Bytes())
	require.Equal(t, []string{"Alpha = one", "# note", "Beta=two"}, doc.SectionLines("  proxy  "))
	require.Equal(t, []string{"unknown record"}, doc.SectionLines("OTHER"))
	require.Empty(t, doc.SectionLines("missing"))
}

func TestReplaceSectionCollapsesDuplicatesAtFirstOccurrenceAndPreservesUntouchedBytes(t *testing.T) {
	body := []byte("\xEF\xBB\xBF; pre\r\n[ Proxy ]\r\nold=1\r\n[Other]\r\n  untouched = yes\r\n[proxy]\r\nold=2\r\n[Tail]\r\n# exact\r\n")
	doc, err := Parse(body)
	require.NoError(t, err)

	doc.ReplaceSection(" pRoXy ", []string{"First=1", "Second=2"})

	require.Equal(t, "\xEF\xBB\xBF; pre\r\n[ Proxy ]\r\nFirst=1\r\nSecond=2\r\n[Other]\r\n  untouched = yes\r\n[Tail]\r\n# exact\r\n", string(doc.Bytes()))
	require.Equal(t, []string{"First=1", "Second=2"}, doc.SectionLines("proxy"))
}

func TestReplaceSectionAppendsMissingSectionAtEOFUsingDocumentNewline(t *testing.T) {
	doc, err := Parse([]byte("[General]\r\nvalue=1\r\n"))
	require.NoError(t, err)

	doc.ReplaceSection("Proxy", []string{"A=1", "B=2"})

	require.Equal(t, "[General]\r\nvalue=1\r\n[Proxy]\r\nA=1\r\nB=2\r\n", string(doc.Bytes()))
}

func TestParseRejectsInvalidOrEmptyHeaders(t *testing.T) {
	for _, body := range []string{"[]\n", "[   ]\n", "[broken\n", "[valid] trailing\n"} {
		t.Run(body, func(t *testing.T) {
			_, err := Parse([]byte(body))
			require.Error(t, err)
		})
	}
}
