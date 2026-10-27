package domain

const (
	DefaultRemoteBatchConcurrency           = 4
	MaxRemoteBatchConcurrency               = 32
	CodeRemoteFetchFailed         ErrorCode = "remote_fetch_failed"
	CodeRemoteFetchCanceled       ErrorCode = "remote_fetch_canceled"
	CodeRemoteFetchTimeout        ErrorCode = "remote_fetch_timeout"
)
