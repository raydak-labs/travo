package models

// SQMConfig is a minimal subset of sqm-scripts UCI config for common setups.
// Stored in /etc/config/sqm as a "queue" section.
type SQMConfig struct {
	Enabled       bool   `json:"enabled"`
	Interface     string `json:"interface"`
	DownloadKbit  int    `json:"download_kbit"`
	UploadKbit    int    `json:"upload_kbit"`
	Qdisc         string `json:"qdisc"`
	Script        string `json:"script"`
	AdvancedHint  string `json:"advanced_hint,omitempty"`
	DetectedUCIID string `json:"detected_uci_section,omitempty"`
}

// SQMApplyResult is returned by POST /api/v1/sqm/apply.
type SQMApplyResult struct {
	OK     bool   `json:"ok"`
	Output string `json:"output,omitempty"`
	Error  string `json:"error,omitempty"`
}
