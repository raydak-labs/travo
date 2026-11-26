package wifi

import (
	"cmp"
	"context"
	"errors"
	"slices"

	"github.com/blackdark-org/travo-greenfield/internal/device/facts"
)

type ScanResult struct {
	Networks []Network     `json:"networks" nullable:"false"`
	Hidden   []AccessPoint `json:"hidden" nullable:"false" doc:"Access points that broadcast no SSID"`
	Radios   []RadioScan   `json:"radios" nullable:"false"`
}

type Network struct {
	SSID          string        `json:"ssid"`
	Bands         []string      `json:"bands" nullable:"false" enum:"2g,5g,6g"`
	BestSignalDBM int           `json:"best_signal_dbm"`
	Security      string        `json:"security" enum:"open,wpa2,wpa2_wpa3,other"`
	AccessPoints  []AccessPoint `json:"access_points" nullable:"false"`
}

type AccessPoint struct {
	BSSID        string `json:"bssid"`
	Band         string `json:"band" enum:"2g,5g,6g"`
	Channel      int    `json:"channel"`
	FrequencyMHz int    `json:"frequency_mhz"`
	SignalDBM    int    `json:"signal_dbm"`
	Quality      int    `json:"quality"`
	QualityMax   int    `json:"quality_max"`
	Security     string `json:"security" enum:"open,wpa2,wpa2_wpa3,other"`
}

type RadioScan struct {
	Radio   string `json:"radio"`
	Band    string `json:"band" enum:"2g,5g,6g,unknown"`
	Status  string `json:"status" enum:"ok,error,skipped"`
	Code    string `json:"code,omitempty" enum:"scan_failed,scan_timeout,bad_payload"`
	APCount int    `json:"ap_count"`
}

type scanFlight struct {
	done   chan struct{}
	result ScanResult
	err    error
}

func emptyScanResult() ScanResult {
	return ScanResult{Networks: []Network{}, Hidden: []AccessPoint{}, Radios: []RadioScan{}}
}

// Scan joins the in-flight site survey. Its work belongs to the daemon, not a request.
func (s *Service) Scan(ctx context.Context) (ScanResult, error) {
	s.flightMu.Lock()
	flight := s.flight
	if flight == nil {
		flight = &scanFlight{done: make(chan struct{})}
		s.flight = flight
		go s.runFlight(flight)
	}
	s.flightMu.Unlock()
	select {
	case <-flight.done:
		return flight.result, flight.err
	case <-ctx.Done():
		return ScanResult{}, ctx.Err()
	}
}

func (s *Service) runFlight(flight *scanFlight) {
	result, err := s.scan(s.daemon)
	s.flightMu.Lock()
	flight.result, flight.err = result, err
	if s.flight == flight {
		s.flight = nil
	}
	close(flight.done)
	s.flightMu.Unlock()
}

type scannedAP struct {
	ssid string
	ap   AccessPoint
}

func (s *Service) scan(ctx context.Context) (ScanResult, error) {
	out := emptyScanResult()
	reply, err := s.ubus.Call(ctx, "network.wireless", "status", nil)
	if ctxErr := ctx.Err(); ctxErr != nil {
		return out, ctxErr
	}
	if err != nil {
		return out, ErrUnavailable
	}
	radios, err := facts.ParseWirelessRadios(reply)
	if err != nil {
		return out, ErrUnavailable
	}
	groups := map[string][]AccessPoint{}
	attempted, succeeded := 0, 0
	for _, radio := range radios {
		if err := ctx.Err(); err != nil {
			return out, err
		}
		status := RadioScan{Radio: radio.Name, Band: radio.Band, Status: "ok"}
		if radio.Disabled {
			status.Status = "skipped"
			out.Radios = append(out.Radios, status)
			continue
		}
		attempted++
		aps, code := s.scanTarget(ctx, scanTarget(radio))
		if err := ctx.Err(); err != nil {
			return out, err
		}
		if code != "" {
			status.Status, status.Code = "error", code
		} else {
			succeeded++
			status.APCount = len(aps)
			for _, scanned := range aps {
				if scanned.ssid == "" {
					out.Hidden = append(out.Hidden, scanned.ap)
				} else {
					groups[scanned.ssid] = append(groups[scanned.ssid], scanned.ap)
				}
			}
		}
		out.Radios = append(out.Radios, status)
	}
	out.Networks = mergeNetworks(groups)
	sortAccessPoints(out.Hidden)
	if attempted > 0 && succeeded == 0 {
		return out, ErrScanFailed
	}
	return out, nil
}

