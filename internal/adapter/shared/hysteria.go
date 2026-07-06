package shared

import "github.com/kuuvahki-labs/sandrone/internal/domain"

// HysteriaV1Obfs returns canonical mode/password values while preserving the
// legacy NodeIR shape that stored a non-URI Hysteria v1 password in Obfs.
func HysteriaV1Obfs(node domain.NodeIR) (string, string) {
	if node.Hysteria == nil {
		return "", ""
	}
	mode := node.Hysteria.Obfs
	password := node.Hysteria.ObfsPassword
	if password == "" && mode != "" && node.SourceFormat != "uri" {
		return "", mode
	}
	return mode, password
}
