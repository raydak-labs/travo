package api

import (
	"context"
	"errors"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/blackdark-org/travo-greenfield/internal/captive"
	"github.com/blackdark-org/travo-greenfield/internal/dnslayer"
	"github.com/blackdark-org/travo-greenfield/internal/guard"
)

// CaptiveAssist is the captive portal DNS bypass (internal/captive).
type CaptiveAssist interface {
	Enable(context.Context) error
	Restore(context.Context) error
	Status() captive.BypassStatus
}

type okOutput struct {
	Body okBody
}

func (s *server) registerCaptive(a huma.API) {
	register(s, false, op("enableCaptiveDNSBypass", http.MethodPost, "/api/v1/captive/dns-bypass",
		"Resolve through the uplink's DNS for 5 minutes so a hotel portal can load", "captive", 409, 503), a, s.enableCaptiveDNSBypass)
	register(s, false, op("restoreCaptiveDNS", http.MethodPost, "/api/v1/captive/dns-restore",
		"End the captive DNS bypass now", "captive", 409, 503), a, s.restoreCaptiveDNS)
}

func (s *server) enableCaptiveDNSBypass(ctx context.Context, _ *struct{}) (*okOutput, error) {
	if s.Captive == nil {
		return nil, captiveUnavailable()
	}
	return okOrCaptiveError(s.Captive.Enable(ctx))
}

func (s *server) restoreCaptiveDNS(ctx context.Context, _ *struct{}) (*okOutput, error) {
	if s.Captive == nil {
		return nil, captiveUnavailable()
	}
	return okOrCaptiveError(s.Captive.Restore(ctx))
}

func okOrCaptiveError(err error) (*okOutput, error) {
	switch {
	case err == nil:
		out := &okOutput{}
		out.Body.Status = "ok"
		return out, nil
	case errors.Is(err, captive.ErrNoUplink):
		return nil, fail(409, "captive_no_uplink", "No uplink is connected")
	case errors.Is(err, captive.ErrNoUplinkDNS):
		return nil, fail(409, "captive_no_uplink_dns", "The uplink did not provide a DNS server")
	case errors.Is(err, guard.ErrActive):
		return nil, fail(409, "dns_change_pending", "A DNS change did not finish; it must be retried first")
	case errors.Is(err, dnslayer.ErrForeignStaged):
		return nil, fail(409, "dns_foreign_staged", "DNS settings have unsaved changes from another tool")
	default:
		return nil, fail(503, "captive_dns_failed", "The DNS change failed")
	}
}

func captiveUnavailable() error {
	return fail(503, "captive_unavailable", "Captive portal assist is unavailable")
}
