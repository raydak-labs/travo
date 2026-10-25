package services

import (
	"log/slog"
	"slices"
	"sync"

	"github.com/openwrt-travel-gui/backend/internal/uci"
)

// uciReverter is implemented by UCI backends that can drop staged (uncommitted)
// changes for a config. RealUCI implements it with `uci revert <config>`; the
// in-memory mock does not, which simply skips the rollback.
type uciReverter interface {
	Revert(config string) error
}

// uciConfigLocks maps a config name to its lock, so a writer of `firewall` and
// a writer of `dhcp` never block each other.
var uciConfigLocks sync.Map // config name -> *sync.Mutex

// lockUCIConfigs locks each named config and returns a single unlock func.
//
// The names are sorted before locking, which makes the acquisition order GLOBAL
// and identical on every path. That is what makes nesting safe: with a fixed
// order, no two goroutines can hold overlapping sets in opposite orders, so the
// wait-for graph is acyclic and deadlock is impossible. Without the sort, two
// flows that each need {network, firewall} but list them in opposite order
// would deadlock the moment they met — and on a router that means every other
// endpoint that touches a config also stops responding, because they all queue
// on the same per-config locks.
//
// The locks are still NOT reentrant: a flow that holds a config must not ask for
// it again. Helpers called from inside a transaction therefore have a lock-free
// core (…Locked) that the transaction calls, while the thin exported wrapper
// takes the lock for callers that enter cold. See setupWireGuardFirewallLocked.
func lockUCIConfigs(configs ...string) func() {
	// Sorted + de-duplicated: acquiring the same mutex twice on one path would
	// self-deadlock, and mutateUCI callers legitimately pass a set.
	names := append([]string(nil), configs...)
	slices.Sort(names)
	locks := make([]*sync.Mutex, 0, len(names))
	prev := ""
	for i, c := range names {
		if i > 0 && c == prev {
			continue
		}
		prev = c
		v, _ := uciConfigLocks.LoadOrStore(c, &sync.Mutex{})
		m := v.(*sync.Mutex)
		m.Lock()
		locks = append(locks, m)
	}
	return func() {
		for i := len(locks) - 1; i >= 0; i-- {
			locks[i].Unlock()
		}
	}
}

// revertUCIConfig drops the staged UCI delta for each config that has one.
//
// The uci CLI keeps uncommitted changes in the process-global
// /tmp/.uci/<config>/changes file, so a write sequence that fails half-way
// must be reverted: otherwise a later, unrelated `uci commit <config>` (a WAN
// save, a DHCP change, …) silently persists the abandoned delta.
//
// The caller must already hold the config lock for every name it passes, since
// the revert discards whatever *anyone* has staged for that config.
func revertUCIConfig(u uci.UCI, configs ...string) {
	reverter, ok := u.(uciReverter)
	if !ok {
		return
	}
	for _, config := range configs {
		if err := reverter.Revert(config); err != nil {
			slog.Warn("uci revert", "config", config, "err", err)
		}
	}
}

// mutateUCI is mutateWireless for the other services: it holds the named
// configs' locks for the duration of fn and reverts all of them if fn fails.
// Same one-list rule, same reason — see mutateWireless.
func mutateUCI(u uci.UCI, configs []string, fn func() error) error {
	defer lockUCIConfigs(configs...)()

	if err := fn(); err != nil {
		revertUCIConfig(u, configs...)
		return err
	}
	return nil
}
