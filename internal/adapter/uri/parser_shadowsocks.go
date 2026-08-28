package uri

import (
	"encoding/json"
	"net/url"
	"strconv"
	"strings"

	"github.com/kuuvahki-labs/sandrone/internal/adapter/shared"
	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

func parseSS(raw string) (domain.NodeIR, *domain.SourceInfo, error) {
	node := domain.NodeIR{Type: domain.NodeTypeShadowsocks, SourceFormat: "uri"}
	source := shared.SourceInfo("ss", shared.SourceRefs("ss"))
	body := strings.TrimPrefix(raw, "ss://")
	base, fragment, _ := strings.Cut(body, "#")
	base, queryStr, _ := strings.Cut(base, "?")
	if base == "" {
		return node, source, domain.NewError(domain.CodeParseFailed, "missing ss payload")
	}
	if !strings.Contains(base, "@") {
		if decoded, ok := decodeBase64String(base); ok && strings.Contains(decoded, "@") {
			base = decoded
			if decodedBase, decodedQuery, ok := strings.Cut(base, "?"); ok {
				base = decodedBase
				if queryStr == "" {
					queryStr = decodedQuery
				}
			}
		}
	}
	userInfo, hostPart, hasHost := strings.Cut(base, "@")
	var methodPassword ssCredentials
	var err error
	if hasHost {
		methodPassword, err = decodeSSUserInfo(userInfo)
	} else {
		methodPassword, hostPart, err = decodeLegacySSPayload(base)
	}
	if err != nil {
		return node, source, err
	}
	hostPart = strings.TrimSuffix(hostPart, "/")
	host, portStr, err := shared.SplitHostPortLoose(hostPart)
	if err != nil {
		return node, source, domain.WrapError(domain.CodeParseFailed, "split ss server host and port", err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil || port <= 0 || port > 65535 {
		return node, source, domain.NewError(domain.CodeParseFailed, "invalid ss port")
	}
	node.Name = shared.DecodeName(fragment, host)
	node.Server = host
	node.Port = uint16(port)
	node.Cipher = methodPassword.Method
	node.Password = methodPassword.Password
	node.Raw = map[string]json.RawMessage{}
	if queryStr != "" {
		values, err := url.ParseQuery(queryStr)
		if err != nil {
			return node, source, domain.WrapError(domain.CodeParseFailed, "parse ss query", err)
		}
		uot, uotKnown := preferredQueryBool(values, "uot")
		if len(uotKnown) > 0 {
			node.UDPOverTCP = &domain.UDPOverTCPOptions{Enabled: uot}
		}
		tfo, tfoKnown := preferredQueryBool(values, "tfo")
		if len(tfoKnown) > 0 && tfo {
			ensureDialer(&node).TFO = true
		}
		for _, key := range sortedQueryKeys(values) {
			switch key {
			case "plugin":
				applySIP002Plugin(&node, values.Get(key))
			case "plugin-opts", "plugin_opts":
				if node.PluginOptions == nil {
					node.PluginOptions = map[string]any{"raw": values.Get(key)}
				}
			default:
				if uotKnown[key] || tfoKnown[key] {
					continue
				}
				node.Raw["uri.query."+key] = json.RawMessage(strconv.Quote(values.Get(key)))
			}
		}
	}
	return node, source, nil
}

type ssCredentials struct {
	Method   string
	Password string
}

func parseSSR(raw string) (domain.NodeIR, *domain.SourceInfo, error) {
	node := domain.NodeIR{Type: domain.NodeTypeShadowsocksR, SourceFormat: "uri"}
	source := shared.SourceInfo("ssr", shared.SourceRefs("ssr"))
	payload := strings.TrimPrefix(raw, "ssr://")
	decoded, ok := decodeBase64String(payload)
	if !ok {
		return node, source, domain.NewError(domain.CodeParseFailed, "decode ssr payload")
	}
	base, queryStr, _ := strings.Cut(decoded, "/?")
	parts := strings.Split(base, ":")
	if len(parts) != 6 {
		return node, source, domain.NewError(domain.CodeParseFailed, "invalid ssr payload")
	}
	port, err := strconv.Atoi(parts[1])
	if err != nil || port <= 0 || port > 65535 {
		return node, source, domain.NewError(domain.CodeParseFailed, "invalid ssr port")
	}
	password, ok := decodeBase64String(parts[5])
	if !ok || password == "" {
		return node, source, domain.NewError(domain.CodeParseFailed, "decode ssr password")
	}
	node.Server = parts[0]
	node.Port = uint16(port)
	node.Cipher = shared.NormalizeShadowsocksRCipher(parts[3])
	node.Password = password
	node.ShadowsocksR = &domain.ShadowsocksROptions{
		Protocol: parts[2],
		Obfs:     parts[4],
	}
	node.Name = shared.DecodeName("", node.Server)
	node.Raw = map[string]json.RawMessage{}
	if queryStr != "" {
		values, err := url.ParseQuery(queryStr)
		if err != nil {
			return node, source, domain.WrapError(domain.CodeParseFailed, "parse ssr query", err)
		}
		if remarks := decodeSSRQueryValue(values.Get("remarks")); remarks != "" {
			node.Name = remarks
		}
		node.ShadowsocksR.ProtocolParam = decodeSSRQueryValue(values.Get("protoparam"))
		node.ShadowsocksR.ObfsParam = decodeSSRQueryValue(values.Get("obfsparam"))
		preserveURIQuery(&node, values, map[string]bool{
			"remarks": true, "group": true, "protoparam": true, "obfsparam": true,
		})
	}
	return node, source, nil
}

func decodeSSRQueryValue(value string) string {
	if value == "" {
		return ""
	}
	if decoded, ok := decodeBase64String(value); ok {
		return decoded
	}
	return value
}

func decodeLegacySSPayload(s string) (ssCredentials, string, error) {
	credentials, err := decodeSSUserInfo(s)
	if err != nil {
		return ssCredentials{}, "", err
	}
	password, hostPart, ok := strings.Cut(credentials.Password, "@")
	if !ok || hostPart == "" {
		return ssCredentials{}, "", domain.NewError(domain.CodeParseFailed, "missing ss server")
	}
	credentials.Password = password
	return credentials, hostPart, nil
}

func decodeSSUserInfo(s string) (ssCredentials, error) {
	if decoded, ok := decodeBase64String(s); ok {
		method, password, ok := strings.Cut(decoded, ":")
		if !ok || method == "" {
			return ssCredentials{}, domain.NewError(domain.CodeParseFailed, "invalid ss credentials")
		}
		return ssCredentials{Method: shared.NormalizeShadowsocksCipher(method), Password: password}, nil
	}
	if unescaped, unescapeErr := url.PathUnescape(s); unescapeErr == nil && strings.Contains(unescaped, ":") {
		method, password, ok := strings.Cut(unescaped, ":")
		if ok && method != "" {
			return ssCredentials{Method: shared.NormalizeShadowsocksCipher(method), Password: password}, nil
		}
	}
	return ssCredentials{}, domain.NewError(domain.CodeParseFailed, "decode ss userinfo")
}

func applySIP002Plugin(node *domain.NodeIR, plugin string) {
	if plugin == "" {
		return
	}
	name, opts, hasOpts := strings.Cut(plugin, ";")
	if name == "" || !hasOpts {
		node.Plugin = plugin
		return
	}
	node.Plugin = name
	if opts != "" {
		node.PluginOptions = map[string]any{"raw": opts}
	}
}
