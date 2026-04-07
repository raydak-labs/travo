package models

// FailoverCandidateKind identifies the uplink type shown in the UI.
type FailoverCandidateKind string

const (
	FailoverCandidateKindEthernet FailoverCandidateKind = "ethernet"
	FailoverCandidateKindWiFi     FailoverCandidateKind = "wifi"
	FailoverCandidateKindUSB      FailoverCandidateKind = "usb"
)

// FailoverTrackingState describes the current runtime health summary for a candidate.
type FailoverTrackingState string

const (
	FailoverTrackingStateOnline       FailoverTrackingState = "online"
	FailoverTrackingStateOffline      FailoverTrackingState = "offline"
	FailoverTrackingStateDisabled     FailoverTrackingState = "disabled"
	FailoverTrackingStateNotInstalled FailoverTrackingState = "not_installed"
	FailoverTrackingStateNotAvailable FailoverTrackingState = "not_available"
	FailoverTrackingStateUnknown      FailoverTrackingState = "unknown"
)

// FailoverHealthConfig contains the shared mwan3 tracking parameters used by the app.
type FailoverHealthConfig struct {
	TrackIPs         []string `json:"track_ips"`
	Reliability      int      `json:"reliability"`
	Count            int      `json:"count"`
	Timeout          int      `json:"timeout"`
	Interval         int      `json:"interval"`
	FailureInterval  int      `json:"failure_interval"`
	RecoveryInterval int      `json:"recovery_interval"`
	Down             int      `json:"down"`
	Up               int      `json:"up"`
}

// FailoverCandidate is a single orderable UI row.
type FailoverCandidate struct {
	ID            string                `json:"id"`
	Label         string                `json:"label"`
	InterfaceName string                `json:"interface_name"`
	Kind          FailoverCandidateKind `json:"kind"`
	Available     bool                  `json:"available"`
	Enabled       bool                  `json:"enabled"`
	Priority      int                   `json:"priority"`
	TrackingState FailoverTrackingState `json:"tracking_state"`
	IsUp          bool                  `json:"is_up"`
}

// FailoverEvent describes an observed active-uplink switch.
type FailoverEvent struct {
	FromInterface string `json:"from_interface"`
	ToInterface   string `json:"to_interface"`
	Timestamp     int64  `json:"timestamp"`
	Reason        string `json:"reason"`
}

// FailoverConfig is the top-level GET/PUT contract.
type FailoverConfig struct {
	Available         bool                 `json:"available"`
	ServiceInstalled  bool                 `json:"service_installed"`
	Enabled           bool                 `json:"enabled"`
	ActiveInterface   string               `json:"active_interface"`
	Candidates        []FailoverCandidate  `json:"candidates"`
	Health            FailoverHealthConfig `json:"health"`
	LastFailoverEvent *FailoverEvent       `json:"last_failover_event,omitempty"`
}
