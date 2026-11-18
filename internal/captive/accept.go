package captive

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/blackdark-org/travo-greenfield/internal/connectivity"
)

// Walker bounds (invariants §12, ADR 0023).
const (
	AcceptBudget    = 30 * time.Second
	acceptSteps     = 5
	acceptRedirects = 5
	acceptRequest   = 5 * time.Second
	acceptBodyCap   = 256 << 10
)

var (
	ErrAcceptNotCaptive = errors.New("the probe does not report a captive portal")
	ErrAcceptBusy       = errors.New("an automatic login is already running")
)

// AcceptOutcome is how a walk ended; every outcome is an expected value, not an error.
type AcceptOutcome string

const (
	AcceptOnline    AcceptOutcome = "online"
	AcceptNeedsUser AcceptOutcome = "needs_user"
	AcceptNoForm    AcceptOutcome = "no_form"
	AcceptFailed    AcceptOutcome = "failed"
)

// AcceptStep is one request of the walk. Host only: paths and queries carry portal tokens.
type AcceptStep struct {
	Kind   string `json:"kind" enum:"fetch,renew,follow,submit"`
	Host   string `json:"host" doc:"Host of the request; empty for renew"`
	Status int    `json:"status" doc:"HTTP status; 0 for a transport error or a renew"`
}

type AcceptResult struct {
	Outcome AcceptOutcome `json:"outcome" enum:"online,needs_user,no_form,failed"`
	Steps   []AcceptStep  `json:"steps"`
	Renewed bool          `json:"renewed" doc:"Step 0 renewed the wwan DHCP lease"`
}

// LeaseRenew is the guarded wwan renew (Renewer); only step 0 uses it.
type LeaseRenew interface {
	Availability(context.Context, connectivity.Status) RenewState
	Renew(context.Context) error
}

// Acceptor walks click-through portal pages for the user. It changes no router state
// except the guarded step-0 renew, and sends only fields the portal page supplied.
type Acceptor struct {
	conn      Connectivity
	renew     LeaseRenew // nil: no renew (no wwan support)
	newClient func() *http.Client
	inflight  sync.Mutex
}

func NewAcceptor(conn Connectivity, renew LeaseRenew, newClient func() *http.Client) *Acceptor {
	return &Acceptor{conn: conn, renew: renew, newClient: newClient}
}

// NewPortalClient is the production HTTP edge: a fresh cookie jar per walk, bounded
// redirects and requests, verified TLS, no proxy, and never a loopback, link-local or
// unspecified address (a portal page must not reach services on the router itself).
func NewPortalClient() *http.Client {
	jar, _ := cookiejar.New(nil) // never fails without options
	dialer := &net.Dialer{Timeout: acceptRequest, Control: refuseLocal}
	transport := &http.Transport{DialContext: dialer.DialContext, TLSHandshakeTimeout: acceptRequest,
		ResponseHeaderTimeout: acceptRequest, DisableKeepAlives: true}
	return &http.Client{Jar: jar, Transport: transport, Timeout: acceptRequest, CheckRedirect: limitRedirects}
}

func limitRedirects(_ *http.Request, via []*http.Request) error {
	if len(via) >= acceptRedirects {
		return http.ErrUseLastResponse
	}
	return nil
}

var errLocalTarget = errors.New("portal target is a local address")

func refuseLocal(_, address string, _ syscall.RawConn) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return err
	}
	ip := net.ParseIP(host)
	if ip == nil || ip.IsLoopback() || ip.IsUnspecified() || ip.IsLinkLocalUnicast() || ip.IsMulticast() {
		return errLocalTarget
	}
	return nil
}

// Accept walks the portal the shared probe reports. Step 0 fetches the portal URL; only
// it may renew the wwan lease, and only after a transport failure and when the renew is
// available (a kept guard reads blocked and is skipped). Later steps follow a meta
// refresh or submit the page's only form, re-probing after each; online ends the walk.
func (a *Acceptor) Accept(ctx context.Context) (AcceptResult, error) {
	if !a.inflight.TryLock() {
		return AcceptResult{}, ErrAcceptBusy
	}
	defer a.inflight.Unlock()
	ctx, cancel := context.WithTimeout(ctx, AcceptBudget)
	defer cancel()
	st := a.conn.Status(ctx)
	if st.Internet != connectivity.Captive || st.PortalURL == nil {
		return AcceptResult{}, ErrAcceptNotCaptive
	}
	start, ok := resolve(&url.URL{}, *st.PortalURL)
	if !ok {
		return AcceptResult{}, ErrAcceptNotCaptive
	}
	w := walk{client: a.newClient(), result: AcceptResult{Steps: []AcceptStep{}}}
	page, err := w.do(ctx, "fetch", Plan{Method: http.MethodGet, Target: start})
	if err != nil && a.renewStep0(ctx, &w, st) {
		page, err = w.do(ctx, "fetch", Plan{Method: http.MethodGet, Target: start})
	}
	if err != nil {
		return w.end(AcceptFailed), nil
	}
	for range acceptSteps {
		plan := PlanPage(page.url, strings.NewReader(page.body))
		switch plan.Kind {
		case PlanNeedsUser:
			return w.end(AcceptNeedsUser), nil
		case PlanNone:
			return w.end(AcceptNoForm), nil
		}
		if page, err = w.do(ctx, string(plan.Kind), plan); err != nil {
			return w.end(AcceptFailed), nil
		}
		a.conn.Invalidate()
		if a.conn.Status(ctx).Internet == connectivity.Online {
			return w.end(AcceptOnline), nil
		}
	}
	return w.end(AcceptFailed), nil
}

// renewStep0 is the walk's only renew (invariants §12): the Renewer itself proves wwan
// is the kernel default route, holds the guard and bounds its wait.
func (a *Acceptor) renewStep0(ctx context.Context, w *walk, st connectivity.Status) bool {
	if a.renew == nil || a.renew.Availability(ctx, st) != RenewAvailable {
		return false
	}
	w.result.Steps = append(w.result.Steps, AcceptStep{Kind: "renew"})
	if a.renew.Renew(ctx) != nil {
		return false
	}
	w.result.Renewed = true
	return true
}

type page struct {
	url  *url.URL
	body string
}

type walk struct {
	client *http.Client
	result AcceptResult
}

func (w *walk) end(o AcceptOutcome) AcceptResult {
	w.result.Outcome = o
	return w.result
}

// do sends one plan and reads at most acceptBodyCap of the reply; a non-2xx final
// status is a failure (redirects were already followed within the bound).
func (w *walk) do(ctx context.Context, kind string, p Plan) (page, error) {
	step := AcceptStep{Kind: kind, Host: p.Target.Host}
	defer func() { w.result.Steps = append(w.result.Steps, step) }()
	req, err := newPlanRequest(ctx, p)
	if err != nil {
		return page{}, err
	}
	res, err := w.client.Do(req)
	if err != nil {
		return page{}, err
	}
	defer res.Body.Close()
	step.Status = res.StatusCode
	body, err := io.ReadAll(io.LimitReader(res.Body, acceptBodyCap))
	if err != nil {
		return page{}, err
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return page{}, errors.New(res.Status)
	}
	return page{url: res.Request.URL, body: string(body)}, nil
}

func newPlanRequest(ctx context.Context, p Plan) (*http.Request, error) {
	target := *p.Target
	if p.Method != http.MethodPost {
		if len(p.Fields) > 0 {
			target.RawQuery = p.Fields.Encode()
		}
		return http.NewRequestWithContext(ctx, http.MethodGet, target.String(), nil)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target.String(), strings.NewReader(p.Fields.Encode()))
	if err == nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	return req, err
}
