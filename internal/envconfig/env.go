// Package envconfig owns the environment variable names used to configure the
// Sandrone server runtime.
package envconfig

const (
	DataDir           = "SANDRONE_DATA_DIR"
	Listen            = "SANDRONE_LISTEN"
	Token             = "SANDRONE_TOKEN"
	LogLevel          = "SANDRONE_LOG_LEVEL"
	MCPPath           = "SANDRONE_MCP_PATH"
	MCPMaxOutputBytes = "SANDRONE_MCP_MAX_OUTPUT_BYTES"

	StorageBackend    = "SANDRONE_STORAGE_BACKEND"
	S3Endpoint        = "SANDRONE_S3_ENDPOINT"
	S3Region          = "SANDRONE_S3_REGION"
	S3Bucket          = "SANDRONE_S3_BUCKET"
	S3Prefix          = "SANDRONE_S3_PREFIX"
	S3ForcePathStyle  = "SANDRONE_S3_FORCE_PATH_STYLE"
	S3AccessKeyID     = "SANDRONE_S3_ACCESS_KEY_ID"
	S3SecretAccessKey = "SANDRONE_S3_SECRET_ACCESS_KEY" // #nosec G101 -- environment variable name, not a credential
	S3SessionToken    = "SANDRONE_S3_SESSION_TOKEN"     // #nosec G101 -- environment variable name, not a credential
)
