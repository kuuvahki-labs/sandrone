package domain

// SubscriptionTrafficItem describes one source's subscription usage metadata.
type SubscriptionTrafficItem struct {
	SourceName     string `json:"source_name,omitempty" yaml:"source_name,omitempty"`
	SourceURL      string `json:"source_url,omitempty" yaml:"source_url,omitempty"`
	ObservedAt     string `json:"observed_at,omitempty" yaml:"observed_at,omitempty"`
	UploadBytes    int64  `json:"upload_bytes,omitzero" yaml:"upload_bytes,omitempty"`
	DownloadBytes  int64  `json:"download_bytes,omitzero" yaml:"download_bytes,omitempty"`
	UsedBytes      int64  `json:"used_bytes,omitzero" yaml:"used_bytes,omitempty"`
	TotalBytes     *int64 `json:"total_bytes,omitzero" yaml:"total_bytes,omitempty"`
	RemainingBytes *int64 `json:"remaining_bytes,omitzero" yaml:"remaining_bytes,omitempty"`
	ExpiresAt      string `json:"expires_at,omitempty" yaml:"expires_at,omitempty"`
	RemainingDays  *int   `json:"remaining_days,omitzero" yaml:"remaining_days,omitempty"`
	ResetDay       *int   `json:"reset_day,omitzero" yaml:"reset_day,omitempty"`
	AppURL         string `json:"app_url,omitempty" yaml:"app_url,omitempty"`
	PlanName       string `json:"plan_name,omitempty" yaml:"plan_name,omitempty"`
}
