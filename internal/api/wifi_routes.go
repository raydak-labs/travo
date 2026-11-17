package api

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/blackdark-org/travo-greenfield/internal/wifi"
)

type wifiOutput struct{ Body wifi.Summary }
type wifiScanOutput struct{ Body wifi.ScanResult }

func (s *server) registerWiFi(a huma.API) {
	register(s, false, op("getWiFi", http.MethodGet, "/api/v1/wifi", "WiFi radios, uplink and access points", "wifi", 503), a, s.getWiFi)
	register(s, false, op("getWiFiScan", http.MethodGet, "/api/v1/wifi/scan", "Scan nearby networks (read-only)", "wifi", 503), a, s.getWiFiScan)
	join := op(wifiJoinOperation, http.MethodPost, "/api/v1/wifi/join", "Join an uplink network (guarded transaction)", "wifi", 400, 409, 422, 503)
	join.DefaultStatus = http.StatusAccepted
	join.MaxBodyBytes = 4096
	register(s, false, join, a, s.joinWiFi)
	s.registerWiFiAPs(a)
	s.registerWiFiGuest(a)
	s.registerWiFiMAC(a)

}

func (s *server) getWiFi(ctx context.Context, _ *struct{}) (*wifiOutput, error) {
	if s.WiFi == nil {
		return nil, wifiDeviceUnavailable()
	}
	summary, err := s.WiFi.Summary(ctx)
	if err != nil {
		return nil, wifiFailure(err)
	}
	return &wifiOutput{Body: summary}, nil
}

func (s *server) getWiFiScan(ctx context.Context, _ *struct{}) (*wifiScanOutput, error) {
	if s.WiFi == nil {
		return nil, wifiDeviceUnavailable()
	}
	result, err := s.WiFi.Scan(ctx)
	if err != nil {
		return nil, wifiFailure(err)
	}
	return &wifiScanOutput{Body: result}, nil
}

func wifiDeviceUnavailable() error {
	slog.Warn("WiFi device unavailable")
	return fail(http.StatusServiceUnavailable, "device_unavailable", "WiFi device is unavailable")
}

func wifiFailure(err error) error {
	code, message := "wifi_unavailable", "WiFi is unavailable"
	if errors.Is(err, wifi.ErrScanFailed) {
		code, message = "wifi_scan_failed", "WiFi scan failed on every radio"
	}
	slog.Warn("WiFi request failed", "code", code)
	return fail(http.StatusServiceUnavailable, code, message)
}
