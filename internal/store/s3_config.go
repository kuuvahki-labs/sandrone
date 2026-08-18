package store

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/kuuvahki-labs/sandrone/internal/envconfig"
)

type S3Config struct {
	Endpoint        string
	Region          string
	Bucket          string
	Prefix          string
	ForcePathStyle  bool
	AccessKeyID     string
	SecretAccessKey string
	SessionToken    string
}

func NormalizeS3Config(cfg S3Config) (S3Config, error) {
	required := []struct {
		name  string
		value string
	}{
		{name: envconfig.S3Endpoint, value: cfg.Endpoint},
		{name: envconfig.S3Region, value: cfg.Region},
		{name: envconfig.S3Bucket, value: cfg.Bucket},
		{name: envconfig.S3Prefix, value: cfg.Prefix},
		{name: envconfig.S3AccessKeyID, value: cfg.AccessKeyID},
		{name: envconfig.S3SecretAccessKey, value: cfg.SecretAccessKey},
	}
	for _, field := range required {
		if strings.TrimSpace(field.value) == "" {
			return S3Config{}, fmt.Errorf("%s is required", field.name)
		}
	}

	endpoint, err := url.Parse(cfg.Endpoint)
	if err != nil || endpoint.Scheme == "" || endpoint.Host == "" {
		return S3Config{}, fmt.Errorf("%s must be an absolute HTTP(S) URL", envconfig.S3Endpoint)
	}
	if endpoint.Scheme != "http" && endpoint.Scheme != "https" {
		return S3Config{}, fmt.Errorf("%s must use HTTP or HTTPS", envconfig.S3Endpoint)
	}
	if endpoint.User != nil || endpoint.RawQuery != "" || endpoint.Fragment != "" {
		return S3Config{}, fmt.Errorf("%s must not contain user info, query, or fragment", envconfig.S3Endpoint)
	}

	prefix := strings.TrimRight(cfg.Prefix, "/")
	clean, err := CleanKey(prefix)
	if err != nil || clean != prefix {
		return S3Config{}, fmt.Errorf("%s must be a safe relative namespace", envconfig.S3Prefix)
	}

	cfg.Endpoint = endpoint.String()
	cfg.Region = strings.TrimSpace(cfg.Region)
	cfg.Bucket = strings.TrimSpace(cfg.Bucket)
	cfg.Prefix = clean + "/"
	return cfg, nil
}
