package services

import (
	"net/url"
	"testing"
)

func TestExtractCaptiveAcceptTargets(t *testing.T) {
	base, err := url.Parse("http://portal.example/welcome")
	if err != nil {
		t.Fatal(err)
	}

	html := `<html><body>
<a href="/ok">Accept Terms</a>
<a href="http://portal.example/continue">Continue</a>
<a href="/skip">Random</a>
</body></html>`

	got := extractCaptiveAcceptTargets(html, base)
	if len(got) != 2 {
		t.Fatalf("expected 2 targets, got %d: %v", len(got), got)
	}
	if got[0] != "http://portal.example/ok" {
		t.Fatalf("unexpected first target: %q", got[0])
	}
	if got[1] != "http://portal.example/continue" {
		t.Fatalf("unexpected second target: %q", got[1])
	}
}

func TestExtractJSRedirect(t *testing.T) {
	base, _ := url.Parse("http://gw.example/")

	tests := []struct {
		name string
		html string
		want string
	}{
		{
			name: "window.location.href",
			html: `<script>window.location.href = "http://portal.example/login";</script>`,
			want: "http://portal.example/login",
		},
		{
			name: "location.href with JS escapes",
			html: `<script>location.href = "http:\/\/portal.example\/guest\/login.php";</script>`,
			want: "http://portal.example/guest/login.php",
		},
		{
			name: "meta refresh",
			html: `<meta http-equiv="refresh" content="0;url=http://portal.example/splash">`,
			want: "http://portal.example/splash",
		},
		{
			name: "relative path",
			html: `<script>window.location = "/guest/login";</script>`,
			want: "http://gw.example/guest/login",
		},
		{
			name: "no redirect",
			html: `<html><body>Hello</body></html>`,
			want: "",
		},
		{
			name: "javascript: href ignored",
			html: `<script>window.location.href = "javascript:void(0)";</script>`,
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractJSRedirect(tt.html, base)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestExtractAllForms(t *testing.T) {
	base, _ := url.Parse("http://gw.example/login")

	html := `<form action="/submit" method="POST">
<input type="hidden" name="token" value="abc123">
<input type="text" name="user" value="">
<input type="checkbox" name="agree" value="yes">
</form>`

	forms := extractAllForms(html, base)
	if len(forms) != 1 {
		t.Fatalf("expected 1 form, got %d", len(forms))
	}
	f := forms[0]
	if f.Action != "http://gw.example/submit" {
		t.Errorf("action = %q, want http://gw.example/submit", f.Action)
	}
	if f.Method != "POST" {
		t.Errorf("method = %q, want POST", f.Method)
	}
	if f.Values.Get("token") != "abc123" {
		t.Errorf("token = %q, want abc123", f.Values.Get("token"))
	}
	if f.Values.Get("agree") != "yes" {
		t.Errorf("agree = %q, want yes", f.Values.Get("agree"))
	}
}

func TestExtractCaptiveForms_CTAButton(t *testing.T) {
	base, _ := url.Parse("http://portal.example/")

	html := `<form action="/accept" method="POST">
<input type="hidden" name="session" value="xyz">
<button>Accept and Continue</button>
</form>`

	forms := extractCaptiveForms(html, base)
	if len(forms) != 1 {
		t.Fatalf("expected 1 CTA form, got %d", len(forms))
	}
	if forms[0].Action != "http://portal.example/accept" {
		t.Errorf("action = %q", forms[0].Action)
	}
}

func TestExtractCaptiveForms_NoCTA(t *testing.T) {
	base, _ := url.Parse("http://portal.example/")

	html := `<form action="/search" method="GET">
<input type="text" name="q" value="">
<button>Search</button>
</form>`

	forms := extractCaptiveForms(html, base)
	if len(forms) != 0 {
		t.Fatalf("expected 0 CTA forms for search form, got %d", len(forms))
	}
}

func TestParseFormInputs_CheckboxAndRadio(t *testing.T) {
	formBody := `
<input type="checkbox" name="terms">
<input type="radio" name="plan" value="free">
<input type="radio" name="plan" value="premium">
<input type="hidden" name="lang" value="en">
`
	vals := parseFormInputs(formBody)
	if vals.Get("terms") != "on" {
		t.Errorf("terms = %q, want on", vals.Get("terms"))
	}
	if vals.Get("plan") != "free" {
		t.Errorf("plan = %q, want free (first radio)", vals.Get("plan"))
	}
	if vals.Get("lang") != "en" {
		t.Errorf("lang = %q, want en", vals.Get("lang"))
	}
}

func TestAutoSubmitRegex(t *testing.T) {
	tests := []struct {
		js   string
		want bool
	}{
		{"document.forms[0].submit()", true},
		{"document.sendin.submit()", true},
		{"document.loginForm.submit()", true},
		{"some_other_function()", false},
		{"submit()", false},
	}
	for _, tt := range tests {
		got := captiveAutoSubmitRe.MatchString(tt.js)
		if got != tt.want {
			t.Errorf("autoSubmit(%q) = %v, want %v", tt.js, got, tt.want)
		}
	}
}

func TestStripTags(t *testing.T) {
	got := stripTags(`<b>Accept</b> <a href="/">Terms</a>`)
	if got != "Accept Terms" {
		t.Errorf("stripTags = %q, want 'Accept Terms'", got)
	}
}

func TestResolveCaptiveURL(t *testing.T) {
	base, _ := url.Parse("http://portal.example/page")

	tests := []struct {
		href    string
		want    string
		wantErr bool
	}{
		{"/login", "http://portal.example/login", false},
		{"http://other.example/ok", "http://other.example/ok", false},
		{"#anchor", "", true},
		{"javascript:void(0)", "", true},
		{"", "", true},
		{"ftp://bad.example", "", true},
	}
	for _, tt := range tests {
		got, err := resolveCaptiveURL(base, tt.href)
		if tt.wantErr && err == nil {
			t.Errorf("resolveCaptiveURL(%q) expected error", tt.href)
		}
		if !tt.wantErr && got != tt.want {
			t.Errorf("resolveCaptiveURL(%q) = %q, want %q", tt.href, got, tt.want)
		}
	}
}
