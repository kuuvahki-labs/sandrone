package domain

// BackupExportResult is the raw ZIP body and download filename produced by a
// Store backup export.
type BackupExportResult struct {
	Body     []byte
	Filename string
}
