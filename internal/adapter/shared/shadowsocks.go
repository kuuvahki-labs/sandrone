package shared

import "strings"

// NormalizeShadowsocksCipher canonicalizes established Shadowsocks cipher
// aliases while leaving unknown values intact for validation.
func NormalizeShadowsocksCipher(method string) string {
	trimmed := strings.TrimSpace(method)
	key := strings.ToLower(strings.ReplaceAll(trimmed, "_", "-"))
	aliases := map[string]string{
		"aead-chacha20-poly1305":  "chacha20-ietf-poly1305",
		"chacha20-poly1305":       "chacha20-ietf-poly1305",
		"chacha20-ietf-poly1305":  "chacha20-ietf-poly1305",
		"xchacha20-ietf-poly1305": "xchacha20-ietf-poly1305",
		"aead-aes-128-gcm":        "aes-128-gcm",
		"aes-128-gcm":             "aes-128-gcm",
		"aead-aes-192-gcm":        "aes-192-gcm",
		"aes-192-gcm":             "aes-192-gcm",
		"aead-aes-256-gcm":        "aes-256-gcm",
		"aes-256-gcm":             "aes-256-gcm",
		"aes-128-cfb":             "aes-128-cfb",
		"aes-192-cfb":             "aes-192-cfb",
		"aes-256-cfb":             "aes-256-cfb",
		"chacha20-ietf":           "chacha20-ietf",
		"plain":                   "plain",
		"none":                    "none",
	}
	if normalized, ok := aliases[key]; ok {
		return normalized
	}
	return trimmed
}

// NormalizeShadowsocksRCipher additionally accepts the historical ClashR
// "dummy" spelling for the canonical no-encryption method.
func NormalizeShadowsocksRCipher(method string) string {
	if strings.EqualFold(strings.TrimSpace(method), "dummy") {
		return "none"
	}
	return NormalizeShadowsocksCipher(method)
}
