package wifi

import (
	"context"
	"errors"
	"net/netip"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/blackdark-org/travo-greenfield/internal/device"
	"github.com/blackdark-org/travo-greenfield/internal/device/facts"
	"github.com/blackdark-org/travo-greenfield/internal/lockout"
	"github.com/blackdark-org/travo-greenfield/internal/txn"
)

type Beginner interface {
	Begin(context.Context, string, []string, func(context.Context) error, txn.Probe) (txn.Status, error)
}

type JoinDeps struct {
	UCI                     device.UCI
	Ubus                    device.Ubus
	Routes                  device.DefaultRouteReader
	Links                   device.LinkIndexReader
	Keys                    device.WirelessKeyWriter
	Neighbours              device.NeighbourReader
	Txns                    Beginner
	Clock                   func() time.Time
	ProbeBudget, ProbeEvery time.Duration
}

type Security string

const (
	SecurityOpen     Security = "open"
	SecurityWPA2     Security = "wpa2"
	SecurityWPA2WPA3 Security = "wpa2_wpa3"
	SecurityOther    Security = "other"
)

type JoinRequest struct {
	SSID, BSSID, Key string
	Security         Security
	// Acknowledge overrides a lockout refusal (invariants §2); Caller is the client IP.
	Acknowledge bool
	Caller      netip.Addr
}

var (
	ErrSecurityUnsupported = errors.New("wifi security unsupported")
	ErrNetworkNotFound     = errors.New("wifi network not in scan")
	ErrNoFreeRadio         = errors.New("no free radio for uplink")
	ErrTransactionPending  = errors.New("transaction pending")
	ErrValidation          = errors.New("invalid join request")
	ErrFirewallUnavailable = errors.New("wifi firewall unavailable")
	macPattern             = regexp.MustCompile("^([0-9a-f]{2}:){5}[0-9a-f]{2}$")
)

type ValidationError struct{ Code string }

func (e ValidationError) Error() string { return ErrValidation.Error() }
func (e ValidationError) Unwrap() error { return ErrValidation }

func (s *Service) WithJoin(d JoinDeps) *Service {
	if d.ProbeBudget <= 0 {
		d.ProbeBudget = 30 * time.Second
	}
	if d.ProbeEvery <= 0 {
		d.ProbeEvery = time.Second
	}
	s.join = &d
	return s
}

// ValidateJoinRequest lowercases the BSSID and applies every Join input rule.
// Join and saved uplink profiles share it so both accept the same credentials.
func ValidateJoinRequest(req JoinRequest) (JoinRequest, error) {
	req.BSSID = strings.ToLower(req.BSSID)
	if err := req.validate(); err != nil {
		return JoinRequest{}, err
	}
	return req, nil
}

func (r JoinRequest) validate() error {
	if err := validSSID(r.SSID); err != nil {
		return err
	}
	if r.BSSID != "" && !macPattern.MatchString(r.BSSID) {
		return ValidationError{"wifi_bssid_invalid"}
	}
	switch r.Security {
	case SecurityOpen:
		if r.Key != "" {
			return ValidationError{"wifi_key_not_allowed"}
		}
	case SecurityWPA2, SecurityWPA2WPA3:
		if r.Key == "" {
			return ValidationError{"wifi_key_required"}
		}
		if err := validKey(r.Key); err != nil {
			return err
		}
	default:
		return ErrSecurityUnsupported
	}
	return nil
}

func (s *Service) Join(ctx context.Context, req JoinRequest) (txn.Status, error) {
	req, err := ValidateJoinRequest(req)
	if err != nil {
		return txn.Status{}, err
	}
	if s.join == nil || s.join.Links == nil {
		return txn.Status{}, ErrUnavailable
	}
	radios, err := s.joinRadios(ctx)
	if err != nil {
		return txn.Status{}, err
	}
	scan, err := s.Scan(ctx)
	if err != nil {
		return txn.Status{}, ErrUnavailable
	}
	bands := joinBands(scan, req)
	if len(bands) == 0 {
		return txn.Status{}, ErrNetworkNotFound
	}
	if _, err = s.prepareJoin(ctx, req, radios, bands); err != nil {
		return txn.Status{}, err
	}
	var plan joinPlan
	var oldLinks map[int]struct{}
	st, err := s.join.Txns.Begin(ctx, "wifi: join "+req.SSID, []string{"wireless", "network", "firewall"}, func(ctx context.Context) error {
		radios, err := s.joinRadios(ctx)
		if err != nil {
			return err
		}
		if plan, err = s.prepareJoin(ctx, req, radios, bands); err != nil {
			return err
		}
		links, err := s.join.Links.LinkIndices(ctx)
		if err != nil || len(links) == 0 {
			return ErrUnavailable
		}
		oldLinks = make(map[int]struct{}, len(links))
		for _, index := range links {
			oldLinks[index] = struct{}{}
		}
		return s.stageWrites(ctx, plan.writes, req.Key)
	}, func(ctx context.Context) error { return s.probe(plan, oldLinks)(ctx) })
	if errors.Is(err, txn.ErrActive) {
		return st, ErrTransactionPending
	}
	return st, err
}

