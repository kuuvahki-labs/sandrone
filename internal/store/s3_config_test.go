package store

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func validS3Config() S3Config {
	return S3Config{
		Endpoint:        "https://account.example.invalid",
		Region:          "auto",
		Bucket:          "sandrone",
		Prefix:          "sandrone/",
		AccessKeyID:     "access-marker",
		SecretAccessKey: "secret-marker",
		SessionToken:    "session-marker",
	}
}

func TestNormalizeS3Config(t *testing.T) {
	cfg := validS3Config()
	cfg.Prefix = "nested/sandrone"

	got, err := NormalizeS3Config(cfg)
	require.NoError(t, err)
	require.Equal(t, "nested/sandrone/", got.Prefix)
	require.Equal(t, cfg.Endpoint, got.Endpoint)
}

func TestNormalizeS3ConfigRejectsMissingRequiredField(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*S3Config)
		field  string
	}{
		{name: "endpoint", mutate: func(c *S3Config) { c.Endpoint = "" }, field: "SANDRONE_S3_ENDPOINT"},
		{name: "region", mutate: func(c *S3Config) { c.Region = "" }, field: "SANDRONE_S3_REGION"},
		{name: "bucket", mutate: func(c *S3Config) { c.Bucket = "" }, field: "SANDRONE_S3_BUCKET"},
		{name: "prefix", mutate: func(c *S3Config) { c.Prefix = "" }, field: "SANDRONE_S3_PREFIX"},
		{name: "access", mutate: func(c *S3Config) { c.AccessKeyID = "" }, field: "SANDRONE_S3_ACCESS_KEY_ID"},
		{name: "secret", mutate: func(c *S3Config) { c.SecretAccessKey = "" }, field: "SANDRONE_S3_SECRET_ACCESS_KEY"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validS3Config()
			tt.mutate(&cfg)
			_, err := NormalizeS3Config(cfg)
			require.ErrorContains(t, err, tt.field)
		})
	}
}

func TestNormalizeS3ConfigRejectsUnsafeEndpoint(t *testing.T) {
	for _, endpoint := range []string{
		"account.example.invalid",
		"ftp://account.example.invalid",
		"https://user:pass@account.example.invalid",
		"https://account.example.invalid?secret=x",
		"https://account.example.invalid#fragment",
	} {
		t.Run(endpoint, func(t *testing.T) {
			cfg := validS3Config()
			cfg.Endpoint = endpoint
			_, err := NormalizeS3Config(cfg)
			require.ErrorContains(t, err, "SANDRONE_S3_ENDPOINT")
		})
	}
}

func TestNormalizeS3ConfigRejectsUnsafePrefix(t *testing.T) {
	for _, prefix := range []string{"/sandrone", "../sandrone", "sandrone//nested", `sandrone\nested`} {
		t.Run(prefix, func(t *testing.T) {
			cfg := validS3Config()
			cfg.Prefix = prefix
			_, err := NormalizeS3Config(cfg)
			require.ErrorContains(t, err, "SANDRONE_S3_PREFIX")
		})
	}
}

func TestNormalizeS3ConfigDoesNotExposeSecrets(t *testing.T) {
	cfg := validS3Config()
	cfg.Endpoint = "https://user:password@example.invalid"
	_, err := NormalizeS3Config(cfg)
	require.Error(t, err)
	for _, secret := range []string{cfg.AccessKeyID, cfg.SecretAccessKey, cfg.SessionToken, "password"} {
		require.False(t, strings.Contains(err.Error(), secret))
	}
}
