package services

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/openwrt-travel-gui/backend/internal/models"
)

const captiveAutoAcceptMaxBodyBytes = 256 * 1024

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
)

func stripTags(s string) string {
	out := regexp.MustCompile(`(?s)<[^>]+>`).ReplaceAllString(s, " ")
	return strings.Join(strings.Fields(out), " ")
}

func looksLikeCaptiveCTA(anchorInnerHTML string) bool {
	text := strings.ToLower(stripTags(anchorInnerHTML))
	return captiveCTARe.MatchString(text)
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

		// Parse action
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

		// Extract form inputs
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
				// Check all checkboxes (accept terms, etc.)
				if value == "" {
					value = "on"
				}
				values.Set(name, value)
			case "radio":
				// Only set if not already set
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

	client := &http.Client{
		Timeout: 15 * time.Second,
	}

	req, err := http.NewRequest(http.MethodGet, portalURL, nil)
	if err != nil {
		return models.CaptiveAutoAcceptResult{}, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Linux; Android 10) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Mobile Safari/537.36")

	resp, err := client.Do(req)
	if err != nil {
		return models.CaptiveAutoAcceptResult{
			OK:               false,
			Message:          "failed to fetch portal page",
			Detected:         false,
			CanReachInternet: false,
		}, nil
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, captiveAutoAcceptMaxBodyBytes))
	if err != nil {
		return models.CaptiveAutoAcceptResult{}, err
	}

	if resp.Request != nil && resp.Request.URL != nil {
		if ifu, perr := url.Parse(resp.Request.URL.String()); perr == nil {
			base = ifu
		}
	}

	attempted := 0
	html := string(body)

	// Strategy 1: Try form submissions (most modern captive portals)
	forms := extractCaptiveForms(html, base)
	for _, form := range forms {
		if attempted >= 5 {
			break
		}
		attempted++

		var formReq *http.Request
		if form.Method == "GET" {
			u, perr := url.Parse(form.Action)
			if perr != nil {
				continue
			}
			u.RawQuery = form.Values.Encode()
			formReq, err = http.NewRequest(http.MethodGet, u.String(), nil)
		} else {
			formReq, err = http.NewRequest(http.MethodPost, form.Action, strings.NewReader(form.Values.Encode()))
			if formReq != nil {
				formReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			}
		}
		if err != nil {
			continue
		}
		formReq.Header.Set("User-Agent", "Mozilla/5.0 (Linux; Android 10) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Mobile Safari/537.36")
		formReq.Header.Set("Referer", base.String())

		formResp, ferr := client.Do(formReq)
		if ferr != nil {
			continue
		}
		func() {
			defer func() { _ = formResp.Body.Close() }()
			_, _ = io.Copy(io.Discard, io.LimitReader(formResp.Body, 64*1024))
		}()

		// Check if internet is reachable now
		st, _ := c.CheckCaptivePortal()
		if st.CanReachInternet {
			return models.CaptiveAutoAcceptResult{
				OK:               true,
				Message:          "internet reachable after form submission",
				Detected:         st.Detected,
				CanReachInternet: st.CanReachInternet,
				PortalURL:        st.PortalURL,
				Attempts:         attempted,
			}, nil
		}
	}

	// Strategy 2: Try anchor links with CTA text (fallback)
	targets := extractCaptiveAcceptTargets(html, base)
	if len(targets) == 0 && len(forms) == 0 {
		st, _ := c.CheckCaptivePortal()
		return models.CaptiveAutoAcceptResult{
			OK:               st.CanReachInternet,
			Message:          "no obvious accept/continue link or form found",
			Detected:         st.Detected,
			CanReachInternet: st.CanReachInternet,
			PortalURL:        st.PortalURL,
		}, nil
	}

	for _, u := range targets {
		if attempted >= 8 {
			break
		}
		attempted++

		r2, err := http.NewRequest(http.MethodGet, u, nil)
		if err != nil {
			continue
		}
		r2.Header.Set("User-Agent", "Mozilla/5.0 (Linux; Android 10) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Mobile Safari/537.36")

		resp2, err := client.Do(r2)
		if err != nil {
			continue
		}
		func() {
			defer func() { _ = resp2.Body.Close() }()
			_, _ = io.Copy(io.Discard, io.LimitReader(resp2.Body, 64*1024))
		}()
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
