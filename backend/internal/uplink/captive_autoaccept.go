package uplink

import (
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/openwrt-travel-gui/backend/internal/models"
)

const (
	captiveAutoAcceptMaxBodyBytes = 256 * 1024
	captiveMaxSteps               = 8 // max pages to follow in a multi-step portal

	// captiveUbusBin is the ubus CLI used for interface state probes.
	captiveUbusBin = "/sbin/ubus"
)

// Bounded waits for the wwan bounce. Variables (not constants) so tests can
// shrink them; production values must stay small enough for an HTTP handler.
var (
	// captiveWwanDownSettle is the pause after ifdown so netifd releases the lease.
	captiveWwanDownSettle = 2 * time.Second
	// captiveWwanUpSettle bounds the wait for a fresh DHCP lease after ifup.
	captiveWwanUpSettle = 8 * time.Second
	// captiveWwanPollInterval is the retry interval while waiting for a lease.
	captiveWwanPollInterval = 200 * time.Millisecond
)

var (
	captiveAnchorRe     = regexp.MustCompile(`(?is)<a\s+[^>]*href\s*=\s*["']([^"']+)["'][^>]*>(.*?)</a>`)
	captiveCTARe        = regexp.MustCompile(`(?i)\b(accept|agree|continue|connect|sign\s*in|log\s*in|start|free\s*wifi)\b`)
	captiveFormRe       = regexp.MustCompile(`(?is)<form\s+[^>]*>(.*?)</form>`)
	captiveFormActionRe = regexp.MustCompile(`(?is)action\s*=\s*["']([^"']+)["']`)
	captiveFormMethodRe = regexp.MustCompile(`(?is)method\s*=\s*["']([^"']+)["']`)
	captiveInputRe      = regexp.MustCompile(`(?is)<input\s+([^>]*)>`)
	captiveInputNameRe  = regexp.MustCompile(`(?is)name\s*=\s*["']([^"']+)["']`)
	captiveInputValueRe = regexp.MustCompile(`(?is)value\s*=\s*["']([^"']*?)["']`)
	captiveInputTypeRe  = regexp.MustCompile(`(?is)type\s*=\s*["']([^"']+)["']`)
	captiveButtonRe     = regexp.MustCompile(`(?is)<button[^>]*>(.*?)</button>`)

	// JS redirect patterns: window.location, location.href, document.location, meta refresh
	captiveJSRedirectRe = regexp.MustCompile(
		`(?i)(?:window\.location(?:\.href)?|document\.location(?:\.href)?|location\.href)\s*=\s*["']([^"']+)["']`)
	captiveMetaRefreshRe = regexp.MustCompile(
		`(?i)<meta\s+[^>]*http-equiv\s*=\s*["']refresh["'][^>]*content\s*=\s*["']\d+;\s*url=([^"']+)["']`)
	// Auto-submit pattern: document.formName.submit(), document.forms[0].submit()
	captiveAutoSubmitRe = regexp.MustCompile(
		`(?i)document\.(?:forms\[\d+\]|\w+)\.submit\(\)`)

	captiveHTMLTagRe = regexp.MustCompile(`(?s)<[^>]+>`)
)

