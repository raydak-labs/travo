package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// ---------------------------------------------------------------------------
// Shared dnsmasq resolver layer stack (ADR 0001 §3)
//
// AdGuard forwarding and VPN DNS forwarding both write the SAME two options of
// the SAME section: dhcp.@dnsmasq[0].server and .noresolv. Separate per-feature
// snapshot FILES cannot keep them from colliding — the files stay separate but
// the EFFECTS overwrite each other, so the last restore wins with a value that
// was valid in a state the operator has since left.
//
// The stack is the single owner of the pre-any-layer state:
//
//   - the first layer to enable records the base state and its own resolvers;
//   - the last layer to disable restores the base and drops the record;
//   - a layer disabling while others remain only removes its own entry and
//     leaves dnsmasq pointing at whatever is now on top.
//
// The record lives here rather than in AdGuardService because VpnService is the
// service that also owns the self-heal that pops a layer on the read path;
// AdGuardService drives the same code through AdGuardChecker.
const (
	// dnsmasqLayerStackPath holds the base dnsmasq resolver state plus the
	// layers currently stacked, bottom first. It is a state file, not a crash
	// guard, so it keeps its own literal rather than deriving from guard.DefaultDir.
	dnsmasqLayerStackPath = "/etc/trafo/dnsmasq-layers.json"

	// dnsmasqLayerStackFileName is the record's file name on its own, so a
	// service that resolves the record relative to its own state directory (the
	// captive service, next to its guard file) lands on the same file.
	dnsmasqLayerStackFileName = "dnsmasq-layers.json"

	// The features that stack onto dnsmasq's resolver options.
	dnsLayerVPN     = "vpn"
	dnsLayerAdGuard = "adguard"
	// dnsLayerCaptive is the captive portal bypass (ADR 0001 §4). It is a layer
	// like the other two, not a second writer of the same two UCI options: a
	// snapshot outside the stack cannot see the layers below it, so its restore
	// lands on top of a state it never observed.
	dnsLayerCaptive = "captive"
)

// dnsmasqResolverState is a dnsmasq resolver list plus its noresolv flag. It is
// the shape the per-feature snapshots wrote, kept so a device that is upgraded
// while a layer is active is still readable.
type dnsmasqResolverState struct {
	NoResolv string   `json:"noresolv"`
	Servers  []string `json:"servers"`
}

// dnsmasqLayer is one stacked feature and the resolvers it wants dnsmasq to
// forward to.
//
// NoResolv is the flag this layer wants while it owns dnsmasq. It is absent
// from the record unless a layer needs something other than the default: a
// layer recorded before it existed means "1", so records written by earlier
// releases keep meaning the same thing.
type dnsmasqLayer struct {
	Name     string   `json:"name"`
	Servers  []string `json:"servers"`
	NoResolv string   `json:"noresolv,omitempty"`
}

// noResolvValue is the flag a layer applies while it owns dnsmasq.
func (l dnsmasqLayer) noResolvValue() string {
	if strings.TrimSpace(l.NoResolv) != "" {
		return strings.TrimSpace(l.NoResolv)
	}
	return "1"
}

// dnsmasqLayerStack is the shared record. Layers are ordered bottom first, so
// the last entry is the one that owns dnsmasq right now.
type dnsmasqLayerStack struct {
	NoResolv string         `json:"noresolv"`
	Servers  []string       `json:"servers"`
	Layers   []dnsmasqLayer `json:"layers"`
}

// indexOf returns the position of the named layer, or -1.
func (s *dnsmasqLayerStack) indexOf(layer string) int {
	for i, l := range s.Layers {
		if l.Name == layer {
			return i
		}
	}
	return -1
}

// dnsmasqDNS is the surface the shared stack needs: dnsmasq's two resolver
// options plus one small file. VpnService reaches the system through
// CommandRunner and AdGuardService through AdGuardChecker; the stack needs
// neither, so both implement this.
type dnsmasqDNS interface {
	getServers() ([]string, error)
	getNoResolv() (string, error)
	setServers(servers []string) error
	setNoResolv(value string) error
	commitAndRestart() error
	readFile(path string) ([]byte, error)
	writeFile(path string, data []byte, perm os.FileMode) error
	removeFile(path string) error
}

// commandRunnerDNS drives the stack through VpnService's CommandRunner.
//
// The dnsmasq writes are spelled out one shell-out per call rather than
// assembled from an argument slice so they stay greppable:
// TestShelledOutUCIWritesAreOnTheRecord asserts that this file still names every
// shelled-out `dhcp` write, which is what keeps `dhcp` in vpnFlowConfigs
// (ADR 0010).
type commandRunnerDNS struct{ cmd CommandRunner }

func (c commandRunnerDNS) getServers() ([]string, error) {
	out, err := c.cmd.Run("uci", "get", "dhcp.@dnsmasq[0].server")
	if err != nil {
		return nil, err
	}
	// `uci get` returns a space-separated list for list options.
	return strings.Fields(strings.TrimSpace(string(out))), nil
}

