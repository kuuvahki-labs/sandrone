package script

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSha256Hex(t *testing.T) {
	require.Equal(t, "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", sha256Hex(""))
	require.Equal(t, "2c26b46b68ffc68ff99b453c1d30413413422d706483bfa0f98a5e886266e7ae", sha256Hex("foo"))
}