func (c *CaptiveService) AutoAcceptCaptivePortal(portalURL string) (models.CaptiveAutoAcceptResult, error) {
	defer c.invalidateStatus()
	portalURL = strings.TrimSpace(portalURL)
	if portalURL == "" {
		return models.CaptiveAutoAcceptResult{}, fmt.Errorf("portal_url is required")
	}

	base, err := url.Parse(portalURL)
	if err != nil || base.Scheme == "" || base.Host == "" {
		return models.CaptiveAutoAcceptResult{}, fmt.Errorf("invalid portal_url")
	}

	jar, _ := cookiejar.New(nil)
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // captive portals use self-signed certs
	}

	// Refresh guard file timestamp to prevent auto-restore during our operation
	c.refreshBypassTimestamp()

	// Client that follows redirects (for HTTPS-capable hosts like wifi.melia.com)
	client := &http.Client{
		Timeout:   15 * time.Second,
		Jar:       jar,
		Transport: transport,
	}

	// Client that does NOT follow redirects (to handle gateway→HTTPS fallback)
	noRedirectClient := &http.Client{
		Timeout:   15 * time.Second,
		Jar:       jar,
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	ua := "Mozilla/5.0 (Linux; Android 10) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Mobile Safari/537.36"
	attempted := 0
	bounceNote := ""
	currentURL := portalURL
	visitedURLs := map[string]bool{}

	// Extract gateway IP for HTTP fallback when HTTPS targets are blocked
	gatewayHost := base.Host

	// Multi-step loop: fetch page → detect action → execute → repeat
	for step := range captiveMaxSteps {
		html, pageBase, fetchErr := c.fetchPortalPage(noRedirectClient, client, ua, currentURL)
		if fetchErr != nil || html == "" {
			// If HTTPS failed, try HTTP on the gateway with the same path
			parsed, _ := url.Parse(currentURL)
			if parsed != nil && parsed.Scheme == "https" && gatewayHost != "" {
				fallbackURL := &url.URL{
					Scheme:   "http",
					Host:     gatewayHost,
					Path:     parsed.Path,
					RawQuery: parsed.RawQuery,
				}
				html, pageBase, fetchErr = c.fetchPortalPage(noRedirectClient, client, ua, fallbackURL.String())
			}
			// If still failing on step 0, bounce the active uplink's DHCP lease
			// to get a fresh one (needed after a MAC change when the gateway
			// blocks a stale MAC/IP). Only wwan is ever bounced, and only while
			// it is the active uplink, so ethernet-only or USB-tether setups are
			// not disturbed. The bounce is crash-guarded and bounded.
			if (fetchErr != nil || html == "") && step == 0 && c.cmd != nil {
				if bounceErr := c.bounceActiveWwanDHCP(); bounceErr != nil {
					bounceNote = "wwan bounce failed: " + bounceErr.Error()
					break
				}
				html, pageBase, fetchErr = c.fetchPortalPage(noRedirectClient, client, ua, currentURL)
			}
			if fetchErr != nil || html == "" {
				break
			}
		}
		base = pageBase

		// Check if internet is already reachable (skip on first step)
		if step > 0 {
			st, _ := c.CheckCaptivePortal()
			if st.CanReachInternet {
				return models.CaptiveAutoAcceptResult{
					OK:               true,
					Message:          fmt.Sprintf("internet reachable after step %d", step),
					Detected:         st.Detected,
					CanReachInternet: true,
					PortalURL:        st.PortalURL,
					Attempts:         attempted,
				}, nil
			}
		}

		// Strategy 1: Check for auto-submit forms (e.g., MikroTik external portal response)
		hasAutoSubmit := captiveAutoSubmitRe.MatchString(html)
		if hasAutoSubmit {
			allForms := extractAllForms(html, base)
			for _, form := range allForms {
				if attempted >= 10 {
					break
				}
				attempted++

				respHTML, respBase, submitErr := c.submitForm(client, ua, base, form, gatewayHost)
				if submitErr == nil {
					// Chain: if response also has auto-submit, submit that too
					c.chainAutoSubmit(client, ua, respBase, respHTML, gatewayHost, &attempted)
					st, _ := c.CheckCaptivePortal()
					if st.CanReachInternet {
						return models.CaptiveAutoAcceptResult{
							OK:               true,
							Message:          "internet reachable after auto-submit form",
							Detected:         st.Detected,
							CanReachInternet: true,
							PortalURL:        st.PortalURL,
							Attempts:         attempted,
						}, nil
					}
				}
			}
		}

		// Strategy 2: CTA forms (accept/agree buttons)
		forms := extractCaptiveForms(html, base)
		for _, form := range forms {
			if attempted >= 10 {
				break
			}
			attempted++

			respHTML, respBase, submitErr := c.submitForm(client, ua, base, form, gatewayHost)
			if submitErr == nil {
				// Chain: if response has auto-submit, submit that too
				c.chainAutoSubmit(client, ua, respBase, respHTML, gatewayHost, &attempted)
				st, _ := c.CheckCaptivePortal()
				if st.CanReachInternet {
					return models.CaptiveAutoAcceptResult{
						OK:               true,
						Message:          "internet reachable after form submission",
						Detected:         st.Detected,
						CanReachInternet: true,
						PortalURL:        st.PortalURL,
						Attempts:         attempted,
					}, nil
				}
			}
		}

		// Strategy 3: JS redirect → follow to next page
		jsRedirect := extractJSRedirect(html, base)
		if jsRedirect != "" && !visitedURLs[jsRedirect] {
			visitedURLs[jsRedirect] = true
			currentURL = jsRedirect
			continue
		}

		// Strategy 4: CTA anchor links
		targets := extractCaptiveAcceptTargets(html, base)
		for _, u := range targets {
			if attempted >= 10 {
				break
			}
			attempted++

			r2, r2Err := http.NewRequest(http.MethodGet, u, nil)
			if r2Err != nil {
				continue
			}
			r2.Header.Set("User-Agent", ua)
			r2.Header.Set("Referer", base.String())

			resp2, resp2Err := client.Do(r2)
			if resp2Err != nil {
				continue
			}

			body2, _ := io.ReadAll(io.LimitReader(resp2.Body, captiveAutoAcceptMaxBodyBytes))
			_ = resp2.Body.Close()

			html2 := string(body2)
			respBase := base
			if resp2.Request != nil && resp2.Request.URL != nil {
				respBase, _ = url.Parse(resp2.Request.URL.String())
			}

			// Chain auto-submit forms from the linked page
			c.chainAutoSubmit(client, ua, respBase, html2, gatewayHost, &attempted)

			st, _ := c.CheckCaptivePortal()
			if st.CanReachInternet {
				return models.CaptiveAutoAcceptResult{
					OK:               true,
					Message:          "internet reachable after following accept link",
					Detected:         st.Detected,
					CanReachInternet: true,
					PortalURL:        st.PortalURL,
					Attempts:         attempted,
				}, nil
			}
		}

		// No more strategies for this page
		break
	}

	st, err := c.CheckCaptivePortal()
	if err != nil {
		return models.CaptiveAutoAcceptResult{}, err
	}

	ok := st.CanReachInternet && !st.Detected
	msg := "tried captive portal auto-accept"
	if ok {
		msg = "internet reachable after auto-accept attempt"
	} else if attempted == 0 {
		msg = "no requests were sent"
	}
	if bounceNote != "" {
		msg += " (" + bounceNote + ")"
	}

	return models.CaptiveAutoAcceptResult{
		OK:               ok,
		Message:          msg,
		Detected:         st.Detected,
		CanReachInternet: st.CanReachInternet,
		PortalURL:        st.PortalURL,
		Attempts:         attempted,
	}, nil
}

