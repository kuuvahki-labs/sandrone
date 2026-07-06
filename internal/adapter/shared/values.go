package shared

import (
	"encoding/json"
	"fmt"
	"net"
	"net/netip"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

func StringValue(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case fmt.Stringer:
		return t.String()
	case nil:
		return ""
	default:
		return fmt.Sprint(t)
	}
}

func IntValue(v any) (int, error) {
	switch t := v.(type) {
	case int:
		return t, nil
	case int64:
		return intFromInt64(t)
	case uint64:
		return intFromUint64(t)
	case float64:
		if t > float64(maxIntValue()) || t < float64(minIntValue()) {
			return 0, fmt.Errorf("integer out of range: %v", t)
		}
		return int(t), nil
	case json.Number:
		n, err := t.Int64()
		if err != nil {
			return 0, err
		}
		return intFromInt64(n)
	case string:
		if t == "" {
			return 0, nil
		}
		return strconv.Atoi(t)
	default:
		return 0, fmt.Errorf("unsupported number type %T", v)
	}
}

func intFromInt64(value int64) (int, error) {
	if value > maxInt64ForArch() || value < minInt64ForArch() {
		return 0, fmt.Errorf("integer out of range: %d", value)
	}
	return strconv.Atoi(strconv.FormatInt(value, 10))
}

func intFromUint64(value uint64) (int, error) {
	if value > maxUint64ForArch() {
		return 0, fmt.Errorf("integer out of range: %d", value)
	}
	return strconv.Atoi(strconv.FormatUint(value, 10))
}

func maxIntValue() int {
	if strconv.IntSize == 32 {
		return 1<<31 - 1
	}
	return 1<<63 - 1
}

func minIntValue() int {
	return -maxIntValue() - 1
}

func maxInt64ForArch() int64 {
	if strconv.IntSize == 32 {
		return 1<<31 - 1
	}
	return 1<<63 - 1
}

func minInt64ForArch() int64 {
	return -maxInt64ForArch() - 1
}

func maxUint64ForArch() uint64 {
	if strconv.IntSize == 32 {
		return 1<<31 - 1
	}
	return 1<<63 - 1
}

func Uint16Value(v any) (uint16, error) {
	n, err := IntValue(v)
	if err != nil {
		return 0, err
	}
	if n < 0 || n > 65535 {
		return 0, fmt.Errorf("port out of range: %d", n)
	}
	return uint16(n), nil
}

func BoolValue(v any) bool {
	switch t := v.(type) {
	case bool:
		return t
	case string:
		switch strings.ToLower(t) {
		case "true", "1", "yes", "y", "on":
			return true
		default:
			return false
		}
	case int:
		return t != 0
	case float64:
		return t != 0
	default:
		return false
	}
}

func StringSliceValue(v any) []string {
	switch t := v.(type) {
	case nil:
		return nil
	case []string:
		return append([]string{}, t...)
	case []any:
		out := make([]string, 0, len(t))
		for _, item := range t {
			if s := StringValue(item); s != "" {
				out = append(out, s)
			}
		}
		return out
	case string:
		if t == "" {
			return nil
		}
		return []string{t}
	default:
		s := StringValue(v)
		if s == "" {
			return nil
		}
		return []string{s}
	}
}

func StringMapValue(v any) map[string]string {
	switch t := v.(type) {
	case nil:
		return nil
	case map[string]string:
		out := make(map[string]string, len(t))
		for key, value := range t {
			out[key] = value
		}
		return out
	case map[string]any:
		out := make(map[string]string, len(t))
		for key, value := range t {
			out[key] = StringValue(value)
		}
		return out
	case map[any]any:
		out := make(map[string]string, len(t))
		for key, value := range t {
			out[StringValue(key)] = StringValue(value)
		}
		return out
	default:
		return nil
	}
}

func AnyMapValue(v any) map[string]any {
	switch t := v.(type) {
	case nil:
		return nil
	case map[string]any:
		out := make(map[string]any, len(t))
		for key, value := range t {
			out[key] = value
		}
		return out
	case map[string]string:
		out := make(map[string]any, len(t))
		for key, value := range t {
			out[key] = value
		}
		return out
	case map[any]any:
		out := make(map[string]any, len(t))
		for key, value := range t {
			out[StringValue(key)] = value
		}
		return out
	default:
		return nil
	}
}

