package uplink

import (
	"fmt"
	"net/url"
	"strings"
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
