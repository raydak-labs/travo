// Command travo-greenfield is the on-router daemon.
package main

import (
	"context"
	"crypto/rand"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/blackdark-org/travo-greenfield/internal/adguard"
	"github.com/blackdark-org/travo-greenfield/internal/api"
	"github.com/blackdark-org/travo-greenfield/internal/apitokens"
	"github.com/blackdark-org/travo-greenfield/internal/app"
	"github.com/blackdark-org/travo-greenfield/internal/auth"
	"github.com/blackdark-org/travo-greenfield/internal/auth/epochfile"
	"github.com/blackdark-org/travo-greenfield/internal/captive"
	"github.com/blackdark-org/travo-greenfield/internal/connectivity"
	"github.com/blackdark-org/travo-greenfield/internal/device"
	"github.com/blackdark-org/travo-greenfield/internal/device/cli"
	"github.com/blackdark-org/travo-greenfield/internal/dhcpserver"
	"github.com/blackdark-org/travo-greenfield/internal/diagnostics"
	"github.com/blackdark-org/travo-greenfield/internal/dnslayer"
	"github.com/blackdark-org/travo-greenfield/internal/factoryreset"
	"github.com/blackdark-org/travo-greenfield/internal/guard"
	"github.com/blackdark-org/travo-greenfield/internal/hook"
	"github.com/blackdark-org/travo-greenfield/internal/ifrestart"
	"github.com/blackdark-org/travo-greenfield/internal/lanaddress"
	"github.com/blackdark-org/travo-greenfield/internal/lifecycle"
	"github.com/blackdark-org/travo-greenfield/internal/netview"
	"github.com/blackdark-org/travo-greenfield/internal/portfwd"
	"github.com/blackdark-org/travo-greenfield/internal/reboot"
	"github.com/blackdark-org/travo-greenfield/internal/sshkeys"
	"github.com/blackdark-org/travo-greenfield/internal/static"
	"github.com/blackdark-org/travo-greenfield/internal/syssettings"
	"github.com/blackdark-org/travo-greenfield/internal/systemlog"
	"github.com/blackdark-org/travo-greenfield/internal/throughput"
	"github.com/blackdark-org/travo-greenfield/internal/tlscert"
	"github.com/blackdark-org/travo-greenfield/internal/trust"
	"github.com/blackdark-org/travo-greenfield/internal/txn"
	"github.com/blackdark-org/travo-greenfield/internal/updateapply"
	"github.com/blackdark-org/travo-greenfield/internal/updatecheck"
	"github.com/blackdark-org/travo-greenfield/internal/updatestage"
	"github.com/blackdark-org/travo-greenfield/internal/version"
	"github.com/blackdark-org/travo-greenfield/internal/vpnkill"
	"github.com/blackdark-org/travo-greenfield/internal/vpnprofiles"
	"github.com/blackdark-org/travo-greenfield/internal/vpnstatus"
	"github.com/blackdark-org/travo-greenfield/internal/vpntunnel"
	"github.com/blackdark-org/travo-greenfield/internal/wifi"
	"github.com/blackdark-org/travo-greenfield/internal/wifiprofiles"
	"github.com/blackdark-org/travo-greenfield/internal/wifireconnect"
	"github.com/blackdark-org/travo-greenfield/internal/wol"
	"github.com/blackdark-org/travo-greenfield/packaging"
)