// chainAutoSubmit checks if an HTML response contains auto-submit forms and submits them.
// This handles multi-step portals where submitting one form returns a page that
// auto-submits to another endpoint (e.g., external auth → MikroTik gateway).
func (c *CaptiveService) chainAutoSubmit(client *http.Client, ua string, base *url.URL, html string, gatewayHost string, attempted *int) {
	for range 3 {
		if !captiveAutoSubmitRe.MatchString(html) {
			return
		}
		allForms := extractAllForms(html, base)
		if len(allForms) == 0 {
			return
		}
		for _, form := range allForms {
			if *attempted >= 15 {
				return
			}
			*attempted++
			respHTML, respBase, err := c.submitForm(client, ua, base, form, gatewayHost)
			if err != nil {
				continue
			}
			// Continue chaining with the response
			html = respHTML
			base = respBase
		}
	}
}

// fetchPortalPage fetches a URL with redirect-aware fallback.
// If the initial request is to an HTTP gateway that redirects to HTTPS, and
// the HTTPS target fails (common in captive portals), it retries by requesting
// the redirect path via HTTP on the same gateway host.
func (c *CaptiveService) fetchPortalPage(noRedirect, followRedirect *http.Client, ua, pageURL string) (string, *url.URL, error) {
	parsed, err := url.Parse(pageURL)
	if err != nil {
		return "", nil, err
	}

	// First try: no-redirect to see if there's a redirect
	req, err := http.NewRequest(http.MethodGet, pageURL, nil)
	if err != nil {
		return "", nil, err
	}
	req.Header.Set("User-Agent", ua)

	resp, err := noRedirect.Do(req)
	if err != nil {
		return "", nil, err
	}

	// If it's a redirect, check if the target is reachable
	if resp.StatusCode >= 300 && resp.StatusCode < 400 {
		_ = resp.Body.Close()
		location := resp.Header.Get("Location")
		if location == "" {
			return "", nil, fmt.Errorf("redirect with no Location header")
		}
		redirectURL, perr := parsed.Parse(location)
		if perr != nil {
			return "", nil, perr
		}

		// Try to follow the redirect
		req2, _ := http.NewRequest(http.MethodGet, redirectURL.String(), nil)
		req2.Header.Set("User-Agent", ua)
		resp2, err2 := followRedirect.Do(req2)
		if err2 == nil {
			body, _ := io.ReadAll(io.LimitReader(resp2.Body, captiveAutoAcceptMaxBodyBytes))
			_ = resp2.Body.Close()
			base := redirectURL
			if resp2.Request != nil && resp2.Request.URL != nil {
				base = resp2.Request.URL
			}
			return string(body), base, nil
		}

		// HTTPS redirect failed — try HTTP on the original host with the redirect path
		if redirectURL.Scheme == "https" && parsed.Scheme == "http" {
			httpFallback := &url.URL{
				Scheme:   "http",
				Host:     parsed.Host,
				Path:     redirectURL.Path,
				RawQuery: redirectURL.RawQuery,
			}
			req3, _ := http.NewRequest(http.MethodGet, httpFallback.String(), nil)
			req3.Header.Set("User-Agent", ua)
			resp3, err3 := followRedirect.Do(req3)
			if err3 == nil {
				body, _ := io.ReadAll(io.LimitReader(resp3.Body, captiveAutoAcceptMaxBodyBytes))
				_ = resp3.Body.Close()
				base := httpFallback
				if resp3.Request != nil && resp3.Request.URL != nil {
					base = resp3.Request.URL
				}
				return string(body), base, nil
			}
		}
		return "", nil, fmt.Errorf("failed to follow redirect to %s", redirectURL)
	}

	// No redirect — read the response body
	body, err := io.ReadAll(io.LimitReader(resp.Body, captiveAutoAcceptMaxBodyBytes))
	_ = resp.Body.Close()
	if err != nil {
		return "", nil, err
	}
	return string(body), parsed, nil
}

