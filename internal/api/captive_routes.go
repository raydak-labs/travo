package api

import (
	"context"
	"errors"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/blackdark-org/travo-greenfield/internal/captive"
	"github.com/blackdark-org/travo-greenfield/internal/connectivity"
	"github.com/blackdark-org/travo-greenfield/internal/dnslayer"
	"github.com/blackdark-org/travo-greenfield/internal/guard"
)

// CaptiveAssist is the captive portal DNS bypass (internal/captive).
type CaptiveAssist interface {
	Enable(context.Context) error
	Restore(context.Context) error
	Status() captive.BypassStatus
}

// CaptiveRenewer is the guarded wwan DHCP renew (internal/captive).
type CaptiveRenewer interface {
	Renew(context.Context) error
	Availability(context.Context, connectivity.Status) captive.RenewState
}

// CaptiveAcceptor walks click-through portal pages (internal/captive, ADR 0023).
type CaptiveAcceptor interface {
	Accept(context.Context) (captive.AcceptResult, error)
}

type acceptOutput struct {
	Body captive.AcceptResult
}

type okOutput struct {
	Body okBody
}

func (s *server) registerCaptive(a huma.API) {
	register(s, false, op("enableCaptiveDNSBypass", http.MethodPost, "/api/v1/captive/dns-bypass",
		"Resolve through the uplink's DNS for 5 minutes so a hotel portal can load", "captive", 409, 503), a, s.enableCaptiveDNSBypass)
	register(s, false, op("restoreCaptiveDNS", http.MethodPost, "/api/v1/captive/dns-restore",
		"End the captive DNS bypass now", "captive", 409, 503), a, s.restoreCaptiveDNS)
	register(s, false, op("renewCaptiveDHCP", http.MethodPost, "/api/v1/captive/dhcp-renew",
		"Renew the wwan DHCP lease for a hotel portal", "captive", 409, 503), a, s.renewCaptiveDHCP)
	register(s, false, op("autoAcceptCaptivePortal", http.MethodPost, "/api/v1/captive/auto-accept",
		"Accept a click-through hotel portal automatically", "captive", 409, 503), a, s.autoAcceptCaptivePortal)
}

// autoAcceptCaptivePortal reports every walk outcome as 200: needs_user, no_form and
// failed are expected answers, not errors.
func (s *server) autoAcceptCaptivePortal(ctx context.Context, _ *struct{}) (*acceptOutput, error) {
	if s.CaptiveAccept == nil {
		return nil, captiveUnavailable()
	}
	result, err := s.CaptiveAccept.Accept(ctx)
	switch {
	case errors.Is(err, captive.ErrAcceptNotCaptive):
		return nil, fail(409, "captive_accept_not_captive", "No captive portal is detected")
	case errors.Is(err, captive.ErrAcceptBusy):
		return nil, fail(409, "captive_accept_busy", "An automatic login is already running")
	case err != nil:
		return nil, fail(503, "captive_accept_failed", "The automatic login failed")
	}
	return &acceptOutput{Body: result}, nil
}

func (s *server) renewCaptiveDHCP(ctx context.Context, _ *struct{}) (*okOutput, error) {
	if s.CaptiveRenew == nil {
		return nil, captiveUnavailable()
	}
	err := s.CaptiveRenew.Renew(ctx)
	switch {
	case errors.Is(err, captive.ErrRenewNotApplicable):
		return nil, fail(409, "captive_renew_not_applicable", "Renewing applies only to a WiFi uplink without internet")
	case errors.Is(err, captive.ErrRenewBusy):
		return nil, fail(409, "captive_renew_busy", "A renew is already running")
	case errors.Is(err, captive.ErrRenewUnproven):
		return nil, fail(503, "captive_renew_unproven", "The uplink did not come back after the renew")
	case err != nil:
		return nil, fail(503, "captive_renew_failed", "The DHCP renew failed")
	}
	out := &okOutput{}
	out.Body.Status = "ok"
	return out, nil
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
