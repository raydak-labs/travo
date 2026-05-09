package services

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

func stripTags(s string) string {
	out := captiveHTMLTagRe.ReplaceAllString(s, " ")
	return strings.Join(strings.Fields(out), " ")
}

func parseFormInputs(formBody string) url.Values {
	values := url.Values{}
	for _, inp := range captiveInputRe.FindAllStringSubmatch(formBody, -1) {
		if len(inp) < 2 {
			continue
		}
		attrs := inp[1]
		nameMatch := captiveInputNameRe.FindStringSubmatch(attrs)
		if nameMatch == nil {
			continue
		}
		name := nameMatch[1]
		value := ""
		if valMatch := captiveInputValueRe.FindStringSubmatch(attrs); valMatch != nil {
			value = valMatch[1]
		}
		typeMatch := captiveInputTypeRe.FindStringSubmatch(attrs)
		inputType := "text"
		if typeMatch != nil {
			inputType = strings.ToLower(typeMatch[1])
		}
		switch inputType {
		case "checkbox":
			if value == "" {
				value = "on"
			}
			values.Set(name, value)
		case "radio":
			if values.Get(name) == "" {
				if value == "" {
					value = "on"
				}
				values.Set(name, value)
			}
		default:
			values.Set(name, value)
		}
	}
	return values
}

func parseFormMeta(fullForm string, base *url.URL) (string, string) {
	action := ""
	if m := captiveFormActionRe.FindStringSubmatch(fullForm); m != nil {
		action = m[1]
	}
	method := "POST"
	if m := captiveFormMethodRe.FindStringSubmatch(fullForm); m != nil {
		method = strings.ToUpper(m[1])
	}
	actionURL := base.String()
	if action != "" {
		resolved, err := resolveCaptiveURL(base, action)
		if err == nil {
			actionURL = resolved
		}
	}
	return actionURL, method
}

func looksLikeCaptiveCTA(anchorInnerHTML string) bool {
	text := strings.ToLower(stripTags(anchorInnerHTML))
	return captiveCTARe.MatchString(text)
}

// extractJSRedirect finds JavaScript/meta redirect URLs from an HTML page.
func extractJSRedirect(html string, base *url.URL) string {
	// Try JS redirects: window.location = "...", location.href = "..."
	for _, m := range captiveJSRedirectRe.FindAllStringSubmatch(html, 5) {
		if len(m) < 2 {
			continue
		}
		// Unescape JS string escapes: \/ → / (common in JSON/JS)
		href := strings.ReplaceAll(m[1], `\/`, `/`)
		abs, err := resolveCaptiveURL(base, href)
		if err != nil {
			continue
		}
		return abs
	}
	// Try <meta http-equiv="refresh" content="N;url=...">
	for _, m := range captiveMetaRefreshRe.FindAllStringSubmatch(html, 3) {
		if len(m) < 2 {
			continue
		}
		abs, err := resolveCaptiveURL(base, m[1])
		if err != nil {
			continue
		}
		return abs
	}
	return ""
}

// extractAllForms extracts ALL forms from HTML (not just CTA ones).
// Used for auto-submit pages where the form might not have a submit button.
func extractAllForms(html string, base *url.URL) []captiveFormData {
	var forms []captiveFormData

	for _, formMatch := range captiveFormRe.FindAllStringSubmatch(html, 5) {
		if len(formMatch) < 2 {
			continue
		}
		fullForm := formMatch[0]
		formBody := formMatch[1]

		actionURL, method := parseFormMeta(fullForm, base)
		values := parseFormInputs(formBody)

		forms = append(forms, captiveFormData{
			Action: actionURL,
			Method: method,
			Values: values,
		})
	}
	return forms
}

