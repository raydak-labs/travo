package services

import (
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/openwrt-travel-gui/backend/internal/models"
)

const captiveProbeURL = "http://detectportal.firefox.com/canonical.html"

// HTTPProber performs HTTP probes for captive portal detection.
type HTTPProber interface {
	// Do sends a GET request and returns status code, body, redirect URL (if any), and error.
	Do(url string) (statusCode int, body string, redirectURL string, err error)
}

// RealHTTPProber uses net/http with redirect checking disabled.
type RealHTTPProber struct {
	client *http.Client
}

// NewRealHTTPProber creates a prober with a 5-second timeout and no-redirect policy.
func NewRealHTTPProber() *RealHTTPProber {
	return &RealHTTPProber{
		client: &http.Client{
			Timeout: 5 * time.Second,
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
}

// Do performs an HTTP GET and returns status, body, redirect URL, and error.
func (p *RealHTTPProber) Do(url string) (int, string, string, error) {
	resp, err := p.client.Get(url)
	if err != nil {
		return 0, "", "", err
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, "", "", err
	}

	var redirectURL string
	if loc := resp.Header.Get("Location"); loc != "" {
		redirectURL = loc
	}

	return resp.StatusCode, string(bodyBytes), redirectURL, nil
}

// MockHTTPProber returns preset responses for testing.
type MockHTTPProber struct {
	StatusCode  int
	Body        string
	RedirectURL string
	Err         error
}

// Do returns the preset mock response.
func (m *MockHTTPProber) Do(_ string) (int, string, string, error) {
	return m.StatusCode, m.Body, m.RedirectURL, m.Err
}

// CaptiveService checks for captive portal detection.
type CaptiveService struct {
	prober HTTPProber
}

// NewCaptiveService creates a new CaptiveService with the given HTTP prober.
func NewCaptiveService(prober HTTPProber) *CaptiveService {
	return &CaptiveService{prober: prober}
}

// CheckCaptivePortal probes for captive portals by making an HTTP request
// to a known endpoint and checking for redirects or unexpected responses.
func (c *CaptiveService) CheckCaptivePortal() (models.CaptivePortalStatus, error) {
	statusCode, body, redirectURL, err := c.prober.Do(captiveProbeURL)
	if err != nil {
		// Connection failed — no internet
		return models.CaptivePortalStatus{
			Detected:         false,
			CanReachInternet: false,
		}, nil
	}

	// Check for redirect (captive portal intercept)
	if statusCode == http.StatusMovedPermanently ||
		statusCode == http.StatusFound ||
		statusCode == http.StatusTemporaryRedirect {
		portalURL := redirectURL
		return models.CaptivePortalStatus{
			Detected:         true,
			PortalURL:        &portalURL,
			CanReachInternet: false,
		}, nil
	}

	// Check for expected response
	if statusCode == http.StatusOK && strings.Contains(body, "success") {
		return models.CaptivePortalStatus{
			Detected:         false,
			CanReachInternet: true,
		}, nil
	}

	// Status 200 but wrong body — likely a captive portal login page
	return models.CaptivePortalStatus{
		Detected:         true,
		CanReachInternet: false,
	}, nil
}
