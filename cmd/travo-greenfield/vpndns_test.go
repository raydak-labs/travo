package main

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/blackdark-org/travo-greenfield/internal/app"
	"github.com/blackdark-org/travo-greenfield/internal/dnslayer"
	"github.com/blackdark-org/travo-greenfield/internal/vpndns"
	"github.com/blackdark-org/travo-greenfield/internal/vpnstatus"
	"github.com/blackdark-org/travo-greenfield/internal/vpntunnel"
)

type connectedTunnel struct{}

func (connectedTunnel) Status(context.Context) (vpnstatus.TunnelStatus, error) {
	return vpnstatus.TunnelStatus{State: vpnstatus.Connected}, nil
}

type layerSpy struct{ pops int }

func (*layerSpy) Load() (*dnslayer.Record, error) {
	return &dnslayer.Record{Layers: []dnslayer.Layer{{Name: dnslayer.VPN, Servers: []string{"198.18.0.1"}, NoResolv: true}}}, nil
}
func (*layerSpy) Status() (dnslayer.Status, error)           { return dnslayer.Status{}, nil }
func (*layerSpy) Push(context.Context, dnslayer.Layer) error { return nil }
func (l *layerSpy) Pop(context.Context, dnslayer.Name) error { l.pops++; return nil }

// The reconciler reads network through the transaction locks, so a staged
// travo_dns=none that the transaction then reverts never pops the layer.
func TestVPNDNSReconcileWaitsForStagedNetwork(t *testing.T) {
	ctx := context.Background()
	deps, err := app.New(app.Options{Mock: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, w := range [][3]string{{"proto", "wireguard"}, {vpntunnel.ResolversOption, "198.18.0.1"}} {
		if err := deps.Device.SetType(ctx, "network", "wg0", "interface"); err != nil {
			t.Fatal(err)
		}
		if err := deps.Device.Set(ctx, "network", "wg0", w[0], w[1]); err != nil {
			t.Fatal(err)
		}
	}
	if err := deps.Device.Commit(ctx, "network"); err != nil {
		t.Fatal(err)
	}
	layers := &layerSpy{}
	svc := newVPNDNS(deps, layers, connectedTunnel{}, nil)
	staged, release, aborted := make(chan struct{}), make(chan struct{}), make(chan error, 1)
	// Stage under the network write lock and revert, as txn.Manager.begin does on abort.
	go func() {
		lctx, unlock, err := deps.Txn.Locks.Lock(ctx, []string{"network"})
		if err != nil {
			close(staged)
			aborted <- err
			return
		}
		defer unlock()
		err = deps.Device.Set(lctx, "network", "wg0", vpntunnel.ResolversOption, "none")
		close(staged)
		<-release
		aborted <- errors.Join(err, deps.Device.Revert(lctx, "network"))
	}()
	<-staged
	waiting := make(chan struct{})
	var once sync.Once
	deps.Txn.Locks.OnContended(func(string) { once.Do(func() { close(waiting) }) })
	done := make(chan vpndns.Action, 1)
	go func() {
		action, err := svc.Reconcile(ctx)
		if err != nil {
			t.Error(err)
		}
		done <- action
	}()
	select {
	case <-waiting: // Reconcile is blocked on the lock the staging writer holds
	case <-done:
		t.Fatal("reconcile read network while a transaction was staging it")
	case <-time.After(10 * time.Second):
		t.Fatal("reconcile never reached the network lock")
	}
	close(release)
	if err := <-aborted; err != nil {
		t.Fatal(err)
	}
	if action := <-done; action != vpndns.Keep || layers.pops != 0 {
		t.Fatalf("action %v, pops %d", action, layers.pops)
	}
}
