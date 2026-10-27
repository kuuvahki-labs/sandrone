package cli

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/kuuvahki-labs/sandrone/pkg/sandrone"
)

// parseURLList handles the CLI manifest, not a node document. Each entry is a
// remote input; fetched bodies are never recursively interpreted as manifests.
func parseURLList(body []byte, defaults remoteInputFlags) (*sandrone.RemoteBatchInput, error) {
	batch := &sandrone.RemoteBatchInput{}
	for index, line := range strings.Split(string(body), "\n") {
		value := strings.TrimSpace(line)
		if value == "" {
			continue
		}
		parsed, err := url.ParseRequestURI(value)
		if err != nil || parsed.Hostname() == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.User != nil {
			return nil, fmt.Errorf("URL list line %d must be an HTTP(S) URL without user credentials", index+1)
		}
		entry := defaults
		entry.inputURL = value
		batch.Sources = append(batch.Sources, *entry.remoteInput())
	}
	if len(batch.Sources) == 0 {
		return nil, fmt.Errorf("URL list must contain at least one HTTP(S) URL")
	}
	return batch, nil
}
