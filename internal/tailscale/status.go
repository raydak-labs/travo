package tailscale

import (
	"encoding/json"
	"fmt"
	"net/netip"
	"slices"
	"strings"
)

// Backend states tailscaled reports in `BackendState` (ipn.State names).
const (
	StateNeedsLogin = "NeedsLogin"
	StateRunning    = "Running"
	StateStopped    = "Stopped"
	StateNoState    = "NoState"
)

// rawPeer is the slice of ipnstate.PeerStatus Travo reads. `Self` and every `Peer` entry share
// it (testdata/device/axt1800-25.12.6/tailscale/status-needslogin.json).
type rawPeer struct {
	ID             string   `json:"ID"`
	HostName       string   `json:"HostName"`
	DNSName        string   `json:"DNSName"`
	TailscaleIPs   []string `json:"TailscaleIPs"`
	Online         bool     `json:"Online"`
	ExitNode       bool     `json:"ExitNode"`
	ExitNodeOption bool     `json:"ExitNodeOption"`
}

type rawTailnet struct {
	Name            string `json:"Name"`
	MagicDNSSuffix  string `json:"MagicDNSSuffix"`
	MagicDNSEnabled bool   `json:"MagicDNSEnabled"`
}

// rawStatus is `tailscale status --json`.
type rawStatus struct {
	Version        string              `json:"Version"`
	BackendState   string              `json:"BackendState"`
	AuthURL        string              `json:"AuthURL"`
	Self           *rawPeer            `json:"Self"`
	Health         []string            `json:"Health"`
	MagicDNSSuffix string              `json:"MagicDNSSuffix"`
	CurrentTailnet *rawTailnet         `json:"CurrentTailnet"`
	Peer           map[string]*rawPeer `json:"Peer"`
}

// rawPrefs is `tailscale debug prefs` (testdata/device/axt1800-25.12.6/tailscale/prefs-default.json).
type rawPrefs struct {
	ExitNodeID             string   `json:"ExitNodeID"`
	ExitNodeIP             string   `json:"ExitNodeIP"`
	ExitNodeAllowLANAccess bool     `json:"ExitNodeAllowLANAccess"`
	CorpDNS                bool     `json:"CorpDNS"`
	RunSSH                 bool     `json:"RunSSH"`
	WantRunning            bool     `json:"WantRunning"`
	LoggedOut              bool     `json:"LoggedOut"`
	AdvertiseRoutes        []string `json:"AdvertiseRoutes"`
}

// Node is one tailnet machine.
type Node struct {
	ID       string   `json:"id" doc:"Stable node ID; empty before login"`
	HostName string   `json:"host_name"`
	DNSName  string   `json:"dns_name" doc:"MagicDNS name with trailing dot; empty before login"`
	IPs      []string `json:"ips"`
	Online   bool     `json:"online"`
}

// ExitNode is a peer that offers itself as exit node.
type ExitNode struct {
	Node
	Active bool `json:"active" doc:"This router routes through this exit node now"`
}

// MagicDNS is the tailnet's DNS name suffix; Travo serves it through the `tailscale` DNS layer.
type MagicDNS struct {
	Enabled bool   `json:"enabled"`
	Suffix  string `json:"suffix" doc:"Tailnet DNS suffix without trailing dot, e.g. tail1234.ts.net"`
}

// Prefs are tailscaled's stored preferences Travo manages.
type Prefs struct {
	// ExitNode is the stable ID of the selected exit node, or its IP while unresolved.
	ExitNode        string   `json:"exit_node"`
	AdvertiseRoutes []string `json:"advertise_routes"`
	SSH             bool     `json:"ssh"`
	// AcceptDNS must stay false (invariants §11); true means someone changed it outside Travo.
	AcceptDNS   bool `json:"accept_dns"`
	WantRunning bool `json:"want_running"`
}

// Backend is the logged-in view of tailscaled, parsed from status and prefs.
type Backend struct {
	State     string     `json:"backend_state" doc:"tailscaled backend state: NeedsLogin, NeedsMachineAuth, Starting, Running, Stopped or NoState; empty when the daemon is not running"`
	AuthURL   string     `json:"auth_url" doc:"Login URL while the backend waits for the user; empty otherwise"`
	Self      Node       `json:"self"`
	MagicDNS  MagicDNS   `json:"magic_dns"`
	ExitNodes []ExitNode `json:"exit_nodes"`
	Prefs     Prefs      `json:"prefs"`
	Health    []string   `json:"health"`
}

// Parse is pure: status and prefs JSON in, the backend view out. Peers are sorted by
// host name so the UI order does not depend on map iteration.
func Parse(statusJSON, prefsJSON []byte) (Backend, error) {
	var st rawStatus
	if err := json.Unmarshal(statusJSON, &st); err != nil {
		return Backend{}, fmt.Errorf("tailscale status: %w", err)
	}
	var p rawPrefs
	if err := json.Unmarshal(prefsJSON, &p); err != nil {
		return Backend{}, fmt.Errorf("tailscale prefs: %w", err)
	}
	if st.BackendState == "" {
		return Backend{}, fmt.Errorf("tailscale status: no BackendState")
	}
	b := Backend{State: st.BackendState, AuthURL: st.AuthURL, Health: nonNil(st.Health), ExitNodes: []ExitNode{}}
	b.Self = node(st.Self)
	if st.CurrentTailnet != nil {
		b.MagicDNS = MagicDNS{Enabled: st.CurrentTailnet.MagicDNSEnabled, Suffix: strings.TrimSuffix(st.CurrentTailnet.MagicDNSSuffix, ".")}
	}
	var peers []*rawPeer
	for _, peer := range st.Peer {
		if peer != nil && peer.ExitNodeOption {
			peers = append(peers, peer)
		}
	}
	slices.SortFunc(peers, func(a, b *rawPeer) int {
		return strings.Compare(a.HostName+"\x00"+a.ID, b.HostName+"\x00"+b.ID)
	})
	for _, peer := range peers {
		b.ExitNodes = append(b.ExitNodes, ExitNode{Node: node(peer), Active: peer.ExitNode})
	}
	b.Prefs = Prefs{
		ExitNode: exitNodeRef(p, peers), AdvertiseRoutes: nonNil(p.AdvertiseRoutes),
		SSH: p.RunSSH, AcceptDNS: p.CorpDNS, WantRunning: p.WantRunning,
	}
	return b, nil
}

// exitNodeRef names the selected exit node by stable ID. A selection made by IP that the
// netmap has not resolved yet keeps the IP, so a GET still shows a selection exists.
func exitNodeRef(p rawPrefs, peers []*rawPeer) string {
	if p.ExitNodeID != "" {
		return p.ExitNodeID
	}
	if p.ExitNodeIP == "" {
		return ""
	}
	for _, peer := range peers {
		if slices.Contains(peer.TailscaleIPs, p.ExitNodeIP) {
			return peer.ID
		}
	}
	return p.ExitNodeIP
}

func node(p *rawPeer) Node {
	if p == nil {
		return Node{IPs: []string{}}
	}
	return Node{ID: p.ID, HostName: p.HostName, DNSName: p.DNSName, IPs: nonNil(p.TailscaleIPs), Online: p.Online}
}

// IPv4 returns the node's Tailscale IPv4 address; `tailscale set --exit-node` takes it.
func (n Node) IPv4() (string, bool) {
	for _, s := range n.IPs {
		if a, err := netip.ParseAddr(s); err == nil && a.Is4() {
			return s, true
		}
	}
	return "", false
}

func nonNil[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}
