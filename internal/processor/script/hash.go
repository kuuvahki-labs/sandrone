package script

import (
	"crypto/sha256"
	"encoding/hex"
)

func sha256Hex(in string) string {
	sum := sha256.Sum256([]byte(in))
	return hex.EncodeToString(sum[:])
}
