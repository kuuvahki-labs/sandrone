package shared_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/adapter/shared"
)

func TestNormalizePacketEncodingKnownAliasOnly(t *testing.T) {
	require.Equal(t, "packetaddr", shared.NormalizePacketEncoding(" PACKET "))
	require.Equal(t, "xudp", shared.NormalizePacketEncoding(" xudp "))
	require.Equal(t, "future-value", shared.NormalizePacketEncoding(" future-value "))
}
