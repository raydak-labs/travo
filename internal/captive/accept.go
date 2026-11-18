package captive

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"sync"
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

// PortalEdge is the walk's I/O: the router's interfaces and routes, read at walk time,
// and the HTTP client built on the resulting TargetPolicy.
type PortalEdge struct {
	Interfaces  func() ([]IfaceAddrs, error)
	RouteSource func(netip.Addr) (netip.Addr, error)
	NewClient   func(TargetPolicy) *http.Client
}

// KernelPortalEdge is the production edge (the daemon runs on the router).
func KernelPortalEdge() PortalEdge {
	return PortalEdge{Interfaces: KernelInterfaces, RouteSource: KernelRouteSource, NewClient: NewPortalClient}
}

// Acceptor walks click-through portal pages for the user. It changes no router state
// except the guarded step-0 renew, and sends only fields the portal page supplied.
type Acceptor struct {
	conn     Connectivity
	renew    LeaseRenew // nil: no renew (no wwan support)
	edge     PortalEdge
	inflight sync.Mutex
}

func NewAcceptor(conn Connectivity, renew LeaseRenew, edge PortalEdge) *Acceptor {
	return &Acceptor{conn: conn, renew: renew, edge: edge}
}

func limitRedirects(_ *http.Request, via []*http.Request) error {
	if len(via) >= acceptRedirects {
		return http.ErrUseLastResponse
	}
	return nil
}

var errPortalStatus = errors.New("portal answered with a non-2xx status")

// client builds a fresh client whose policy allows only targets routed out of uplink.
func (a *Acceptor) client(uplink string) (*http.Client, error) {
	ifaces, err := a.edge.Interfaces()
	if err != nil {
		return nil, fmt.Errorf("captive: interfaces: %w", err)
	}
	return a.edge.NewClient(NewTargetPolicy(ifaces, uplink, a.edge.RouteSource)), nil
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
	if st.Internet != connectivity.Captive || st.PortalURL == nil || st.ActiveInterface == nil {
		return AcceptResult{}, ErrAcceptNotCaptive
	}
	start, ok := resolve(&url.URL{}, *st.PortalURL)
	if !ok {
		return AcceptResult{}, ErrAcceptNotCaptive
	}
	client, err := a.client(*st.ActiveInterface)
	if err != nil {
		return AcceptResult{}, err
	}
	w := walk{client: client, result: AcceptResult{Steps: []AcceptStep{}}}
	page, err := w.do(ctx, "fetch", Plan{Method: http.MethodGet, Target: start})
	// Only a transport failure hints at a stale lease; a portal that answered does not.
	// A refused target is no transport failure either.
	if err != nil && !errors.Is(err, errPortalStatus) && !errors.Is(err, ErrTargetRefused) && a.renewStep0(ctx, &w, st) {
		// The renew may change the uplink address: rebuild the policy from the kernel.
		if w.client, err = a.client(*st.ActiveInterface); err != nil {
			return AcceptResult{}, err
		}
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
		return page{}, fmt.Errorf("%w: %s", errPortalStatus, res.Status)
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
