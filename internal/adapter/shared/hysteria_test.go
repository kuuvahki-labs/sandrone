package shared_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/adapter/shared"
)

func TestParseMihomoHysteria2HopIntervalMatchesRuntimeNormalization(t *testing.T) {
	for _, test := range []struct {
		wire string
		min  string
		max  string
	}{
		{wire: "0", min: "30s"},
		{wire: "1", min: "5s"},
		{wire: "4-9", min: "5s", max: "9s"},
		{wire: "0-0", min: "30s", max: "30s"},
		{wire: "30-5", min: "30s", max: "30s"},
		{wire: "9223372037", min: "9223372037"},
	} {
		t.Run(test.wire, func(t *testing.T) {
			minimum, maximum := shared.ParseMihomoHysteria2HopInterval(test.wire)
			require.Equal(t, test.min, minimum)
			require.Equal(t, test.max, maximum)
		})
	}
}

func TestFormatMihomoHysteria2HopIntervalRequiresWholeSeconds(t *testing.T) {
	value, err := shared.FormatMihomoHysteria2HopInterval("5s", "9s")
	require.NoError(t, err)
	require.Equal(t, "5-9", value)

	_, err = shared.FormatMihomoHysteria2HopInterval("1500ms", "")
	require.Error(t, err)
}
