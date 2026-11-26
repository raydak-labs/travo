package connectivity

import (
	"context"
	"io"
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

// NewHTTPProbe classifies the reply of a 204 endpoint (invariants §12, ADR 0022): only the
// expected answer means online, which is a 204 or a 200 with the endpoint's known empty
// body (proxies that rewrite 204 to 200). A redirect to an http(s) URL is a portal at that
// URL. A 200 with any body is a portal that intercepts the request in place, so the probe
// URL itself opens its login page. Everything else is offline.
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
			if emptyBody(res) {
				return Result{Internet: Online}
			}
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

// emptyBody reads at most one byte: the known body of a 204 endpoint is empty, and a
// portal page is not.
func emptyBody(res *http.Response) bool {
	if res.ContentLength == 0 {
		return true
	}
	n, err := io.ReadFull(res.Body, make([]byte, 1))
	return n == 0 && (err == io.EOF || err == io.ErrUnexpectedEOF)
}
