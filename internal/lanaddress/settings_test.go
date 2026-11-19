package lanaddress

import (
	"errors"
	"testing"
)

func TestValidate(t *testing.T) {
	for addr, ok := range map[string]bool{
		"192.168.8.1/24":   true,
		" 10.0.0.1/30 ":    true,
		"172.16.5.9/27":    true,
		"192.168.8.1":      false, // no prefix
		"192.168.8.1/23":   false,
		"192.168.8.1/31":   false,
		"8.8.8.1/24":       false, // public
		"100.64.0.1/24":    false, // CGNAT is not private
		"192.168.8.0/24":   false, // network address
		"192.168.8.255/24": false, // broadcast
		"fd00::1/64":       false,
	} {
		_, err := Validate(Request{Address: addr})
		if (err == nil) != ok || (err != nil && !errors.Is(err, ErrInvalid)) {
			t.Errorf("%q: %v", addr, err)
		}
	}
}
