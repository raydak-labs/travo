package captive

import (
	"net/url"
	"strings"
	"testing"
)

// Portal pages here are the shapes scripts/portal-sim.py serves, not device payloads.
// Replace or extend them with verbatim hotel captures once the joint session has some.
func TestPlanPage(t *testing.T) {
	base, _ := url.Parse("http://portal.hotel.test/login?x=1")
	for _, tc := range []struct {
		name, html string
		kind       PlanKind
		method     string
		target     string
		fields     string
	}{
		{"terms form", `<form method="post" action="/accept"><input type="hidden" name="token" value="sim-1">
			<label><input type="checkbox" name="terms" value="yes"> accept</label>
			<button type="submit" name="go" value="connect">Connect</button></form>`,
			PlanSubmit, "POST", "http://portal.hotel.test/accept", "go=connect&terms=yes&token=sim-1"},
		{"get form, empty action, unnamed submit", `<FORM><INPUT TYPE=checkbox NAME=ok><input type=submit value=Go></FORM>`,
			PlanSubmit, "GET", "http://portal.hotel.test/login?x=1", "ok=on"},
		{"prefilled text and radios", `<form action="https://auth.example.net/c"><input name="lang" value="en">
			<input type=radio name=plan value=free><input type=radio name=plan value=paid checked><input type=radio name=plan value=x>
			<select name="country"><option>de</option></select><input type=text name=gone disabled></form>`,
			PlanSubmit, "GET", "https://auth.example.net/c", "lang=en&plan=paid"},
		{"room number", `<form method=post><input type=hidden name=t value=1><input type="text" name="room" required></form>`,
			PlanNeedsUser, "", "", ""},
		{"password", `<form><input type=password name=pw></form>`, PlanNeedsUser, "", "", ""},
		{"textarea", `<form><textarea name=note></textarea></form>`, PlanNeedsUser, "", "", ""},
		{"two forms", `<form><input type=hidden name=a></form><form><input type=hidden name=b></form>`, PlanNeedsUser, "", "", ""},
		{"meta refresh", `<html><head><meta http-equiv="Refresh" content="0; URL='/terms'"></head></html>`,
			PlanFollow, "GET", "http://portal.hotel.test/terms", ""},
		{"javascript action", `<form action="javascript:go()"><input type=hidden name=a></form>`, PlanNone, "", "", ""},
		{"meta without url", `<meta http-equiv="refresh" content="5">`, PlanNone, "", "", ""},
		{"plain page", `<h1>Welcome</h1><a href="/go">Continue</a>`, PlanNone, "", "", ""},
		{"empty", ``, PlanNone, "", "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := PlanPage(base, strings.NewReader(tc.html))
			if got.Kind != tc.kind {
				t.Fatalf("kind %s, want %s", got.Kind, tc.kind)
			}
			if tc.target == "" {
				return
			}
			if got.Method != tc.method || got.Target.String() != tc.target || got.Fields.Encode() != tc.fields {
				t.Fatalf("got %s %s %q", got.Method, got.Target, got.Fields.Encode())
			}
		})
	}
}