func RawNumberOrString(s string) json.RawMessage {
	if _, err := strconv.Atoi(s); err == nil {
		return json.RawMessage(s)
	}
	return json.RawMessage(strconv.Quote(s))
}

func AddRaw(raw map[string]json.RawMessage, key string, value any) {
	if raw == nil {
		return
	}
	b, err := json.Marshal(value)
	if err != nil {
		b = []byte(strconv.Quote(fmt.Sprint(value)))
	}
	raw[key] = b
}

func AddUnknownRaw(raw map[string]json.RawMessage, prefix string, doc map[string]any, known map[string]bool) {
	if raw == nil {
		return
	}
	keys := make([]string, 0, len(doc))
	for key := range doc {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if known[key] {
			continue
		}
		AddRaw(raw, prefix+key, doc[key])
	}
}

func KnownFields(keys ...string) map[string]bool {
	out := make(map[string]bool, len(keys))
	AddKnownFields(out, keys...)
	return out
}

func AddKnownFields(fields map[string]bool, keys ...string) map[string]bool {
	if fields == nil {
		fields = map[string]bool{}
	}
	for _, key := range keys {
		fields[key] = true
	}
	return fields
}

func SplitHostPortLoose(hostPart string) (string, string, error) {
	if strings.HasPrefix(hostPart, "[") {
		end := strings.Index(hostPart, "]")
		if end < 0 {
			return "", "", fmt.Errorf("invalid ipv6 host")
		}
		host := hostPart[1:end]
		if len(hostPart) <= end+2 || hostPart[end+1] != ':' {
			return "", "", fmt.Errorf("missing port")
		}
		return host, hostPart[end+2:], nil
	}
	host, port, err := net.SplitHostPort(hostPart)
	if err == nil {
		return strings.Trim(host, "[]"), port, nil
	}
	if host, port, ok := SplitBareIPv6HostPort(hostPart); ok {
		return host, port, nil
	}
	host, port, ok := strings.Cut(hostPart, ":")
	if !ok {
		return "", "", fmt.Errorf("missing port")
	}
	return host, port, nil
}

func SplitBareIPv6HostPort(hostPart string) (string, string, bool) {
	lastColon := strings.LastIndex(hostPart, ":")
	if lastColon <= 0 || lastColon == len(hostPart)-1 {
		return "", "", false
	}
	host := hostPart[:lastColon]
	addr, err := netip.ParseAddr(host)
	if err != nil || !addr.Is6() {
		return "", "", false
	}
	return host, hostPart[lastColon+1:], true
}

func ParseURLHostPort(u *url.URL, defaultPort string) (string, uint16, error) {
	host := u.Hostname()
	portStr := u.Port()
	if host == "" && u.Host != "" {
		host, portStr, _ = strings.Cut(u.Host, ":")
	}
	if portStr == "" {
		portStr = defaultPort
	}
	if host == "" {
		return "", 0, fmt.Errorf("missing host")
	}
	port, err := strconv.Atoi(portStr)
	if err != nil || port <= 0 || port > 65535 {
		return "", 0, fmt.Errorf("invalid port")
	}
	return host, uint16(port), nil
}

func QueryFirst(values url.Values, keys ...string) string {
	for _, key := range keys {
		if value := values.Get(key); value != "" {
			return value
		}
	}
	return ""
}

func DecodeName(fragment, fallback string) string {
	if fragment != "" {
		if decoded, err := url.QueryUnescape(fragment); err == nil && decoded != "" {
			return decoded
		}
		return fragment
	}
	if fallback != "" {
		return fallback
	}
	return "node"
}

func EnsureRaw(node *domain.NodeIR) {
	if node.Raw == nil {
		node.Raw = map[string]json.RawMessage{}
	}
}

func Warning(code, message, node, field, source, target string) domain.Warning {
	return domain.Warning{
		Code:    code,
		Message: message,
		Node:    node,
		Field:   field,
		Source:  source,
		Target:  target,
	}
}
