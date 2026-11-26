package tailscale

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"strings"
	"sync"
)

// Mock is an in-memory tailscaled for mock mode and tests. A login completes on the
// second status read after `up`, as if the user had opened the auth URL meanwhile.
type Mock struct {
	mu       sync.Mutex
	prefs    rawPrefs
	loggedIn bool
	authURL  string
	reads    int
	// Calls records every write as "up ..."/"set ..."/"logout" for tests.
	Calls [][]string
}

// mockPeers are documentation-range tailnet peers (RFC 6598 addresses, example names).
var mockPeers = map[string]*rawPeer{
	"nodekey:mock1": {ID: "nMock1CNTRL", HostName: "exit-home", DNSName: "exit-home.tail0000.ts.net.", TailscaleIPs: []string{"100.64.0.2"}, Online: true, ExitNodeOption: true},
	"nodekey:mock2": {ID: "nMock2CNTRL", HostName: "laptop", DNSName: "laptop.tail0000.ts.net.", TailscaleIPs: []string{"100.64.0.3"}, Online: false},
}

func NewMock() *Mock {
	return &Mock{prefs: rawPrefs{CorpDNS: true, LoggedOut: true}}
}

func (m *Mock) Status(context.Context) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.authURL != "" {
		m.reads++
		if m.reads >= 2 {
			m.loggedIn, m.authURL = true, ""
		}
	}
	st := rawStatus{Version: "1.104.1-1 (mock)", BackendState: StateNeedsLogin, AuthURL: m.authURL,
		Self: &rawPeer{HostName: "travo"}, Health: []string{}}
	if m.loggedIn && m.prefs.WantRunning {
		st.BackendState = StateRunning
		st.Self = &rawPeer{ID: "nSelfCNTRL", HostName: "travo", DNSName: "travo.tail0000.ts.net.", TailscaleIPs: []string{"100.64.0.1"}, Online: true}
		st.CurrentTailnet = &rawTailnet{Name: "example.com", MagicDNSSuffix: "tail0000.ts.net", MagicDNSEnabled: true}
		st.Peer = map[string]*rawPeer{}
		for k, p := range mockPeers {
			c := *p
			c.ExitNode = c.ExitNodeOption && len(c.TailscaleIPs) > 0 && c.TailscaleIPs[0] == m.prefs.ExitNodeIP
			st.Peer[k] = &c
		}
	} else if m.loggedIn {
		st.BackendState = StateStopped
	}
	return json.Marshal(st)
}

func (m *Mock) Prefs(context.Context) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return json.Marshal(m.prefs)
}

func (m *Mock) Up(_ context.Context, args []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Calls = append(m.Calls, append([]string{"up"}, args...))
	if err := m.apply(args, true); err != nil {
		return err
	}
	m.prefs.WantRunning, m.prefs.LoggedOut = true, false
	if !m.loggedIn {
		m.authURL, m.reads = "https://login.tailscale.com/a/mock", 0
		return ErrUpTimeout
	}
	return nil
}

func (m *Mock) Set(_ context.Context, args []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Calls = append(m.Calls, append([]string{"set"}, args...))
	return m.apply(args, false)
}

func (m *Mock) Logout(context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Calls = append(m.Calls, []string{"logout"})
	m.loggedIn, m.authURL = false, ""
	m.prefs.LoggedOut, m.prefs.WantRunning = true, false
	return nil
}

// apply parses the flags Travo passes; unknown flags fail like the real CLI.
func (m *Mock) apply(args []string, up bool) error {
	fs := flag.NewFlagSet("tailscale", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	reset := fs.Bool("reset", false, "")
	acceptDNS := fs.Bool("accept-dns", true, "")
	exit := fs.String("exit-node", "", "")
	lan := fs.Bool("exit-node-allow-lan-access", false, "")
	routes := fs.String("advertise-routes", "", "")
	ssh := fs.Bool("ssh", false, "")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return errors.New("unexpected arguments")
	}
	if up && !*reset {
		return errors.New("mock: Travo always passes --reset to up")
	}
	seen := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { seen[f.Name] = true })
	if seen["accept-dns"] {
		m.prefs.CorpDNS = *acceptDNS
	}
	if seen["exit-node"] {
		m.prefs.ExitNodeIP, m.prefs.ExitNodeID = *exit, ""
	}
	if seen["exit-node-allow-lan-access"] {
		m.prefs.ExitNodeAllowLANAccess = *lan
	}
	if seen["advertise-routes"] {
		m.prefs.AdvertiseRoutes = nil
		if *routes != "" {
			m.prefs.AdvertiseRoutes = strings.Split(*routes, ",")
		}
	}
	if seen["ssh"] {
		m.prefs.RunSSH = *ssh
	}
	return nil
}

var _ CLI = (*Mock)(nil)
