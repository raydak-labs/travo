package device

import (
	"strings"
	"testing"
)

func TestValidDiagnosticTarget(t *testing.T) {
	for _, ok := range []string{"10.0.1.1", "::1", "2001:db8::1", "example.com", "openwrt.lan", "localhost", "a-b.c3", "xn--bcher-kva.example", strings.Repeat("a", 63) + ".com"} {
		if !ValidDiagnosticTarget(ok) {
			t.Errorf("refused %q", ok)
		}
	}
	for _, bad := range []string{
		"", "-c100", "--help", "example.com; reboot", "$(reboot)", "a b", "a\nb", "example.com.", ".example.com",
		"ex_ample.com", "-example.com", "example-.com", "fe80::1%br-lan", "1.2.3.999", "1.2.3", "10.0.1.1/24",
		strings.Repeat("a", 64) + ".com", strings.Repeat("a.", 127) + "com", "exämple.com", "a..b",
	} {
		if ValidDiagnosticTarget(bad) {
			t.Errorf("accepted %q", bad)
		}
	}
}