func (c commandRunnerDNS) getNoResolv() (string, error) {
	out, err := c.cmd.Run("uci", "get", "dhcp.@dnsmasq[0].noresolv")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func (c commandRunnerDNS) setServers(servers []string) error {
	// Best-effort clear: `uci delete` exits 1 with "Entry not found" when the
	// list is already empty, which is the normal state on a router that has
	// never had forwarding configured.
	_, _ = c.cmd.Run("uci", "delete", "dhcp.@dnsmasq[0].server")
	for _, s := range servers {
		if err := c.addDnsmasqServer(s); err != nil {
			return fmt.Errorf("adding dnsmasq server %q: %w", s, err)
		}
	}
	return nil
}

// addDnsmasqServer appends one entry to dhcp.@dnsmasq[0].server. The
// fmt.Sprintf stays on the call line so TestShelledOutUCIWritesAreOnTheRecord
// can still find this shelled-out `dhcp` write.
func (c commandRunnerDNS) addDnsmasqServer(s string) error {
	_, err := c.cmd.Run("uci", "add_list", fmt.Sprintf("dhcp.@dnsmasq[0].server=%s", s))
	return err
}

func (c commandRunnerDNS) setNoResolv(value string) error {
	if _, err := c.cmd.Run("uci", "set", "dhcp.@dnsmasq[0].noresolv="+value); err != nil {
		return fmt.Errorf("setting dnsmasq noresolv=%s: %w", value, err)
	}
	return nil
}

func (c commandRunnerDNS) commitAndRestart() error {
	if _, err := c.cmd.Run("uci", "commit", "dhcp"); err != nil {
		return fmt.Errorf("committing dhcp: %w", err)
	}
	if _, err := c.cmd.Run("/etc/init.d/dnsmasq", "restart"); err != nil {
		return fmt.Errorf("restarting dnsmasq: %w", err)
	}
	return nil
}

func (c commandRunnerDNS) readFile(path string) ([]byte, error) { return os.ReadFile(path) }

// writeFile writes the record atomically. A power cut or a concurrent reader
// between the truncate and the write left a half-written JSON document, and a
// record that does not parse is not a recoverable state: load() fails, so every
// layer enable aborts and every restore is refused until an operator deletes a
// file whose contents they cannot read.
func (c commandRunnerDNS) writeFile(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }() // no-op after a successful rename
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpName, perm); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

