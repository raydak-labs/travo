package dnslayer

import "slices"

// owned lists the dnsmasq options the stack writes, in the order Stage writes new ones.
var owned = []string{"server", "noresolv", "rebind_protection"}

// opKind is one uci write Stage issues.
type opKind int

const (
	opDelete opKind = iota // uci delete
	opWrite                // uci set (scalar) or add_list per value
)

type stageOp struct {
	kind opKind
	name string
}

// planStage returns the writes that turn cur into want. It leaves options whose value
// already matches untouched, and when want carries an order (a restored base), it
// re-appends the tail of owned options that would otherwise come back out of order:
// uci keeps an updated option in place but appends a re-added list or new option (#157).
func planStage(cur, want State) []stageOp {
	var ops []stageOp
	order := slices.Clone(cur.Order)
	rewrite := func(name string) {
		if isSet(cur, name) || slices.Contains(order, name) {
			ops = append(ops, stageOp{opDelete, name})
			order = slices.DeleteFunc(order, func(n string) bool { return n == name })
		}
		if isSet(want, name) {
			ops = append(ops, stageOp{opWrite, name})
			order = append(order, name)
		}
	}
	for _, name := range owned {
		switch {
		case equalOpt(cur, want, name):
		case name == "server" || !isSet(want, name) || !isSet(cur, name):
			rewrite(name) // lists cannot be updated in place; deletes and new options move
		default:
			ops = append(ops, stageOp{opWrite, name}) // uci set keeps the position
		}
	}
	if len(want.Order) == 0 {
		return ops // a record from before #157: order unknown, write only what changed
	}
	wantOrder := setNames(want, targetOrder(want.Order))
	got := setNames(want, order)
	for i := range wantOrder {
		if i < len(got) && got[i] == wantOrder[i] {
			continue
		}
		for _, name := range wantOrder[i:] {
			rewrite(name)
		}
		break
	}
	return ops
}

// targetOrder is base's order followed by the owned options base lacks, in the order
// Stage appends them when a layer adds them. A layered state and its restore then
// always land in the same order.
func targetOrder(base []string) []string {
	out := slices.Clone(base)
	for _, n := range owned {
		if !slices.Contains(out, n) {
			out = append(out, n)
		}
	}
	return out
}

// setNames filters names to owned options set in st, keeping their order.
func setNames(st State, names []string) []string {
	var out []string
	for _, n := range names {
		if slices.Contains(owned, n) && isSet(st, n) {
			out = append(out, n)
		}
	}
	return out
}

func isSet(st State, name string) bool {
	switch name {
	case "server":
		return st.Server.Set
	case "noresolv":
		return st.NoResolv.Set
	default:
		return st.RebindProtection.Set
	}
}

func equalOpt(a, b State, name string) bool {
	switch name {
	case "server":
		return a.Server.Set == b.Server.Set && a.Server.Scalar == b.Server.Scalar && slices.Equal(a.Server.Values, b.Server.Values)
	case "noresolv":
		return a.NoResolv == b.NoResolv
	default:
		return a.RebindProtection == b.RebindProtection
	}
}