// submitForm sends a form and returns the response body for chaining.
// If the form action is an HTTPS URL that fails, retries via HTTP on the gateway.
func (c *CaptiveService) submitForm(client *http.Client, ua string, referer *url.URL, form captiveFormData, gatewayHost string) (string, *url.URL, error) {
	body, base, err := c.doSubmitForm(client, ua, referer, form)
	if err != nil && gatewayHost != "" {
		// If the form action is HTTPS and failed, try HTTP on the gateway
		actionURL, perr := url.Parse(form.Action)
		if perr == nil && actionURL.Scheme == "https" && actionURL.Host != gatewayHost {
			fallbackForm := captiveFormData{
				Action: (&url.URL{
					Scheme:   "http",
					Host:     gatewayHost,
					Path:     actionURL.Path,
					RawQuery: actionURL.RawQuery,
				}).String(),
				Method: form.Method,
				Values: form.Values,
			}
			return c.doSubmitForm(client, ua, referer, fallbackForm)
		}
	}
	return body, base, err
}

func (c *CaptiveService) doSubmitForm(client *http.Client, ua string, referer *url.URL, form captiveFormData) (string, *url.URL, error) {
	var req *http.Request
	var err error

	if form.Method == http.MethodGet {
		u, perr := url.Parse(form.Action)
		if perr != nil {
			return "", nil, perr
		}
		u.RawQuery = form.Values.Encode()
		req, err = http.NewRequest(http.MethodGet, u.String(), nil)
	} else {
		req, err = http.NewRequest(http.MethodPost, form.Action, strings.NewReader(form.Values.Encode()))
		if req != nil {
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		}
	}
	if err != nil {
		return "", nil, err
	}
	req.Header.Set("User-Agent", ua)
	req.Header.Set("Referer", referer.String())

	resp, ferr := client.Do(req)
	if ferr != nil {
		return "", nil, ferr
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, captiveAutoAcceptMaxBodyBytes))
	base := referer
	if resp.Request != nil && resp.Request.URL != nil {
		base = resp.Request.URL
	}
	return string(body), base, nil
}
