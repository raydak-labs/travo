package dnslayer

import (
	"slices"
	"testing"
)

// applyOrder replays planStage's ops the way uci orders a section: an update keeps its
// place, a delete removes, a write of an absent option appends.
func applyOrder(order []string, ops []stageOp) []string {
	order = slices.Clone(order)
	for _, op := range ops {
		i := slices.Index(order, op.name)
		switch {
		case op.kind == opDelete && i >= 0:
			order = slices.Delete(order, i, i+1)
		case op.kind == opWrite && i < 0:
			order = append(order, op.name)
		}
	}
	return order
}

func TestPlanStageRestoresBaseOrder(t *testing.T) {
	bypass := State{
		Server:           List{Set: true, Values: []string{"10.66.0.1"}},
		NoResolv:         Opt{Set: true, Value: "0"},
		RebindProtection: Opt{Set: true, Value: "0"},
	}
	for _, tc := range []struct {
		name string
		base State
	}{
		{"server before noresolv (AdGuard base, #157)", State{
			Server:   List{Set: true, Values: []string{"127.0.0.1#5353"}},
			NoResolv: Opt{Set: true, Value: "1"},
			Order:    []string{"server", "noresolv"},
		}},
		{"rebind and noresolv before server (uci-export-dhcp.txt)", State{
			Server:           List{Set: true, Values: []string{"127.0.0.1#5353"}},
			NoResolv:         Opt{Set: true, Value: "1"},
			RebindProtection: Opt{Set: true, Value: "1"},
			Order:            []string{"rebind_protection", "noresolv", "server"},
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			layered := bypass
			layered.Order = tc.base.Order // Effective keeps base's order
			mid := applyOrder(tc.base.Order, planStage(tc.base, layered))
			cur := bypass
			cur.Order = mid
			got := applyOrder(mid, planStage(cur, tc.base))
			if want := setNames(tc.base, tc.base.Order); !slices.Equal(got, want) {
				t.Fatalf("restored order %v, want %v (after layer: %v)", got, want, mid)
			}
		})
	}
}

// TestPlanStageLayerOverAdGuard is the #157 repro: base has no server, AdGuard adds
// server and noresolv, the captive bypass replaces them, and removing the bypass must
// return the AdGuard state byte for byte.
func TestPlanStageLayerOverAdGuard(t *testing.T) {
	base := State{RebindProtection: Opt{Set: true, Value: "1"}, Order: []string{"rebind_protection"}}
	adguard := State{
		Server:           List{Set: true, Values: []string{"127.0.0.1#5353"}},
		NoResolv:         Opt{Set: true, Value: "1"},
		RebindProtection: base.RebindProtection,
		Order:            base.Order,
	}
	bypass := State{
		Server:           List{Set: true, Values: []string{"10.66.0.1"}},
		NoResolv:         Opt{Set: true, Value: "0"},
		RebindProtection: Opt{Set: true, Value: "0"},
		Order:            base.Order,
	}
	step := func(cur State, order []string, want State) ([]string, State) {
		cur.Order = order
		next := applyOrder(order, planStage(cur, want))
		return next, want
	}
	order, cur := step(base, base.Order, adguard)
	agOrder := slices.Clone(order)
	order, cur = step(cur, order, bypass)
	order, _ = step(cur, order, adguard)
	if !slices.Equal(order, agOrder) || !slices.Equal(agOrder, []string{"rebind_protection", "server", "noresolv"}) {
		t.Fatalf("adguard order %v, after bypass and restore %v", agOrder, order)
	}
}

func TestPlanStageLeavesMatchingOptionsAlone(t *testing.T) {
	base := State{
		Server:   List{Set: true, Values: []string{"127.0.0.1#5353"}},
		NoResolv: Opt{Set: true, Value: "1"},
		Order:    []string{"server", "noresolv"},
	}
	if ops := planStage(base, base); len(ops) != 0 {
		t.Fatalf("want no writes, got %v", ops)
	}
	// A record from before #157 has no order: only the changed option is written.
	noOrder := base
	noOrder.Order = nil
	want := noOrder
	want.NoResolv = Opt{Set: true, Value: "0"}
	if ops := planStage(noOrder, want); !slices.Equal(ops, []stageOp{{opWrite, "noresolv"}}) {
		t.Fatalf("got %v", ops)
	}
}
