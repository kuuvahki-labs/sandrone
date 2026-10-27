package shared_test

import (
	"encoding/base64"
	"encoding/pem"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/adapter/shared"
	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

const echConfigFixture = "AEH+DQA9AAAgACAJAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAEAAEAAQAOcHVibGljLmV4YW1wbGUAAA=="

func TestParseECHConfigPreservesBytesAcrossEncodings(t *testing.T) {
	t.Parallel()
	want, err := base64.StdEncoding.DecodeString(echConfigFixture)
	require.NoError(t, err)
	text := string(pem.EncodeToMemory(&pem.Block{Type: "ECH CONFIGS", Bytes: want}))
	for _, config := range [][]string{
		{echConfigFixture}, {strings.TrimRight(echConfigFixture, "=")},
		{echConfigFixture[:32], echConfigFixture[32:]}, {text}, strings.Split(text, "\n"),
	} {
		actual, err := shared.ParseECHConfig(config)
		require.NoError(t, err)
		require.Equal(t, want, actual)
	}
	data, err := shared.ParseECHConfig(nil)
	require.NoError(t, err)
	require.Nil(t, data)
}

func TestParseECHConfigRejectsMalformedOrMultipleLists(t *testing.T) {
	t.Parallel()
	valid, err := base64.StdEncoding.DecodeString(echConfigFixture)
	require.NoError(t, err)
	text := string(pem.EncodeToMemory(&pem.Block{Type: "ECH CONFIGS", Bytes: valid}))
	badOuter := append([]byte(nil), valid...)
	badOuter[1]++
	badInner := append([]byte(nil), valid...)
	badInner[5]++
	for _, config := range [][]string{
		{""}, {"not-base64"}, {"AA=="}, {"AAA="},
		{strings.ReplaceAll(echConfigFixture, "+", "-")},
		{base64.StdEncoding.EncodeToString(badOuter)}, {base64.StdEncoding.EncodeToString(badInner)},
		{echConfigFixture, echConfigFixture}, {text, text},
		{strings.ReplaceAll(text, "ECH CONFIGS", "CERTIFICATE")},
		{text + "trailing-secret"},
	} {
		_, err := shared.ParseECHConfig(config)
		require.True(t, domain.IsCode(err, domain.CodeRenderFailed), "%q: %v", config, err)
		require.NotContains(t, err.Error(), "trailing-secret")
	}
}

func TestWithRenderedECHConfigDoesNotMutateSharedTLS(t *testing.T) {
	t.Parallel()
	tls := &domain.TLSOptions{Enabled: true, ECH: &domain.ECHOptions{Enabled: true, Config: []string{strings.TrimRight(echConfigFixture, "=")}}}
	node := domain.NodeIR{TLS: tls, Transport: &domain.TransportOptions{Type: "xhttp", XHTTP: &domain.XHTTPTransportOptions{
		DownloadSettings: &domain.XHTTPDownloadSettings{TLS: tls},
	}}}
	converted, err := shared.WithRenderedECHConfig(node, base64.StdEncoding.EncodeToString)
	require.NoError(t, err)
	require.Equal(t, []string{echConfigFixture}, converted.TLS.ECH.Config)
	require.Equal(t, []string{echConfigFixture}, converted.Transport.XHTTP.DownloadSettings.TLS.ECH.Config)
	require.Equal(t, []string{strings.TrimRight(echConfigFixture, "=")}, tls.ECH.Config)
	require.NotSame(t, node.TLS, converted.TLS)
	require.NotSame(t, node.Transport, converted.Transport)
	require.Same(t, tls, node.Transport.XHTTP.DownloadSettings.TLS)
}

func TestWithRenderedECHConfigPreservesDisabledBranches(t *testing.T) {
	t.Parallel()
	for _, tlsEnabled := range []bool{false, true} {
		tls := &domain.TLSOptions{Enabled: tlsEnabled, ECH: &domain.ECHOptions{Config: []string{"invalid-config"}}}
		node := domain.NodeIR{TLS: tls, Transport: &domain.TransportOptions{Type: "xhttp", XHTTP: &domain.XHTTPTransportOptions{
			DownloadSettings: &domain.XHTTPDownloadSettings{TLS: tls},
		}}}
		converted, err := shared.WithRenderedECHConfig(node, func([]byte) string {
			t.Fatal("disabled ECH must not be encoded")
			return ""
		})
		require.NoError(t, err)
		require.Same(t, node.TLS, converted.TLS)
		require.Same(t, node.Transport, converted.Transport)
		require.Equal(t, []string{"invalid-config"}, tls.ECH.Config)
	}
}
