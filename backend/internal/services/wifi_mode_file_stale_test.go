package services

import (
	"os"
	"path/filepath"
	"testing"
)

// The persisted mode file exists because "client" and "repeater" were thought to
// be indistinguishable in UCI. They are not: client mode leaves no access point
// enabled, repeater mode leaves one. Observed on 192.168.1.1: the file said
// "client" while radio1 ran an enabled access point, so deriveWifiMode answered
// "client", the UI showed Client as already active, and selecting Client again
// did nothing at all — the operator had no way to change mode from the UI.
//
// A refusal, a rollback, a restore or a hand edit can all leave the file
// disagreeing with the config, so the file is only trusted when the live config
// is consistent with it.
func TestDeriveWifiModeIgnoresAPersistedModeTheConfigContradicts(t *testing.T) {
	tests := []struct {
		name      string
		persisted string
		sections  map[string]map[string]string
		want      string
	}{
		{
			name:      "file says client but an access point is enabled",
			persisted: "client",
			sections: map[string]map[string]string{
				"default_radio1": {"mode": "ap", "device": "radio1", "disabled": "0"},
				"sta0":           {"mode": "sta", "device": "radio0", "disabled": "0"},
			},
			want: "repeater",
		},
		{
			name:      "file says ap but an uplink STA is enabled",
			persisted: "ap",
			sections: map[string]map[string]string{
				"default_radio0": {"mode": "ap", "device": "radio0", "disabled": "0"},
				"sta0":           {"mode": "sta", "device": "radio1", "disabled": "0"},
			},
			want: "repeater",
		},
		{
			name:      "file says repeater but no access point is enabled",
			persisted: "repeater",
			sections: map[string]map[string]string{
				"sta0": {"mode": "sta", "device": "radio0", "disabled": "0"},
			},
			want: "client",
		},
		{
			name:      "file agrees with the config",
			persisted: "client",
			sections: map[string]map[string]string{
				"sta0": {"mode": "sta", "device": "radio0", "disabled": "0"},
			},
			want: "client",
		},
		{
			name:      "file says ap and only access points are enabled",
			persisted: "ap",
			sections: map[string]map[string]string{
				"default_radio0": {"mode": "ap", "device": "radio0", "disabled": "0"},
				"default_radio1": {"mode": "ap", "device": "radio1", "disabled": "0"},
			},
			want: "ap",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc, u := newTestWifiServiceWithModeFile(
				filepath.Join(t.TempDir(), "wifi-mode"))
			if err := os.WriteFile(svc.modeFile, []byte(tc.persisted), 0o600); err != nil {
				t.Fatal(err)
			}
			// The test service seeds default_radio0/1 as ENABLED access points
			// plus an enabled sta0. Silence them so each case states the whole
			// configuration it is about.
			for _, name := range []string{"default_radio0", "default_radio1", "sta0"} {
				if err := u.Set("wireless", name, "disabled", "1"); err != nil {
					t.Fatal(err)
				}
			}
			for name, opts := range tc.sections {
				if err := u.AddSection("wireless", name, "wifi-iface"); err != nil {
					t.Fatal(err)
				}
				for k, v := range opts {
					if err := u.Set("wireless", name, k, v); err != nil {
						t.Fatal(err)
					}
				}
			}
			if got := svc.deriveWifiMode(); got != tc.want {
				t.Fatalf("persisted %q with %d sections: mode = %q, want %q",
					tc.persisted, len(tc.sections), got, tc.want)
			}
		})
	}
}
