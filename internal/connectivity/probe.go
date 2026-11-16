package connectivity

import (
	"context"
	"net/http"
	"time"
)

const ProbeURL = "http://connectivitycheck.gstatic.com/generate_204"

// Result is one probe observation. Portal is the login URL a captive portal sent us to;
// it is empty unless Internet is Captive.
type Result struct {
	Internet Internet
	Portal   string
}

// NewHTTPProbe classifies the reply of a 204 endpoint (invariants §12): 204 is online, a
// redirect to an http(s) URL is a portal at that URL, and a 200 is a portal that intercepts
// the request in place, so the probe URL itself opens its login page.
func NewHTTPProbe(url string, timeout time.Duration) Probe {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.DisableKeepAlives = true
	client := &http.Client{
		Transport:     transport,
		Timeout:       timeout,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	return func(ctx context.Context) Result {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return Result{Internet: Offline}
		}
		res, err := client.Do(req)
		if err != nil {
			return Result{Internet: Offline}
		}
		defer res.Body.Close()
		switch {
		case res.StatusCode == http.StatusNoContent:
			return Result{Internet: Online}
		case res.StatusCode == http.StatusOK:
			return Result{Internet: Captive, Portal: url}
		case res.StatusCode >= 300 && res.StatusCode < 400:
			target, err := res.Location()
			if err == nil && target.Hostname() != "" && (target.Scheme == "http" || target.Scheme == "https") {
				return Result{Internet: Captive, Portal: target.String()}
			}
		}
		return Result{Internet: Offline}
	}
}
