package store

import (
	"fmt"
	"net/url"
	"strings"
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
		{name: "SANDRONE_S3_ENDPOINT", value: cfg.Endpoint},
		{name: "SANDRONE_S3_REGION", value: cfg.Region},
		{name: "SANDRONE_S3_BUCKET", value: cfg.Bucket},
		{name: "SANDRONE_S3_PREFIX", value: cfg.Prefix},
		{name: "SANDRONE_S3_ACCESS_KEY_ID", value: cfg.AccessKeyID},
		{name: "SANDRONE_S3_SECRET_ACCESS_KEY", value: cfg.SecretAccessKey},
	}
	for _, field := range required {
		if strings.TrimSpace(field.value) == "" {
			return S3Config{}, fmt.Errorf("%s is required", field.name)
		}
	}

	endpoint, err := url.Parse(cfg.Endpoint)
	if err != nil || endpoint.Scheme == "" || endpoint.Host == "" {
		return S3Config{}, fmt.Errorf("SANDRONE_S3_ENDPOINT must be an absolute HTTP(S) URL")
	}
	if endpoint.Scheme != "http" && endpoint.Scheme != "https" {
		return S3Config{}, fmt.Errorf("SANDRONE_S3_ENDPOINT must use HTTP or HTTPS")
	}
	if endpoint.User != nil || endpoint.RawQuery != "" || endpoint.Fragment != "" {
		return S3Config{}, fmt.Errorf("SANDRONE_S3_ENDPOINT must not contain user info, query, or fragment")
	}

	prefix := strings.TrimRight(cfg.Prefix, "/")
	clean, err := CleanKey(prefix)
	if err != nil || clean != prefix {
		return S3Config{}, fmt.Errorf("SANDRONE_S3_PREFIX must be a safe relative namespace")
	}

	cfg.Endpoint = endpoint.String()
	cfg.Region = strings.TrimSpace(cfg.Region)
	cfg.Bucket = strings.TrimSpace(cfg.Bucket)
	cfg.Prefix = clean + "/"
	return cfg, nil
}
