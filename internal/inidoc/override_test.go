package inidoc

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOverridePlainSectionReplacesExactAssignmentKeysAndDeduplicatesRecords(t *testing.T) {
	base := []byte("[Proxy]\nName = old\nname=lower\nrecord\n  ; keep\n[Other]\nexact = bytes\n[proxy]\nDuplicate=from-second\n")
	patch := []byte("[ pRoXy ]\nName = new\nNAME=upper\n  record  \n; keep\nnew record\n")

	out, err := Override(base, patch)

	require.NoError(t, err)
	require.Equal(t, "[Proxy]\nName = new\nname=lower\nrecord\n  ; keep\nDuplicate=from-second\nNAME=upper\nnew record\n[Other]\nexact = bytes\n", string(out))
}

func TestOverrideTreatsRuleURLQueryAsBareRecord(t *testing.T) {
	base := []byte("[Rule]\nRULE-SET,https://example.com/list?token=old,Proxy\n")
	patch := []byte("[Rule]\nRULE-SET,https://example.com/list?token=new,Proxy\n")

	out, err := Override(base, patch)

	require.NoError(t, err)
	require.Equal(t, "[Rule]\nRULE-SET,https://example.com/list?token=old,Proxy\nRULE-SET,https://example.com/list?token=new,Proxy\n", string(out))
}

func TestOverridePreservesBOMAndConvertsPatchLinesToBaseNewline(t *testing.T) {
	base := []byte("\xEF\xBB\xBF; lead\r\n[General]\r\nmode=rule\r\n[Untouched]\r\n  exact = yes\r\n")
	patch := []byte("[General]\nmode=global\ncomment record\n")

	out, err := Override(base, patch)

	require.NoError(t, err)
	require.Equal(t, "\xEF\xBB\xBF; lead\r\n[General]\r\nmode=global\r\ncomment record\r\n[Untouched]\r\n  exact = yes\r\n", string(out))
}

func TestOverrideSectionOperators(t *testing.T) {
	base := []byte("[Target]\nbase=1\nbase record\n[Other]\nkeep=1\n")
	tests := []struct {
		name  string
		patch string
		want  string
	}{
		{
			name:  "prepend complete body",
			patch: "[+Target]\npre=1\n\n# pre comment\n",
			want:  "[Target]\npre=1\n\n# pre comment\nbase=1\nbase record\n[Other]\nkeep=1\n",
		},
		{
			name:  "append complete body",
			patch: "[Target+]\npost=1\npost record\n",
			want:  "[Target]\nbase=1\nbase record\npost=1\npost record\n[Other]\nkeep=1\n",
		},
		{
			name:  "replace complete body",
			patch: "[Target!]\nnew=1\n# replacement\n",
			want:  "[Target]\nnew=1\n# replacement\n[Other]\nkeep=1\n",
		},
		{
			name:  "delete every matching section",
			patch: "[Target-]\n# comments are allowed\n\n; so are blanks\n",
			want:  "[Other]\nkeep=1\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := Override(base, []byte(tt.patch))
			require.NoError(t, err)
			require.Equal(t, tt.want, string(out))
		})
	}
}

func TestOverrideOperatorsCreateMissingSections(t *testing.T) {
	tests := []struct {
		name  string
		patch string
		want  string
	}{
		{name: "plain", patch: "[Plain]\nx=1\n", want: "[Base]\na=1\n[Plain]\nx=1\n"},
		{name: "prepend", patch: "[+Before]\nx=1\n", want: "[Base]\na=1\n[Before]\nx=1\n"},
		{name: "append", patch: "[After+]\nx=1\n", want: "[Base]\na=1\n[After]\nx=1\n"},
		{name: "replace", patch: "[Fresh!]\nx=1\n", want: "[Base]\na=1\n[Fresh]\nx=1\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := Override([]byte("[Base]\na=1\n"), []byte(tt.patch))
			require.NoError(t, err)
			require.Equal(t, tt.want, string(out))
		})
	}
}

func TestOverrideAngleBracketsEscapeOperatorLookingLiteralSectionNames(t *testing.T) {
	base := []byte("[Section+]\na=old\n[+Section]\na=old\n[Section!]\na=old\n[Section-]\na=old\n")
	patch := []byte("[<Section+>]\na=append-literal\n[<+Section>]\na=prepend-literal\n[<Section!>]\na=replace-literal\n[<Section->]\na=delete-literal\n")

	out, err := Override(base, patch)

	require.NoError(t, err)
	require.Equal(t, "[Section+]\na=append-literal\n[+Section]\na=prepend-literal\n[Section!]\na=replace-literal\n[Section-]\na=delete-literal\n", string(out))
}

func TestOverrideExecutesDuplicatePatchSectionsInSourceOrder(t *testing.T) {
	base := []byte("[Target]\nbase\n")
	patch := []byte("[Target!]\none\n[Target+]\ntwo\n[+Target]\nzero\n")

	out, err := Override(base, patch)

	require.NoError(t, err)
	require.Equal(t, "[Target]\nzero\none\ntwo\n", string(out))
}

func TestOverrideRejectsInvalidDeleteBodiesWithoutMutatingBase(t *testing.T) {
	tests := []string{
		"[Target-]\nkey=value\n",
		"[Target-]\nunknown record\n",
	}
	for _, patch := range tests {
		t.Run(patch, func(t *testing.T) {
			base := []byte("[Target]\nkeep=1\n")
			original := append([]byte(nil), base...)

			_, err := Override(base, []byte(patch))

			require.Error(t, err)
			require.Equal(t, original, base)
			var iniErr *Error
			require.ErrorAs(t, err, &iniErr)
			require.Equal(t, "Target", iniErr.Section)
		})
	}
}

func TestOverrideIgnoresPatchPreambleWithoutASectionTarget(t *testing.T) {
	out, err := Override([]byte("[Target]\nkeep=1\n"), []byte("metadata record\n[Target]\nkey=value\n"))

	require.NoError(t, err)
	require.Equal(t, "[Target]\nkeep=1\nkey=value\n", string(out))
}
