package services

import (
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"
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

// captiveBounceRunner is a scripted CommandRunner for the wwan DHCP bounce.
type captiveBounceRunner struct {
	dump      string
	dumpErr   error
	status    string
	statusErr error
	ifdownErr error
	ifupErr   error
	calls     []string
}

func (r *captiveBounceRunner) Run(name string, args ...string) ([]byte, error) {
	call := name
	for _, a := range args {
		call += " " + a
	}
	r.calls = append(r.calls, call)
	switch call {
	case "/sbin/ubus -S call network.interface dump":
		return []byte(r.dump), r.dumpErr
	case "/sbin/ubus call network.interface.wwan status":
		return []byte(r.status), r.statusErr
	case "/sbin/ifdown wwan":
		return nil, r.ifdownErr
	case "/sbin/ifup wwan":
		return nil, r.ifupErr
	}
	return nil, nil
}

func (r *captiveBounceRunner) called(prefix string) bool {
	for _, c := range r.calls {
		if c == prefix {
			return true
		}
	}
	return false
}

const wwanActiveDump = `{"interface":[
  {"interface":"lan","up":true,"route":[{"target":"0.0.0.0","mask":255}]},
  {"interface":"wwan","up":true,"route":[{"target":"0.0.0.0","mask":0}]}
]}`

const wwanUpNoDefaultRouteDump = `{"interface":[
  {"interface":"wan","up":true,"route":[{"target":"0.0.0.0","mask":0}]},
  {"interface":"wwan","up":true,"route":[{"target":"10.0.0.0","mask":8}]}
]}`

const wwanLeaseStatus = `{"up":true,"ipv4-address":[{"address":"10.0.0.50","mask":24}]}`

// useTempBounceGuard points the wwan bounce crash guard at a temp dir and
// shrinks the production sleeps so the bounce tests stay fast.
func useTempBounceGuard(t *testing.T) string {
	t.Helper()
	origPath, origDown, origUp, origPoll := captiveWwanBounceGuardPath, captiveWwanDownSettle, captiveWwanUpSettle, captiveWwanPollInterval
	path := filepath.Join(t.TempDir(), "captive-wwan-bounce-in-progress")
	captiveWwanBounceGuardPath = path
	captiveWwanDownSettle = time.Millisecond
	captiveWwanUpSettle = 20 * time.Millisecond
	captiveWwanPollInterval = time.Millisecond
	t.Cleanup(func() {
		captiveWwanBounceGuardPath = origPath
		captiveWwanDownSettle, captiveWwanUpSettle, captiveWwanPollInterval = origDown, origUp, origPoll
	})
	return path
}

func newBounceCaptiveService(r CommandRunner) *CaptiveService {
	return &CaptiveService{prober: &MockHTTPProber{}, cmd: r, guardFile: captiveDNSGuardFile}
}

func TestUbusInterfaceDumpHasDefaultRoute(t *testing.T) {
	t.Parallel()

	if !ubusInterfaceDumpHasDefaultRoute([]byte(wwanActiveDump), "wwan") {
		t.Error("expected wwan to be the active uplink (up with a default route)")
	}
	if ubusInterfaceDumpHasDefaultRoute([]byte(wwanUpNoDefaultRouteDump), "wwan") {
		t.Error("wwan without a default route must not count as the active uplink")
	}
	if ubusInterfaceDumpHasDefaultRoute([]byte(wwanActiveDump), "usb0") {
		t.Error("unknown interface must not count as the active uplink")
	}
	if ubusInterfaceDumpHasDefaultRoute([]byte("not json"), "wwan") {
		t.Error("unparsable dump must not count as the active uplink")
	}
}

func TestWwanIsActiveUplinkPropagatesDumpError(t *testing.T) {
	t.Parallel()

	runner := &captiveBounceRunner{dumpErr: errors.New("ubus down")}
	svc := newBounceCaptiveService(runner)
	if _, err := svc.wwanIsActiveUplink(); err == nil {
		t.Error("expected an error when the interface dump fails")
	}
}

func TestBounceActiveWwanDHCPBouncesActiveWwan(t *testing.T) {
	guard := useTempBounceGuard(t)
	runner := &captiveBounceRunner{dump: wwanActiveDump, status: wwanLeaseStatus}
	svc := newBounceCaptiveService(runner)

	if err := svc.bounceActiveWwanDHCP(); err != nil {
		t.Fatalf("bounceActiveWwanDHCP: %v", err)
	}
	if !runner.called("/sbin/ifdown wwan") {
		t.Errorf("expected ifdown wwan, calls = %v", runner.calls)
	}
	if !runner.called("/sbin/ifup wwan") {
		t.Errorf("expected ifup wwan, calls = %v", runner.calls)
	}
	if _, err := os.Stat(guard); !os.IsNotExist(err) {
		t.Errorf("expected crash guard removed after a successful bounce, stat err = %v", err)
	}
}

func TestBounceActiveWwanDHCPSkipsInactiveWwan(t *testing.T) {
	guard := useTempBounceGuard(t)
	runner := &captiveBounceRunner{dump: wwanUpNoDefaultRouteDump, status: wwanLeaseStatus}
	svc := newBounceCaptiveService(runner)

	if err := svc.bounceActiveWwanDHCP(); err != nil {
		t.Fatalf("bounceActiveWwanDHCP: %v", err)
	}
	if runner.called("/sbin/ifdown wwan") || runner.called("/sbin/ifup wwan") {
		t.Errorf("must not bounce wwan when it is not the active uplink, calls = %v", runner.calls)
	}
	if _, err := os.Stat(guard); !os.IsNotExist(err) {
		t.Errorf("no bounce means no guard, stat err = %v", err)
	}
}

func TestBounceActiveWwanDHCPKeepsGuardOnIfupFailure(t *testing.T) {
	guard := useTempBounceGuard(t)
	runner := &captiveBounceRunner{dump: wwanActiveDump, status: wwanLeaseStatus, ifupErr: errors.New("ifup failed")}
	svc := newBounceCaptiveService(runner)

	if err := svc.bounceActiveWwanDHCP(); err == nil {
		t.Fatal("expected an error when ifup wwan fails")
	}
	if _, err := os.Stat(guard); err != nil {
		t.Errorf("crash guard must remain when the bounce did not complete: %v", err)
	}
}

func TestBounceActiveWwanDHCPKeepsGuardOnIfdownFailure(t *testing.T) {
	guard := useTempBounceGuard(t)
	runner := &captiveBounceRunner{dump: wwanActiveDump, status: wwanLeaseStatus, ifdownErr: errors.New("ifdown failed")}
	svc := newBounceCaptiveService(runner)

	if err := svc.bounceActiveWwanDHCP(); err == nil {
		t.Fatal("expected an error when ifdown wwan fails")
	}
	if runner.called("/sbin/ifup wwan") {
		t.Error("must not run ifup after a failed ifdown")
	}
	if _, err := os.Stat(guard); err != nil {
		t.Errorf("crash guard must remain when the bounce did not complete: %v", err)
	}
}

func TestBounceActiveWwanDHCPKeepsGuardWhenLeaseMissing(t *testing.T) {
	guard := useTempBounceGuard(t)
	runner := &captiveBounceRunner{dump: wwanActiveDump, status: `{"up":false}`}
	svc := newBounceCaptiveService(runner)

	if err := svc.bounceActiveWwanDHCP(); err == nil {
		t.Fatal("expected an error when wwan never gets a lease")
	}
	if _, err := os.Stat(guard); err != nil {
		t.Errorf("crash guard must remain when the bounce did not complete: %v", err)
	}
}

func TestBounceActiveWwanDHCPDoesNotBounceWhenUplinkUnknown(t *testing.T) {
	_ = useTempBounceGuard(t)
	runner := &captiveBounceRunner{dumpErr: errors.New("ubus down"), status: wwanLeaseStatus}
	svc := newBounceCaptiveService(runner)

	if err := svc.bounceActiveWwanDHCP(); err == nil {
		t.Fatal("expected an error when the active uplink cannot be determined")
	}
	if runner.called("/sbin/ifdown wwan") {
		t.Error("must not bounce wwan when the active uplink is unknown")
	}
}