func (c commandRunnerDNS) removeFile(path string) error {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// dnsmasqLayerStackFile is the shared record of dnsmasq resolver layers.
type dnsmasqLayerStackFile struct {
	dns  dnsmasqDNS
	path string
}

// load reads the persisted stack. A missing record is not an error — it is how
// "no layer was ever enabled" is represented.
func (f *dnsmasqLayerStackFile) load() (*dnsmasqLayerStack, error) {
	data, err := f.dns.readFile(f.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var st dnsmasqLayerStack
	if err := json.Unmarshal(data, &st); err != nil {
		return nil, fmt.Errorf("parsing the dnsmasq layer stack %s: %w", f.path, err)
	}
	return &st, nil
}

func (f *dnsmasqLayerStackFile) save(st *dnsmasqLayerStack) error {
	data, err := json.Marshal(st)
	if err != nil {
		return err
	}
	return f.dns.writeFile(f.path, data, 0o600)
}

// removeRecord drops the shared record. Called only once the base state has
// been applied to dnsmasq.
func (f *dnsmasqLayerStackFile) removeRecord() error {
	return f.dns.removeFile(f.path)
}

// liveState reads dnsmasq's current resolver options. A missing option is
// normal (an unset list, unset noresolv), so read errors are not failures.
func (f *dnsmasqLayerStackFile) liveState() dnsmasqResolverState {
	var state dnsmasqResolverState
	if servers, err := f.dns.getServers(); err == nil {
		state.Servers = servers
	}
	if noresolv, err := f.dns.getNoResolv(); err == nil {
		state.NoResolv = noresolv
	}
	return state
}

// hasLayer reports whether layer is stacked.
func (f *dnsmasqLayerStackFile) hasLayer(layer string) bool {
	st, err := f.load()
	return err == nil && st != nil && st.indexOf(layer) >= 0
}

// apply writes a resolver list and its noresolv flag, then commits and
// restarts dnsmasq.
func (f *dnsmasqLayerStackFile) apply(state dnsmasqResolverState) error {
	if err := f.dns.setServers(state.Servers); err != nil {
		return err
	}
	noresolv := strings.TrimSpace(state.NoResolv)
	if noresolv == "" {
		noresolv = "0"
	}
	if err := f.dns.setNoResolv(noresolv); err != nil {
		return err
	}
	return f.dns.commitAndRestart()
}

// applyTop points dnsmasq at the top of the stack, or at the base state when
// no layer is left. Every stacked layer sets noresolv=1 — forwarding must not
// fall back to resolv.conf while a layer owns the resolvers.
func (f *dnsmasqLayerStackFile) applyTop(st *dnsmasqLayerStack) error {
	if len(st.Layers) == 0 {
		return f.apply(dnsmasqResolverState{NoResolv: st.NoResolv, Servers: st.Servers})
	}
	top := st.Layers[len(st.Layers)-1]
	return f.apply(dnsmasqResolverState{NoResolv: top.noResolvValue(), Servers: top.Servers})
}

// hasAnyLayer reports whether any feature currently owns dnsmasq's
// resolvers. A caller restoring something OUTSIDE the stack asks this first:
// with another layer on top, dnsmasq is not in the state the caller left it
// in, and "restoring" it would break the layer that is working.
//
// It fails closed. An unreadable record is not "no owner": it is the one case
// where the caller must NOT write, because something may own dnsmasq in a way
// this record can no longer describe. The error is returned so the caller can
// fail loudly instead of guessing.
func (f *dnsmasqLayerStackFile) hasAnyLayer() (bool, error) {
	st, err := f.load()
	if err != nil {
		return false, err
	}
	return st != nil && len(st.Layers) > 0, nil
}

// EnableLayer pushes layer onto the stack and points dnsmasq at its resolvers.
// It is idempotent: re-enabling a layer that is already stacked updates its
// resolvers in place and never re-reads dnsmasq as the base, so a second enable
// can never record a layer's own entry as the pre-layer state.
func (f *dnsmasqLayerStackFile) EnableLayer(layer string, servers []string) error {
	return f.EnableLayerAs(layer, servers, "1")
}

// EnableLayerAs pushes a layer that wants a specific noresolv flag. The captive
// bypass is the only caller: it points dnsmasq at the upstream network's own
// resolver so a portal hostname resolves to the gateway, which means handing
// fallback back to resolv.conf (noresolv=0) rather than cutting it off.
func (f *dnsmasqLayerStackFile) EnableLayerAs(layer string, servers []string, noresolv string) error {
	st, err := f.load()
	if err != nil {
		return err
	}
	if st == nil {
		base := f.liveState()
		st = &dnsmasqLayerStack{NoResolv: base.NoResolv, Servers: base.Servers}
	} else if len(st.Layers) == 0 {
		// A record with no layer left is not holding anything: the read-path
		// heal keeps it so a later disable still has a base to restore. Keeping
		// the base it was written with does not work, because the operator can
		// edit dnsmasq in LuCI between the heal and that disable — and the
		// disable would then put the pre-heal resolvers back over the change.
		// Nothing owns dnsmasq right now, so the live state IS the pre-layer
		// state and is read fresh.
		live := f.liveState()
		st.NoResolv, st.Servers = live.NoResolv, live.Servers
	}
	if i := st.indexOf(layer); i >= 0 {
		st.Layers[i].Servers = servers
		st.Layers[i].NoResolv = noresolv
	} else {
		st.Layers = append(st.Layers, dnsmasqLayer{Name: layer, Servers: servers, NoResolv: noresolv})
	}
	// The record is written before dnsmasq is touched: without it there would
	// be nothing to restore, and a restore target that cannot be written must
	// abort the change rather than follow it.
	if err := f.save(st); err != nil {
		return fmt.Errorf("recording the dnsmasq %s layer: %w", layer, err)
	}
	return f.applyTop(st)
}

// RemoveLayer takes layer off the stack.
//
// keepRecord is for the read-path heal. Popping the last layer still restores
// the base state, but the record survives so the heal is idempotent and a
// later explicit disable — or a re-enable followed by a disable — still
// restores the true pre-any-layer state. The explicit disable path passes
// false: it has applied the base, so the record is done.
//
// Returns applied=false when there was no record for this feature at all, so
// the caller can decide on a no-layer fallback instead of having this decide
// for it.
func (f *dnsmasqLayerStackFile) RemoveLayer(layer string, keepRecord bool) (bool, error) {
	st, err := f.load()
	if err != nil {
		return false, err
	}
	if st == nil {
		return false, nil
	}
	if i := st.indexOf(layer); i >= 0 {
		st.Layers = slices.Delete(st.Layers, i, i+1)
	} else if len(st.Layers) > 0 {
		// This layer is not stacked and something else is on top of dnsmasq:
		// that layer owns the state, and touching it would break the one
		// currently working.
		return false, nil
	}
	if err := f.applyTop(st); err != nil {
		return false, err
	}
	if len(st.Layers) > 0 || keepRecord {
		return true, f.save(st)
	}
	return true, f.removeRecord()
}

// ClearNoResolv hands resolution back to resolv.conf without touching the
// server list. It is the only safe action when no record exists: the list may
// be entirely the operator's own split-DNS entries.
func (f *dnsmasqLayerStackFile) ClearNoResolv() error {
	if err := f.dns.setNoResolv("0"); err != nil {
		return err
	}
	return f.dns.commitAndRestart()
}