func main() {
	addr := flag.String("listen", "127.0.0.1:8081", "listen address")
	listenTLS := flag.String("listen-tls", "", "HTTPS listen address with a self-signed certificate; empty = off (ADR 0016)")
	useMock := flag.Bool("mock", false, "run against the mock device (no router needed)")
	firstRun := flag.Bool("mock-firstrun", false, "with --mock: start without a root password")
	hostAllow := flag.String("host-allow", "", "comma-separated extra Host names accepted (DNS-rebinding guard)")
	staticDir := flag.String("static-dir", "/www/travo-greenfield", "directory with the built web UI")
	releaseSource := flag.String("release-source", updatecheck.DefaultSource, "GitHub release source OWNER/REPO or off")
	worker := flag.Bool("update-worker", false, "internal detached update worker")
	awaitAck := flag.Bool("update-await-ack", false, "internal worker activation handshake")
	proxyListen := flag.String("adguard-proxy-listen", "", "AdGuard Home proxy address; empty = --listen host on port 3001, off disables")
	recoverUpdate := flag.Bool("recover-update", false, "recover an interrupted update locally")
	bundlePin := flag.Bool("adguard-bundle-pin", false, "print the offline AdGuard Home bundle pin (sha256 and file name) and exit")
	hookAction := flag.String("hook", "", "ask the running daemon to perform a hook action (cron jobs, button scripts) and exit")
	hookSocketPath := flag.String("hook-socket", hook.SocketPath, "hook socket the --hook client connects to")
	flag.Parse()
	if *hookAction != "" {
		os.Exit(runHook(*hookSocketPath, *hookAction))
	}
	if *bundlePin {
		os.Exit(printBundlePin(os.Stdout))
	}
	if *worker || *recoverUpdate {
		if err := runUpdateCommand(*worker, *awaitAck); err != nil {
			slog.Error("update recovery", "err", err)
			os.Exit(1)
		}
		return
	}
	dir := *staticDir
	if *useMock && !flagGiven("static-dir") {
		dir = "web/dist"
	}
	if err := run(*addr, *listenTLS, *proxyListen, *useMock, *firstRun, splitList(*hostAllow), dir, *releaseSource); err != nil {
		slog.Error("startup", "err", err)
		os.Exit(1)
	}
}

func runUpdateCommand(worker, awaitAck bool) error {
	if os.Geteuid() != 0 || runtime.GOOS != "linux" || runtime.GOARCH != "arm64" {
		return updateapply.ErrUnavailable
	}
	paths := app.RealPaths()
	svc := updateapply.New(guard.Dir{Path: paths.Guards, Now: time.Now}, cli.New(), nil, version.Version)
	if worker {
		return svc.Work(awaitAck)
	}
	return svc.RecoverLocal()
}

// printBundlePin prints the pin in sha256sum format for install.sh (ADR 0020); 1 when this
// build has none (a dev build, or a release without a bundle).
func printBundlePin(w io.Writer) int {
	if adguard.BundleSHA256 == "" || adguard.BundleName == "" {
		fmt.Fprintln(os.Stderr, "this build has no AdGuard Home bundle pin")
		return 1
	}
	fmt.Fprintf(w, "%s  %s\n", adguard.BundleSHA256, adguard.BundleName)
	return 0
}

func flagGiven(name string) bool {
	given := false
	flag.Visit(func(f *flag.Flag) { given = given || f.Name == name })
	return given
}

// uiHandler serves dir, or a 503 stub when it is missing. Missing or bad csp.json is logged
// and the policy then has no script hashes: the inline theme script is blocked (fails closed).
func uiHandler(dir string) http.Handler {
	if st, err := os.Stat(dir); err != nil || !st.IsDir() {
		slog.Warn("web UI directory missing: UI disabled", "dir", dir, "err", err)
		return static.Missing()
	}
	hashes, err := static.LoadHashes(dir)
	if err != nil {
		slog.Warn("csp.json unreadable: inline script will be blocked", "dir", dir, "err", err)
	}
	return static.Handler(dir, static.CSP(hashes))
}

