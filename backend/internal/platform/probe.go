package platform

import (
	"io"
	"net/http"
	"time"
)

const ConnectivityProbeURL = "http://connectivitycheck.gstatic.com/generate_204"

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
			Timeout: 3 * time.Second,
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
	defer func() { _ = resp.Body.Close() }()

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
