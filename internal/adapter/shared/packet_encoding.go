package shared

import "strings"

// NormalizePacketEncoding canonicalizes established VMess/VLESS packet
// encoding aliases while leaving unknown values intact for validation.
func NormalizePacketEncoding(value string) string {
	trimmed := strings.TrimSpace(value)
	if strings.EqualFold(trimmed, "packet") {
		return "packetaddr"
	}
	return trimmed
}
