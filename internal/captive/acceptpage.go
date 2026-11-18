package captive

import (
	"io"
	"net/http"
	"net/url"
	"strings"

	"golang.org/x/net/html"
)

// PlanKind is what the walker does with one portal page (ADR 0023).
type PlanKind string

const (
	// PlanSubmit sends the page's only form with its hidden fields and ticked boxes.
	PlanSubmit PlanKind = "submit"
	// PlanFollow loads a meta refresh target.
	PlanFollow PlanKind = "follow"
	// PlanNeedsUser: the page asks for data only the user has (room, name, voucher).
	PlanNeedsUser PlanKind = "needs_user"
	// PlanNone: nothing the walker understands.
	PlanNone PlanKind = "none"
)

// Plan is the next request for a portal page; Target is absolute http(s).
type Plan struct {
	Kind   PlanKind
	Method string
	Target *url.URL
	Fields url.Values
}

// form is one parsed <form>; userInput marks a visible empty field only a person can fill.
type form struct {
	method, action string
	fields         url.Values
	submit         *[2]string // name, value of the first named submit control
	sawSubmit      bool
	userInput      bool
	radios         map[string]bool // true once a checked radio of that name was taken
}

// PlanPage reads a portal page and picks the next step. Pure: base is the page's URL.
// Exactly one form without user input is submitted; any form that needs a person, or
// more than one candidate, stops the walk; a meta refresh is followed otherwise.
func PlanPage(base *url.URL, body io.Reader) Plan {
	forms, refresh := scanPage(body)
	candidates := 0
	var pick *form
	for _, f := range forms {
		if f.userInput {
			return Plan{Kind: PlanNeedsUser}
		}
		candidates++
		pick = f
	}
	switch {
	case candidates > 1:
		return Plan{Kind: PlanNeedsUser}
	case candidates == 1:
		return submitPlan(base, pick)
	case refresh != "":
		if target, ok := resolve(base, refresh); ok {
			return Plan{Kind: PlanFollow, Method: http.MethodGet, Target: target}
		}
	}
	return Plan{Kind: PlanNone}
}

func submitPlan(base *url.URL, f *form) Plan {
	target, ok := resolve(base, f.action)
	if !ok {
		return Plan{Kind: PlanNone}
	}
	fields := url.Values{}
	for k, v := range f.fields {
		fields[k] = append([]string(nil), v...)
	}
	if f.submit != nil {
		fields.Add(f.submit[0], f.submit[1])
	}
	method := http.MethodGet
	if strings.EqualFold(f.method, http.MethodPost) {
		method = http.MethodPost
	}
	return Plan{Kind: PlanSubmit, Method: method, Target: target, Fields: fields}
}

// resolve accepts only http(s) targets; an empty action is the page itself.
func resolve(base *url.URL, ref string) (*url.URL, bool) {
	u, err := base.Parse(strings.TrimSpace(ref))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" {
		return nil, false
	}
	u.Fragment = ""
	return u, true
}

func scanPage(body io.Reader) (forms []*form, refresh string) {
	z := html.NewTokenizer(body)
	var cur *form
	for {
		tt := z.Next()
		switch tt {
		case html.ErrorToken:
			return forms, refresh
		case html.EndTagToken:
			if name, _ := z.TagName(); string(name) == "form" {
				cur = nil
			}
		case html.StartTagToken, html.SelfClosingTagToken:
			name, _ := z.TagName()
			attrs := readAttrs(z)
			switch string(name) {
			case "meta":
				if refresh == "" && strings.EqualFold(attrs["http-equiv"], "refresh") {
					refresh = refreshURL(attrs["content"])
				}
			case "form":
				cur = &form{method: attrs["method"], action: attrs["action"], fields: url.Values{}, radios: map[string]bool{}}
				forms = append(forms, cur)
			case "input", "button", "select", "textarea":
				if cur != nil {
					cur.control(string(name), attrs)
				}
			}
		}
	}
}

func readAttrs(z *html.Tokenizer) map[string]string {
	attrs := map[string]string{}
	for {
		k, v, more := z.TagAttr()
		if len(k) > 0 {
			attrs[strings.ToLower(string(k))] = string(v)
		}
		if !more {
			return attrs
		}
	}
}

// control adds one form control. Contents of select and textarea are not read: a
// select is left out (portals use it for the language), a named textarea counts as
// user input.
func (f *form) control(tag string, a map[string]string) {
	name, value := a["name"], a["value"]
	_, disabled := a["disabled"]
	if disabled {
		return
	}
	kind := strings.ToLower(a["type"])
	switch {
	case tag == "select":
	case tag == "textarea":
		if name != "" {
			f.userInput = true
		}
	case tag == "button" || kind == "submit" || kind == "image":
		if tag == "button" && kind != "" && kind != "submit" {
			return
		}
		if !f.sawSubmit && name != "" {
			f.submit = &[2]string{name, value}
		}
		f.sawSubmit = true
	case kind == "hidden":
		if name != "" {
			f.fields.Add(name, value)
		}
	case kind == "checkbox":
		if name != "" {
			f.fields.Add(name, valueOr(value, "on"))
		}
	case kind == "radio":
		_, checked := a["checked"]
		taken, seen := f.radios[name]
		if name != "" && !taken && (checked || !seen) {
			f.fields.Set(name, valueOr(value, "on"))
			f.radios[name] = checked
		}
	case kind == "reset" || kind == "file":
	default: // text, password, email, tel, number, search, no type: visible input
		if value == "" {
			f.userInput = true
		} else if name != "" {
			f.fields.Add(name, value)
		}
	}
}

func valueOr(v, fallback string) string {
	if v == "" {
		return fallback
	}
	return v
}

// refreshURL extracts the target of `content="0; url=/next"`.
func refreshURL(content string) string {
	_, rest, ok := strings.Cut(content, ";")
	if !ok {
		return ""
	}
	rest = strings.TrimSpace(rest)
	if len(rest) < 4 || !strings.EqualFold(rest[:4], "url=") {
		return ""
	}
	return strings.Trim(strings.TrimSpace(rest[4:]), `'"`)
}
