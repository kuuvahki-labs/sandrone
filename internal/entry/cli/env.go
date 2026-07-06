package cli

// CLI environment variables recognized by Sandrone. Keep new CLI env defaults
// here so they are easy to find and audit.
const (
	EnvDataDir        = "SANDRONE_DATA_DIR"
	EnvListen         = "SANDRONE_LISTEN"
	EnvToken          = "SANDRONE_TOKEN"
	EnvWebUIStaticDir = "SANDRONE_WEBUI_STATIC_DIR"
	EnvLogLevel       = "SANDRONE_LOG_LEVEL"
)
