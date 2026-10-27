package shared

import (
	"encoding/base64"
	"encoding/binary"
	"encoding/pem"
	"strings"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

// ParseECHConfig decodes one ECHConfigList from PEM or standard base64, with or
// without padding. Config may contain lines of the same encoded list; separate
// lists are not concatenated and unknown ECH versions remain byte-for-byte intact.
func ParseECHConfig(config []string) ([]byte, error) {
	if len(config) == 0 {
		return nil, nil
	}
	invalid := func() ([]byte, error) {
		return nil, domain.NewError(domain.CodeRenderFailed, "invalid ECH config: expected one ECHConfigList encoded as ECH CONFIGS PEM or standard base64")
	}
	text := strings.TrimSpace(strings.Join(config, "\n"))
	var data []byte
	if strings.HasPrefix(text, "-----BEGIN ") {
		block, rest := pem.Decode([]byte(text))
		if block == nil || block.Type != "ECH CONFIGS" || len(block.Headers) != 0 || strings.TrimSpace(string(rest)) != "" {
			return invalid()
		}
		data = block.Bytes
	} else {
		var err error
		data, err = base64.StdEncoding.Strict().DecodeString(text)
		if err != nil {
			data, err = base64.RawStdEncoding.Strict().DecodeString(text)
		}
		if err != nil {
			return invalid()
		}
	}
	if len(data) <= 2 || int(binary.BigEndian.Uint16(data[:2])) != len(data)-2 {
		return invalid()
	}
	for rest := data[2:]; len(rest) > 0; {
		if len(rest) < 4 {
			return invalid()
		}
		length := int(binary.BigEndian.Uint16(rest[2:4]))
		if length > len(rest)-4 {
			return invalid()
		}
		rest = rest[4+length:]
	}
	return data, nil
}

// WithRenderedECHConfig copies only the TLS branches whose ECH representation
// changes. The input node and its nested options remain safe to reuse by another
// renderer. An absent config preserves DNS-based ECH discovery.
func WithRenderedECHConfig(node domain.NodeIR, encode func([]byte) string) (domain.NodeIR, error) {
	convert := func(tls *domain.TLSOptions) (*domain.TLSOptions, error) {
		// Some targets implicitly enable TLS for protocols such as Trojan, so
		// TLS.Enabled alone cannot determine whether ECH is active here.
		if tls == nil || tls.ECH == nil || !tls.ECH.Enabled || len(tls.ECH.Config) == 0 {
			return tls, nil
		}
		data, err := ParseECHConfig(tls.ECH.Config)
		if err != nil {
			return nil, err
		}
		clonedTLS := *tls
		clonedECH := *tls.ECH
		clonedECH.Config = []string{encode(data)}
		clonedTLS.ECH = &clonedECH
		return &clonedTLS, nil
	}
	var err error
	node.TLS, err = convert(node.TLS)
	if err != nil {
		return node, err
	}
	if node.Transport != nil && node.Transport.XHTTP != nil && node.Transport.XHTTP.DownloadSettings != nil {
		download := node.Transport.XHTTP.DownloadSettings
		converted, err := convert(download.TLS)
		if err != nil {
			return node, err
		}
		if converted != download.TLS {
			clonedTransport := *node.Transport
			clonedXHTTP := *node.Transport.XHTTP
			clonedDownload := *download
			clonedDownload.TLS = converted
			clonedXHTTP.DownloadSettings = &clonedDownload
			clonedTransport.XHTTP = &clonedXHTTP
			node.Transport = &clonedTransport
		}
	}
	return node, nil
}