func resolveCaptiveURL(base *url.URL, href string) (string, error) {
	href = strings.TrimSpace(href)
	if href == "" || strings.HasPrefix(href, "#") || strings.HasPrefix(strings.ToLower(href), "javascript:") {
		return "", fmt.Errorf("ignored href")
	}
	u, err := base.Parse(href)
	if err != nil {
		return "", err
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", fmt.Errorf("unsupported scheme %q", u.Scheme)
	}
	return u.String(), nil
}

// extractCaptiveAcceptTargets returns absolute URLs for likely "accept/continue" anchors.
func extractCaptiveAcceptTargets(html string, base *url.URL) []string {
	seen := map[string]struct{}{}
	var out []string

	for _, m := range captiveAnchorRe.FindAllStringSubmatch(html, -1) {
		if len(m) < 3 {
			continue
		}
		if !looksLikeCaptiveCTA(m[2]) {
			continue
		}
		abs, err := resolveCaptiveURL(base, m[1])
		if err != nil {
			continue
		}
		if _, ok := seen[abs]; ok {
			continue
		}
		seen[abs] = struct{}{}
		out = append(out, abs)
		if len(out) >= 8 {
			break
		}
	}

	return out
}

// captiveFormData represents a parsed form from a captive portal page.
type captiveFormData struct {
	Action string
	Method string
	Values url.Values
}

// extractCaptiveForms finds forms in HTML that look like captive portal accept forms.
func extractCaptiveForms(html string, base *url.URL) []captiveFormData {
	var forms []captiveFormData

	for _, formMatch := range captiveFormRe.FindAllStringSubmatch(html, 5) {
		if len(formMatch) < 2 {
			continue
		}
		fullForm := formMatch[0]
		formBody := formMatch[1]

		// Check if form or its buttons/inputs contain CTA keywords
		hasCTA := captiveCTARe.MatchString(stripTags(formBody))
		if !hasCTA {
			// Check button text
			for _, btn := range captiveButtonRe.FindAllStringSubmatch(formBody, -1) {
				if len(btn) >= 2 && captiveCTARe.MatchString(stripTags(btn[1])) {
					hasCTA = true
					break
				}
			}
		}
		if !hasCTA {
			// Check submit input values
			for _, inp := range captiveInputRe.FindAllStringSubmatch(formBody, -1) {
				if len(inp) < 2 {
					continue
				}
				attrs := inp[1]
				typeMatch := captiveInputTypeRe.FindStringSubmatch(attrs)
				if typeMatch != nil && strings.EqualFold(typeMatch[1], "submit") {
					valMatch := captiveInputValueRe.FindStringSubmatch(attrs)
					if valMatch != nil && captiveCTARe.MatchString(strings.ToLower(valMatch[1])) {
						hasCTA = true
						break
					}
				}
			}
		}
		if !hasCTA {
			continue
		}

		actionURL, method := parseFormMeta(fullForm, base)
		values := parseFormInputs(formBody)

		forms = append(forms, captiveFormData{
			Action: actionURL,
			Method: method,
			Values: values,
		})
		if len(forms) >= 3 {
			break
		}
	}

	return forms
}

func (c *CaptiveService) AutoAcceptCaptivePortal(portalURL string) (models.CaptiveAutoAcceptResult, error) {
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
	currentURL := portalURL
	visitedURLs := map[string]bool{}

	// Extract gateway IP for HTTP fallback when HTTPS targets are blocked
	gatewayHost := base.Host

	// Multi-step loop: fetch page → detect action → execute → repeat
	for step := 0; step < captiveMaxSteps; step++ {
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
			// If still failing on step 0, bounce DHCP to get fresh lease
			// (needed after MAC change when gateway blocks stale MAC-IP)
			if (fetchErr != nil || html == "") && step == 0 && c.cmd != nil {
				_, _ = c.cmd.Run("ifdown", "wwan")
				time.Sleep(2 * time.Second)
				_, _ = c.cmd.Run("ifup", "wwan")
				time.Sleep(8 * time.Second) // wait for DHCP
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
	for depth := 0; depth < 3; depth++ {
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

	if form.Method == "GET" {
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
