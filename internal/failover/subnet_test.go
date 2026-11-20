package failover

import (
	"context"
	"errors"
	"testing"
)

// Live on the AXT1800: wan (cable) and wwan (WiFi) to one home router, both 10.0.1.0/24.
// With wan dropped, wwan's tracking went offline too and the policy had no member left.
func TestEnableRefusesUplinksInOneSubnet(t *testing.T) {
	r := newRig(t, true)
	for _, u := range []string{"wan", "wwan", "usbtether"} {
		r.d.SetUbusReply("network.interface."+u, "status", []byte(deviceCapture(t, "interface-"+u+"-status-failover.json")))
	}
	if _, err := r.s.Enable(context.Background(), []string{"wan", "usbtether", "wwan"}); !errors.Is(err, ErrSameSubnet) || err.Error() != ErrSameSubnet.Error()+": wan and wwan" {
		t.Fatalf("wan+wwan: %v", err)
	}
	if len(r.d.ServiceActions) != 0 || r.txns.probeErr != nil {
		t.Fatalf("refusal acted: %v", r.d.ServiceActions)
	}
	if _, err := r.s.Enable(context.Background(), []string{"wan", "usbtether"}); err != nil {
		t.Fatalf("wan+usbtether: %v", err)
	}
}
