package api

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/blackdark-org/travo-greenfield/internal/wifireconnect"
)

// WiFiReconnectReader reads the capped auto-reconnect counter (internal/wifireconnect).
type WiFiReconnectReader interface {
	Status(context.Context) (wifireconnect.Status, error)
}

type wifiReconnectOutput struct{ Body wifireconnect.Status }

func (s *server) registerWiFiReconnect(a huma.API) {
	register(s, false, op("getWiFiReconnect", http.MethodGet, "/api/v1/wifi/reconnect", "Uplink auto-reconnect attempts and cap", "wifi", 503), a, s.getWiFiReconnect)
}

func (s *server) getWiFiReconnect(ctx context.Context, _ *struct{}) (*wifiReconnectOutput, error) {
	if s.WiFiReconnect == nil {
		return nil, wifiDeviceUnavailable()
	}
	status, err := s.WiFiReconnect.Status(ctx)
	if err != nil {
		slog.Warn("WiFi reconnect state unavailable")
		return nil, fail(http.StatusServiceUnavailable, "wifi_reconnect_unavailable", "Auto-reconnect state is unavailable")
	}
	return &wifiReconnectOutput{Body: status}, nil
}