func (s *Service) joinRadios(ctx context.Context) ([]facts.Radio, error) {
	raw, err := s.join.Ubus.Call(ctx, "network.wireless", "status", nil)
	if err != nil {
		return nil, ErrUnavailable
	}
	radios, err := facts.ParseWirelessRadios(raw)
	if err != nil {
		return nil, ErrUnavailable
	}
	return radios, nil
}

func joinBands(scan ScanResult, req JoinRequest) []string {
	for _, network := range scan.Networks {
		if network.SSID != req.SSID {
			continue
		}
		if req.BSSID == "" {
			return network.Bands
		}
		for _, ap := range network.AccessPoints {
			if strings.EqualFold(ap.BSSID, req.BSSID) {
				return []string{ap.Band}
			}
		}
	}
	return nil
}

// joinPlan is a staged join: the STA radio and section, the writes in an order that
// never stages an enabled AP and STA on one radio, and the APs the probe must prove.
type joinPlan struct {
	radio, section string
	writes         []uciWrite
	expect         []apExpectation
}

func (s *Service) prepareJoin(ctx context.Context, req JoinRequest, radios []facts.Radio, bands []string) (joinPlan, error) {
	wireless, err := s.join.UCI.Show(ctx, "wireless")
	if err != nil {
		return joinPlan{}, ErrUnavailable
	}
	network, err := s.join.UCI.Show(ctx, "network")
	if err != nil {
		return joinPlan{}, ErrUnavailable
	}
	firewall, err := s.join.UCI.Show(ctx, "firewall")
	if err != nil {
		return joinPlan{}, ErrUnavailable
	}
	radio, target, moved := pickRadio(radios, bands, wireless), "", []string(nil)
	if radio == "" {
		if radio, target, moved = reconcileRadio(radios, bands, wireless); radio == "" {
			return joinPlan{}, ErrNoFreeRadio
		}
	}
	writes, before, after, err := planJoin(wireless, network, firewall, req, radio)
	if err != nil {
		return joinPlan{}, err
	}
	if len(moved) > 0 {
		writes = reconcileWrites(wireless, writes, moved, target)
		// A moved AP drops its clients: it does not count as the caller's AP staying up.
		for i := range after {
			if slices.Contains(moved, after[i].Section) {
				after[i].Enabled = false
			}
		}
	}
	caller := classifyCaller(ctx, s.join.Ubus, s.join.Neighbours, apNetworks(wireless), req.Caller)
	if err := checkJoinLockout(caller, withIfnames(ctx, s.join.Ubus, before), after, req.Acknowledge); err != nil {
		return joinPlan{}, err
	}
	section := "wwan"
	if sta := uplinkSections(wireless); len(sta) > 0 {
		section = sta[0].Name
	}
	return joinPlan{radio: radio, section: section, writes: writes, expect: apExpectations(wireless, "", false, "")}, nil
}

// checkJoinLockout runs only when the join takes an AP down (a move counts as down).
func checkJoinLockout(caller lockout.Caller, before, after []lockout.AP, acknowledge bool) error {
	if !disablesAP(before, after) {
		return nil
	}
	return lockout.Check(caller, lockout.Plan{Before: before, After: after}, acknowledge)
}

func (s *Service) probe(plan joinPlan, oldLinks map[int]struct{}) txn.Probe {
	return func(ctx context.Context) error {
		ctx, cancel := context.WithTimeout(ctx, s.join.ProbeBudget)
		defer cancel()
		ticker := time.NewTicker(s.join.ProbeEvery)
		defer ticker.Stop()
		for {
			// Every enabled AP is proven individually too: a moved AP must come up.
			if ctx.Err() == nil && s.proven(ctx, plan.radio, plan.section, oldLinks) && s.apsProven(ctx, plan.expect) {
				return nil
			}
			select {
			case <-ctx.Done():
				return errors.New("fresh STA carrier, default route and access points not proven")
			case <-ticker.C:
			}
		}
	}
}

func (s *Service) proven(ctx context.Context, radio, section string, oldLinks map[int]struct{}) bool {
	radios, err := s.joinRadios(ctx)
	if err != nil {
		return false
	}
	for _, r := range radios {
		if r.Name != radio || r.Disabled {
			continue
		}
		for _, iface := range r.Ifaces {
			if iface.Section == section && iface.Mode == "sta" && iface.Ifname != "" {
				return s.provenFreshLink(ctx, iface.Ifname, oldLinks)
			}
		}
	}
	return false
}

func (s *Service) provenFreshLink(ctx context.Context, ifname string, oldLinks map[int]struct{}) bool {
	before, err := s.join.Links.LinkIndices(ctx)
	index := before[ifname]
	if err != nil || index <= 0 {
		return false
	}
	if _, stale := oldLinks[index]; stale || !s.provenLink(ctx, ifname) {
		return false
	}
	// A name can be reused while carrier and route checks are in flight.
	after, err := s.join.Links.LinkIndices(ctx)
	return err == nil && after[ifname] == index
}

func (s *Service) provenLink(ctx context.Context, ifname string) bool {
	raw, err := s.join.Ubus.Call(ctx, "network.device", "status", map[string]string{"name": ifname})
	if err != nil {
		return false
	}
	link, err := facts.ParseDeviceStatus(raw)
	if err != nil || !link.Present || !link.Carrier {
		return false
	}
	route, found, err := s.join.Routes.DefaultIPv4Route(ctx)
	return err == nil && found && route.Device == ifname
}