// splitList splits a comma-separated flag, dropping blanks.
func splitList(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func run(addr, listenTLS, proxyListen string, useMock, firstRun bool, hostAllow []string, staticDir, releaseSource string) error {
	if !updatecheck.ValidSource(releaseSource) {
		return errors.New("invalid release source: expected OWNER/REPO or off")
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	workers := lifecycle.New(context.Background())
	defer workers.Stop()

	deps, err := app.New(app.Options{Mock: useMock, FirstRun: firstRun, StopCheck: workers.Stopped})
	if err != nil {
		return err
	}
	defer func() {
		if err := deps.Close(); err != nil {
			slog.Warn("cleanup", "err", err)
		}
	}()
	slog.Info("adapters", "mock", deps.Mock, "packages", deps.Packages.Name())

	clock := auth.SystemClock{}
	var rebooter api.Rebooter
	var bootID func(context.Context) string
	if router, ok := deps.Device.(reboot.Router); ok {
		svc := reboot.New(router, deps.Guards, deps.Paths.RebootFile(), clock.Now, workers.Stopped)
		if err := svc.Settle(ctx); err != nil {
			slog.Warn("reboot settlement", "err", err)
		}
		rebooter = svc
		bootID = func(ctx context.Context) string {
			id, err := svc.BootID(ctx)
			if err != nil {
				return ""
			}
			return id
		}
	}

	// Actions may also be registered later (Register is safe while serving): the WiFi
	// schedule adds one pair per rule once the AP editor exists.
	var hooks *hook.Server
	startHooks(deps, workers, func(s *hook.Server) { hooks = s })

	sessions, err := startState(ctx, deps)
	if err != nil {
		return err
	}
	limiter, provider := auth.NewRateLimiter(), &trust.Provider{}
	workers.Go("rate-limit-sweep", lifecycle.Every(time.Minute, func(context.Context) { limiter.Sweep(clock.Now()) }))
	if deps.Txn != nil {
		workers.Go("txn-reap", lifecycle.Every(15*time.Second, deps.Txn.Reap))
	}
	workers.Go("session-sweep", lifecycle.Every(time.Minute, func(context.Context) { sessions.Sweep(clock.Now()) }))
	workers.Go("trust-refresh", lifecycle.Every(30*time.Second, func(c context.Context) {
		if err := provider.Refresh(c, deps.Device); err != nil && c.Err() == nil {
			slog.Warn("trust: keeping last LAN subnets", "err", err)
		}
	}))

	sampler := throughput.New()
	if counters, ok := deps.Device.(device.InterfaceCounterReader); ok {
		workers.Go("interface-throughput", func(ctx context.Context) { sampler.Run(ctx, counters, clock.Now) })
	}
	var connection api.ConnectivityReader
	var captiveAssist api.CaptiveAssist
	var captiveRenew api.CaptiveRenewer
	var captiveAccept api.CaptiveAcceptor
	// One DNS layer stack for every feature (ADR 0010); applies invalidate the shared probe.
	dnsLayers := &dnslayer.Service{Device: deps.Device, Guards: deps.Guards, Apply: deps.Txn,
		RecordPath: filepath.Join(deps.Paths.State, "dns-layers.json")}
	if routes, ok := deps.Device.(device.DefaultRouteReader); ok {
		probe := connectivity.NewHTTPProbe(connectivity.ProbeURL, 3*time.Second)
		if deps.Mock {
			probe = func(context.Context) connectivity.Result { return connectivity.Result{Internet: connectivity.Online} }
		}
		shared := connectivity.New(ctx, routes, deps.Device, probe, clock.Now)
		shared.Links, _ = deps.Device.(device.NetLinkReader)
		connection, dnsLayers.AfterApply = shared, shared.Invalidate
		bypass := captive.NewBypass(dnsLayers, deps.Device, routes, clock.Now)
		captiveAssist = bypass
		workers.Go("captive-dns-restore", lifecycle.Every(15*time.Second, bypass.Tick))
		var renewer captive.LeaseRenew // nil without a lease port: the walker never renews
		if lease, ok := deps.Device.(device.LeaseRenewer); ok {
			r := captive.NewRenewer(deps.Guards, deps.Device, lease, routes, shared, clock.Now, captive.Sleep)
			captiveRenew, renewer = r, r
		}
		captiveAccept = captive.NewAcceptor(shared, renewer, captive.KernelPortalEdge())
	}
	backup, _ := deps.Device.(device.ConfigBackupper)
	profiles := vpnprofiles.New(filepath.Join(deps.Paths.State, "vpn-profiles.json"))
	var vpnStatus api.VPNStatusReader
	var vpnTunnel api.VPNTunnelSetter
	var vpnSplit api.VPNSplit
	var vpnKill api.VPNKillSwitch
	if wg, ok := deps.Device.(device.WireGuardReader); ok {
		statusStart := time.Now()
		keys, hasKeys := deps.Device.(device.WireGuardKeyWriter)
		control, hasControl := deps.Device.(device.WireGuardControl)
		routes, hasRoutes := deps.Device.(device.DefaultRouteReader)
		if hasKeys && hasControl && hasRoutes && deps.Txn != nil {
			tunnel := vpntunnel.New(deps.Device, keys, deps.Device, control, wg, routes, profiles, deps.Txn, clock.Now)
			vpnTunnel, vpnSplit = tunnel, tunnel
		}
		vpnStatus = vpnstatus.New(deps.Device, deps.Device, wg, profiles, clock.Now, func() time.Duration {
			return time.Since(statusStart)
		})
		if deps.Txn != nil {
			vpnKill = &vpnkill.Service{UCI: deps.Device, Tunnel: vpnStatus, Txns: deps.Txn}
		}
	}
	var vpnDNS api.VPNDNS
	if vpnStatus != nil {
		svc := newVPNDNS(deps, dnsLayers, vpnStatus, profiles)
		vpnDNS = svc
		workers.Go("vpn-dns-layer", lifecycle.Every(15*time.Second, func(c context.Context) {
			if _, err := svc.Reconcile(c); err != nil && c.Err() == nil {
				slog.Warn("vpn dns layer", "err", err)
			}
		}))
	}
	var wireless api.WiFiService
	var wirelessJoin api.WiFiJoiner
	var association wifiprofiles.AssociationReader
	var uplink wifireconnect.Uplink
	var wirelessAPs api.WiFiAPEditor
	var wirelessGuest api.WiFiGuestEditor
	var wirelessMAC api.WiFiMACEditor
	var staLockout ifrestart.STALockout
	if scanner, ok := deps.Device.(device.WirelessScanner); ok {
		service := wifi.New(ctx, deps.Device, scanner)
		wireless = service
		association = service
		routes, hasRoutes := deps.Device.(device.DefaultRouteReader)
		keys, hasKeys := deps.Device.(device.WirelessKeyWriter)
		links, hasLinks := deps.Device.(device.LinkIndexReader)
		neighbours, hasNeighbours := deps.Device.(device.NeighbourReader)
		if hasRoutes && hasKeys && hasLinks && hasNeighbours {
			joined := service.WithJoin(wifi.JoinDeps{UCI: deps.Device, Ubus: deps.Device, Routes: routes, Links: links, Keys: keys, Neighbours: neighbours, Txns: deps.Txn})
			wirelessJoin, uplink, wirelessAPs, wirelessGuest, wirelessMAC, staLockout = joined, joined, joined, joined, joined, joined
		}
	}
	savedUplinks := wifiprofiles.New(filepath.Join(deps.Paths.State, "wifi-profiles.json"), wirelessJoin, association)
	var reconnect api.WiFiReconnectReader
	if recovery, ok := deps.Device.(device.UplinkReconnector); ok && uplink != nil {
		svc := wifireconnect.New(filepath.Join(deps.Paths.State, "wifi-reconnect.json"), uplink, deps.Device, deps.Guards, recovery, clock.Now)
		reconnect = svc
		workers.Go("wifi-reconnect", lifecycle.Every(wifireconnect.Interval, svc.Tick))
	}

	if useMock {
		releaseSource = "off"
	}
	updates := updatecheck.New(ctx, releaseSource, version.Version, filepath.Join(deps.Paths.State, "update-check.json"), clock.Now, http.DefaultTransport)
	var updateStage api.UpdateStager
	var stageLease updateapply.Stage
	if verifier, ok := deps.Device.(device.ReleaseVerifier); ok && runtime.GOOS == "linux" && runtime.GOARCH == "arm64" && os.Geteuid() == 0 && releaseSource != "off" {
		stage, err := updatestage.Open(ctx, updatestage.Root, releaseSource, packaging.UpdatePublicKey, updatestage.ReleaseClient(), verifier, updates.EligibleVersion, clock.Now)
		if err != nil {
			slog.Warn("release staging unavailable", "err", err)
		} else {
			updateStage = stage
			stageLease = stage
			defer func() {
				if err := stage.Close(); err != nil {
					slog.Warn("release staging cleanup", "err", err)
				}
			}()
			workers.Go("release-stage-expiry", lifecycle.Every(time.Minute, func(context.Context) { stage.Sweep() }))
		}
	}
	var updater api.UpdateApplier
	if router, ok := deps.Device.(updateapply.Router); ok && !useMock && runtime.GOOS == "linux" && runtime.GOARCH == "arm64" && os.Geteuid() == 0 {
		svc := updateapply.New(deps.Guards, router, stageLease, version.Version)
		updater = svc
		if _, err := svc.Recover(); err != nil && !errors.Is(err, updateapply.ErrBusy) {
			slog.Warn("update recovery", "err", err)
		}
	}
	var adguardSvc api.AdGuard
	aghService := startAdGuard(ctx, deps, dnsLayers, workers, connection)
	if aghService != nil {
		adguardSvc = aghService
	}
	var tailscaleSvc api.Tailscale
	if svc := startTailscale(deps, dnsLayers, workers, connection, vpnStatus); svc != nil {
		tailscaleSvc = svc
	}
	apiDeps := api.Deps{
		DaemonContext: ctx,
		VPNProfiles:   profiles,
		VPNStatus:     vpnStatus,
		VPNTunnel:     vpnTunnel,
		VPNSplit:      vpnSplit,
		VPNKillSwitch: vpnKill,
		VPNDNS:        vpnDNS,
		UpdateApply:   updater,
		UpdateStage:   updateStage,
		Rebooter:      rebooter,
		BootID:        bootID,
		Backup:        backup,
		Updates:       updates,
		Connectivity:  connection,
		Captive:       captiveAssist,
		CaptiveRenew:  captiveRenew,
		CaptiveAccept: captiveAccept,
		WiFi:          wireless,
		WiFiJoin:      wirelessJoin,
		WiFiProfiles:  savedUplinks,
		AdGuard:       adguardSvc,
		Tailscale:     tailscaleSvc,
		WiFiReconnect: reconnect,
		WiFiAPs:       wirelessAPs,
		WiFiGuest:     wirelessGuest,
		WiFiMAC:       wirelessMAC,
		USBTether:     newUSBTether(deps),
		Failover:      startFailover(deps, connection, workers),
		Throughput:    sampler,
		Version:       version.Version, Accounts: deps.Accounts, Ubus: deps.Device, Txns: deps.Txn,
		Sessions: sessions, Limiter: limiter, Clock: clock,
		Tokens:    apitokens.New(filepath.Join(deps.Paths.State, "api-tokens.json")),
		Trusted:   provider.Allowed,
		RouterIPs: provider.RouterIPs, HostAllow: hostAllow,
		SaveEpoch: func(n uint64) error { return epochfile.Save(deps.Paths.EpochFile(), n) },
		UI:        uiHandler(staticDir),
	}
	cron := cronService(deps)
	if svc := startWiFiSchedule(ctx, deps, cron, hooks, wirelessAPs); svc != nil {
		apiDeps.WiFiSchedule = svc
	}
	if svc := startLEDs(deps, cron, hooks); svc != nil {
		apiDeps.LEDs = svc
	}
	if svc := startButtons(ctx, deps, hooks, vpnTunnel, wirelessGuest); svc != nil {
		apiDeps.Buttons = svc
	}
	if history := startStatsHistory(deps, workers); history != nil {
		apiDeps.StatsHistory = history
	}
	apiDeps.SystemSettings = &syssettings.Service{UCI: deps.Device, Apply: deps.Txn}
	apiDeps.OpenWrtUpdates = newOpenWrtUpdates(deps)
	apiDeps.Alerts = newAlerts(deps, aghService, reconnect)
	if f, ok := deps.Device.(sshkeys.File); ok {
		apiDeps.SSHKeys = &sshkeys.Service{File: f}
	}
	if r, ok := deps.Device.(factoryreset.Resetter); ok {
		apiDeps.FactoryReset = &factoryreset.Service{Device: r, Guards: deps.Guards, Stopped: workers.Stopped}
	}
	if r, ok := deps.Device.(systemlog.Reader); ok {
		apiDeps.SystemLog = &systemlog.Service{Log: r}
	}
	apiDeps.Clients = newClients(deps)
	apiDeps.NetworkView = &netview.Service{UCI: deps.Device, Ubus: deps.Device}
	if r, ok := deps.Device.(device.InterfaceRestarter); ok {
		apiDeps.InterfaceRestart = &ifrestart.Service{Device: r, Ubus: deps.Device, Guards: deps.Guards, Lockout: staLockout,
			Now: clock.Now, Sleep: ifrestart.SleepContext, Wait: ifrestart.DefaultWait, Poll: ifrestart.DefaultPoll}
	}
	if d, ok := deps.Device.(device.Diagnoser); ok {
		apiDeps.Diagnostics = &diagnostics.Service{Device: d}
	}
	if deps.Txn != nil {
		leases, _ := deps.Device.(device.DHCPLeaseReader)
		apiDeps.DHCP = &dhcpserver.Service{UCI: deps.Device, Leases: leases, Reload: deps.Device, Txns: deps.Txn}
		apiDeps.WAN = newWAN(deps)
		apiDeps.PortForwards = &portfwd.Service{UCI: deps.Device, Txns: deps.Txn}
		apiDeps.LAN = &lanaddress.Service{UCI: deps.Device, Ubus: deps.Device, Txns: deps.Txn, Now: time.Now,
			OnMoved: func(c context.Context) { _ = provider.Refresh(c, deps.Device) }}
	}
	if w, ok := deps.Device.(device.WakeSender); ok {
		apiDeps.WakeOnLAN = &wol.Service{Ubus: deps.Device, Sender: w}
	}
	h, _ := api.New(apiDeps)
	var certs *tlscert.Source
	if listenTLS != "" {
		certs = certSource(deps.Paths.State, provider, hostAllow)
	}
	proxyAddr := adguardProxyAddr(proxyListen, addr, useMock)
	stopProxy := serveAdGuardProxy(proxyAddr, proxyTLSAddr(proxyAddr, listenTLS), certs, apiDeps)
	defer stopProxy()
	srv := newServer(addr, h)
	stopTLS := func() {}
	if certs != nil {
		stopTLS = serveBackground("https", newServer(listenTLS, h), certs)
	}

	drained := make(chan struct{})
	go func() {
		defer close(drained)
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		stopTLS()
		_ = srv.Shutdown(shutdown) // HTTP drains first; workers stop after (deferred)
	}()
	slog.Info("listening", "addr", addr, "version", version.Version)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	<-drained
	return nil
}

// startState reports stale guards, recovers transactions and loads the session epoch.
func startState(ctx context.Context, deps app.Deps) (*auth.Store, error) {
	stale, err := deps.Guards.Stale()
	if err != nil {
		return nil, err
	}
	for _, g := range stale {
		slog.Warn("stale guard: the guarded feature stays off until cleared", "guard", g.Name, "since", g.Started)
	}
	if !deps.Mock { // mock mode has no watchdog scripts or /etc/config
		if _, err := deps.Txn.Recover(ctx, txn.ProcAlive); err != nil {
			return nil, err
		}
	}
	epoch, err := epochfile.Load(deps.Paths.EpochFile())
	if err != nil {
		return nil, err
	}
	s := auth.NewStore(rand.Reader)
	s.SetEpoch(epoch)
	return s, nil
}
