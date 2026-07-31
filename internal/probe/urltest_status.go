package probe

import (
	"fmt"
	"strconv"
	"strings"
)

const maxExpectedStatusAlternatives = 28

type expectedStatusRange struct {
	start uint16
	end   uint16
}

type expectedStatusMatcher struct {
	expression string
	ranges     []expectedStatusRange
}

func parseExpectedStatus(raw string) (expectedStatusMatcher, error) {
	expression := strings.TrimSpace(raw)
	if expression == "" || expression == "*" {
		return expectedStatusMatcher{expression: "*"}, nil
	}
	expression = strings.ReplaceAll(expression, ",", "/")
	alternatives := strings.Split(expression, "/")
	if len(alternatives) > maxExpectedStatusAlternatives {
		return expectedStatusMatcher{}, fmt.Errorf(
			"expected_status has %d alternatives; maximum is %d",
			len(alternatives),
			maxExpectedStatusAlternatives,
		)
	}
	matcher := expectedStatusMatcher{}
	normalizedAlternatives := make([]string, len(alternatives))
	for i, alternative := range alternatives {
		alternative = strings.TrimSpace(alternative)
		normalizedAlternatives[i] = alternative
		if alternative == "" {
			continue
		}
		start, end, err := parseExpectedStatusRange(alternative)
		if err != nil {
			return expectedStatusMatcher{}, fmt.Errorf(
				"invalid expected_status alternative %q: %w",
				alternative,
				err,
			)
		}
		matcher.ranges = append(matcher.ranges, expectedStatusRange{start: start, end: end})
	}
	matcher.expression = strings.Join(normalizedAlternatives, "/")
	return matcher, nil
}

func parseExpectedStatusRange(raw string) (uint16, uint16, error) {
	parts := strings.Split(raw, "-")
	if len(parts) > 2 || parts[0] == "" || (len(parts) == 2 && parts[1] == "") {
		return 0, 0, fmt.Errorf("expected an unsigned integer or inclusive range")
	}
	startValue, err := strconv.ParseUint(strings.TrimSpace(parts[0]), 10, 16)
	if err != nil {
		return 0, 0, err
	}
	endValue := startValue
	if len(parts) == 2 {
		endValue, err = strconv.ParseUint(strings.TrimSpace(parts[1]), 10, 16)
		if err != nil {
			return 0, 0, err
		}
	}
	if startValue > endValue {
		return 0, 0, fmt.Errorf("range start %d exceeds end %d", startValue, endValue)
	}
	return uint16(startValue), uint16(endValue), nil
}

func (m expectedStatusMatcher) Match(statusCode int) bool {
	if len(m.ranges) == 0 {
		return true
	}
	if statusCode < 0 || statusCode > int(^uint16(0)) {
		return false
	}
	status := uint16(statusCode)
	for _, candidate := range m.ranges {
		if status >= candidate.start && status <= candidate.end {
			return true
		}
	}
	return false
}

func (m expectedStatusMatcher) String() string {
	if len(m.ranges) == 0 {
		return "*"
	}
	return m.expression
}