func scanTarget(radio facts.Radio) string {
	for _, iface := range radio.Ifaces {
		if iface.Ifname != "" {
			return iface.Ifname
		}
	}
	return radio.Name
}

func (s *Service) scanTarget(ctx context.Context, target string) ([]scannedAP, string) {
	radioCtx, cancel := context.WithTimeout(ctx, s.RadioTimeout)
	reply, err := s.scanner.ScanWireless(radioCtx, target)
	timedOut := errors.Is(radioCtx.Err(), context.DeadlineExceeded)
	cancel()
	if err != nil {
		if timedOut || errors.Is(err, context.DeadlineExceeded) {
			return nil, "scan_timeout"
		}
		return nil, "scan_failed"
	}
	aps, err := facts.ParseIwinfoScan(reply)
	if err != nil {
		return nil, "bad_payload"
	}
	out := make([]scannedAP, 0, len(aps))
	for _, ap := range aps {
		band, ok := scanBand(ap.Band)
		if !ok {
			return nil, "bad_payload"
		}
		out = append(out, scannedAP{ssid: ap.SSID, ap: AccessPoint{
			BSSID: ap.BSSID, Band: band, Channel: ap.Channel, FrequencyMHz: ap.MHz,
			SignalDBM: ap.Signal, Quality: ap.Quality, QualityMax: ap.QualityMax,
			Security: securityLabel(ap.Encryption),
		}})
	}
	return out, ""
}

func scanBand(band int) (string, bool) {
	switch band {
	case 2:
		return "2g", true
	case 5:
		return "5g", true
	case 6:
		return "6g", true
	default:
		return "", false
	}
}

func securityLabel(encryption facts.Encryption) string {
	if !encryption.Enabled {
		return "open"
	}
	if slices.Contains(encryption.Authentication, "sae") && slices.Contains(encryption.Authentication, "psk") {
		return "wpa2_wpa3"
	}
	if slices.Equal(encryption.Authentication, []string{"psk"}) && slices.Contains(encryption.WPA, 2) {
		return "wpa2"
	}
	return "other"
}

func mergeNetworks(groups map[string][]AccessPoint) []Network {
	out := make([]Network, 0, len(groups))
	for ssid, aps := range groups {
		sortAccessPoints(aps)
		bands := make([]string, 0, 3)
		for _, ap := range aps {
			if !slices.Contains(bands, ap.Band) {
				bands = append(bands, ap.Band)
			}
		}
		slices.Sort(bands)
		out = append(out, Network{
			SSID: ssid, Bands: bands, BestSignalDBM: aps[0].SignalDBM,
			Security: aps[0].Security, AccessPoints: aps,
		})
	}
	slices.SortFunc(out, func(a, b Network) int {
		if bySignal := cmp.Compare(b.BestSignalDBM, a.BestSignalDBM); bySignal != 0 {
			return bySignal
		}
		return cmp.Compare(a.SSID, b.SSID)
	})
	return out
}

func sortAccessPoints(aps []AccessPoint) {
	slices.SortFunc(aps, func(a, b AccessPoint) int {
		if bySignal := cmp.Compare(b.SignalDBM, a.SignalDBM); bySignal != 0 {
			return bySignal
		}
		return cmp.Compare(a.BSSID, b.BSSID)
	})
}
