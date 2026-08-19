package uri

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/kuuvahki-labs/sandrone/internal/adapter/shared"
	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

func splitURI(raw string, schemes ...string) (authority string, query string, fragment string, err error) {
	scheme, body, ok := strings.Cut(raw, "://")
	if !ok {
		return "", "", "", domain.NewError(domain.CodeParseFailed, "missing URI scheme")
	}
	matched := false
	for _, allowed := range schemes {
		if strings.EqualFold(scheme, allowed) {
			matched = true
			break
		}
	}
	if !matched {
		return "", "", "", domain.NewError(domain.CodeParseFailed, "unsupported URI scheme")
	}
	body, fragment, _ = strings.Cut(body, "#")
	body, query, _ = strings.Cut(body, "?")
	authority, _, _ = strings.Cut(body, "/")
	if authority == "" {
		return "", "", "", domain.NewError(domain.CodeParseFailed, "missing URI authority")
	}
	return authority, query, fragment, nil
}

func splitUserInfo(authority string) (string, string) {
	at := strings.LastIndex(authority, "@")
	if at < 0 {
		return "", authority
	}
	return authority[:at], authority[at+1:]
}

func parseHysteria2Authority(authority string) (string, uint16, []string, error) {
	host, portSpec, err := splitHysteria2Authority(authority)
	if err != nil {
		return "", 0, nil, err
	}
	if host == "" {
		return "", 0, nil, fmt.Errorf("missing host")
	}
	if portSpec == "" {
		return host, 443, nil, nil
	}
	ports := splitList(portSpec)
	firstPort := ports[0]
	if start, _, ok := strings.Cut(firstPort, "-"); ok {
		firstPort = start
	}
	port, err := strconv.Atoi(firstPort)
	if err != nil || port <= 0 || port > 65535 {
		return "", 0, nil, fmt.Errorf("invalid port")
	}
	if len(ports) == 1 && ports[0] == strconv.Itoa(port) {
		return host, uint16(port), nil, nil
	}
	return host, uint16(port), ports, nil
}

func parseHysteriaMPort(values url.Values) ([]string, bool) {
	raw, ok := singleQueryValue(values, "mport")
	if !ok || strings.TrimSpace(raw) == "" {
		return nil, false
	}
	parts := strings.FieldsFunc(raw, func(value rune) bool {
		return value == ',' || value == ';'
	})
	ports := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		startText, endText, isRange := strings.Cut(part, "-")
		start, err := strconv.Atoi(startText)
		if err != nil || start <= 0 || start > 65535 {
			return nil, false
		}
		if !isRange {
			ports = append(ports, strconv.Itoa(start))
			continue
		}
		end, err := strconv.Atoi(endText)
		if err != nil || end < start || end > 65535 {
			return nil, false
		}
		ports = append(ports, strconv.Itoa(start)+"-"+strconv.Itoa(end))
	}
	return ports, len(ports) > 0
}

func equalStringLists(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func splitHysteria2Authority(authority string) (string, string, error) {
	if strings.HasPrefix(authority, "[") {
		end := strings.Index(authority, "]")
		if end < 0 {
			return "", "", fmt.Errorf("invalid ipv6 host")
		}
		host := authority[1:end]
		if len(authority) == end+1 {
			return host, "", nil
		}
		if authority[end+1] != ':' {
			return "", "", fmt.Errorf("invalid host port separator")
		}
		return host, authority[end+2:], nil
	}
	if host, portSpec, ok := shared.SplitBareIPv6HostPort(authority); ok {
		return host, portSpec, nil
	}
	if host, portSpec, ok := strings.Cut(authority, ":"); ok {
		return host, portSpec, nil
	}
	return authority, "", nil
}
