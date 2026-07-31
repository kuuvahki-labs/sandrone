package probe

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExpectedStatusMatcher(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		expression string
		status     int
		want       bool
		wantString string
	}{
		{name: "empty accepts any", status: 599, want: true, wantString: "*"},
		{name: "wildcard accepts any", expression: " * ", status: 204, want: true, wantString: "*"},
		{name: "exact match", expression: "204", status: 204, want: true, wantString: "204"},
		{name: "exact mismatch", expression: "204", status: 200, wantString: "204"},
		{name: "inclusive lower bound", expression: "200-299", status: 200, want: true, wantString: "200-299"},
		{name: "inclusive upper bound", expression: "200-299", status: 299, want: true, wantString: "200-299"},
		{name: "outside range", expression: "200-299", status: 300, wantString: "200-299"},
		{name: "slash alternatives", expression: "200/204/301-303", status: 302, want: true, wantString: "200/204/301-303"},
		{name: "comma alternatives", expression: "200, 204, 301-303", status: 204, want: true, wantString: "200/204/301-303"},
		{name: "empty alternatives", expression: "/204//", status: 204, want: true, wantString: "/204//"},
		{name: "zero", expression: "0", status: 0, want: true, wantString: "0"},
		{name: "uint16 maximum", expression: "65535", status: 65535, want: true, wantString: "65535"},
		{name: "restricted negative runtime status", expression: "0-65535", status: -1, wantString: "0-65535"},
		{name: "restricted overflow runtime status", expression: "0-65535", status: 65536, wantString: "0-65535"},
		{name: "negative runtime status", expression: "*", status: -1, want: true, wantString: "*"},
		{name: "overflow runtime status", expression: "*", status: 65536, want: true, wantString: "*"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matcher, err := parseExpectedStatus(tt.expression)
			require.NoError(t, err)
			require.Equal(t, tt.want, matcher.Match(tt.status))
			require.Equal(t, tt.wantString, matcher.String())
		})
	}
}

func TestParseExpectedStatusRejectsInvalidExpressions(t *testing.T) {
	t.Parallel()
	tests := []string{
		"abc",
		"-1",
		"65536",
		"200-",
		"-299",
		"299-200",
		"200-299-399",
		strings.Repeat("200/", maxExpectedStatusAlternatives) + "200",
	}
	for _, expression := range tests {
		t.Run(expression, func(t *testing.T) {
			_, err := parseExpectedStatus(expression)
			require.Error(t, err)
		})
	}
}
