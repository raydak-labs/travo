package dnslayer

import (
	"reflect"
	"testing"
)

// The real router's base is old Travo's forwarding (`list server '127.0.0.1#5353'`,
// noresolv=1). Without adoption, popping the adguard layer would restore that same
// forwarding and fail-open would change nothing.
func TestAdoptOldTravoForwardingThenPopRemovesIt(t *testing.T) {
	s, d, a := setup(t, deviceFixture)
	before := live(t, d)
	ok, err := s.Adopt(ctx, layers[AdGuard])
	if err != nil || !ok {
		t.Fatalf("adopt: %v %v", ok, err)
	}
	if a.n != 0 || !reflect.DeepEqual(live(t, d), before) {
		t.Fatal("adopt must not write DNS")
	}
	st, _ := s.Status()
	if !reflect.DeepEqual(st.Layers, []Name{AdGuard}) || st.Pending {
		t.Fatalf("status = %+v", st)
	}
	if err := s.Pop(ctx, AdGuard); err != nil {
		t.Fatal(err)
	}
	after := live(t, d)
	if after.Server.Set || after.NoResolv.Set || after.RebindProtection != before.RebindProtection {
		t.Fatalf("pop must leave a resolv-file base: %+v", after)
	}
	if ok, err := s.Adopt(ctx, layers[AdGuard]); err != nil || ok {
		t.Fatalf("nothing left to adopt: %v %v", ok, err)
	}
}

func TestAdoptIgnoresOtherStatesAndExistingRecords(t *testing.T) {
	s, _, _ := setup(t, listFixture) // servers 1.1.1.1 + /lan/...: not the adguard layer
	if ok, err := s.Adopt(ctx, layers[AdGuard]); err != nil || ok {
		t.Fatalf("%v %v", ok, err)
	}
	s, _, _ = setup(t, deviceFixture)
	if err := s.Push(ctx, layers[Captive]); err != nil {
		t.Fatal(err)
	}
	if ok, err := s.Adopt(ctx, layers[AdGuard]); err != nil || ok {
		t.Fatalf("a record exists: %v %v", ok, err)
	}
	if rec, _ := Adopt(nil, State{Server: List{Set: true, Values: []string{"127.0.0.1#5353"}}}, layers[AdGuard]); rec != nil {
		t.Fatal("noresolv must match too")
	}
	if _, err := Adopt(nil, State{}, Layer{Name: "bogus", Servers: []string{"x"}}); err == nil {
		t.Fatal("unknown layer accepted")
	}
}
