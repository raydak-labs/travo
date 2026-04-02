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
	captiveAnchorRe = regexp.MustCompile(`(?is)<a\s+[^>]*href\s*=\s*["']([^"']+)["'][^>]*>(.*?)</a>`)
	captiveCTARe    = regexp.MustCompile(`(?i)\b(accept|agree|continue|connect|sign\s*in|log\s*in|start|free\s*wifi)\b`)
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
	req.Header.Set("User-Agent", "openwrt-travel-gui-captive/1.0")

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

	targets := extractCaptiveAcceptTargets(string(body), base)
	if len(targets) == 0 {
		st, _ := c.CheckCaptivePortal()
		return models.CaptiveAutoAcceptResult{
			OK:               st.CanReachInternet,
			Message:          "no obvious accept/continue link found",
			Detected:         st.Detected,
			CanReachInternet: st.CanReachInternet,
			PortalURL:        st.PortalURL,
		}, nil
	}

	attempted := 0
	for _, u := range targets {
		if attempted >= 5 {
			break
		}
		attempted++

		r2, err := http.NewRequest(http.MethodGet, u, nil)
		if err != nil {
			continue
		}
		r2.Header.Set("User-Agent", "openwrt-travel-gui-captive/1.0")

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
