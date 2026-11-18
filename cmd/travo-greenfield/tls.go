package main

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"path/filepath"
	"time"

	"github.com/blackdark-org/travo-greenfield/internal/tlscert"
	"github.com/blackdark-org/travo-greenfield/internal/trust"
	"github.com/blackdark-org/travo-greenfield/internal/version"
)

// adguardProxyTLSPort serves the AGH proxy to pages loaded over HTTPS: a session cookie set
// over TLS is Secure and never reaches the plain-HTTP proxy on 3001 (ADR 0016).
const adguardProxyTLSPort = "3443"

// certSource builds the lazily generated self-signed certificate in the state dir.
func certSource(stateDir string, provider *trust.Provider, hostAllow []string) *tlscert.Source {
	floor, _ := time.Parse(time.RFC3339, version.BuildTime) // unstamped dev build: zero floor
	return &tlscert.Source{
		Dir: filepath.Join(stateDir, "tls"), Floor: floor, Now: time.Now, Random: rand.Reader,
		Names: func() tlscert.Names {
			ips := append([]netip.Addr{netip.MustParseAddr("127.0.0.1")}, provider.RouterIPs()...)
			return tlscert.Names{DNS: append([]string{"localhost"}, hostAllow...), IPs: ips}
		},
	}
}

// proxyTLSAddr is the TLS twin of the AGH proxy: the TLS listener's host on 3443, only
// while both the plain proxy and TLS are on.
func proxyTLSAddr(proxyAddr, listenTLS string) string {
	if proxyAddr == "" || listenTLS == "" {
		return ""
	}
	host, _, err := net.SplitHostPort(listenTLS)
	if err != nil {
		return ""
	}
	return net.JoinHostPort(host, adguardProxyTLSPort)
}

// serveBackground runs srv until the returned stop is called. With certs it serves TLS.
// A listen failure is logged, not fatal: plain HTTP on --listen must keep working.
func serveBackground(name string, srv *http.Server, certs *tlscert.Source) (stop func()) {
	if srv.Addr == "" {
		return func() {}
	}
	if certs != nil {
		srv.TLSConfig = &tls.Config{MinVersion: tls.VersionTLS12, GetCertificate: certs.GetCertificate}
	}
	go func() {
		slog.Info(name+" listening", "addr", srv.Addr, "tls", certs != nil)
		var err error
		if certs != nil {
			err = srv.ListenAndServeTLS("", "")
		} else {
			err = srv.ListenAndServe()
		}
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Warn(name, "err", err)
		}
	}()
	return func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	}
}

func newServer(addr string, h http.Handler) *http.Server {
	return &http.Server{Addr: addr, Handler: h, ReadHeaderTimeout: 10 * time.Second, IdleTimeout: 60 * time.Second}
}
