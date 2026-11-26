package main

import (
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/blackdark-org/travo-greenfield/internal/tlscert"
)

func freeAddr(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	return l.Addr().String()
}

func TestTLSListenerServesTheSameHandlerWithTheGeneratedCertificate(t *testing.T) {
	certs := &tlscert.Source{
		Dir: t.TempDir(), Now: time.Now, Random: rand.Reader,
		Names: func() tlscert.Names { return tlscert.Names{DNS: []string{"localhost"}} },
	}
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, map[bool]string{true: "tls", false: "plain"}[r.TLS != nil])
	})
	addr := freeAddr(t)
	stop := serveBackground("https", newServer(addr, h), certs)
	defer stop()

	cert, err := certs.Current()
	if err != nil {
		t.Fatal(err)
	}
	pool := x509.NewCertPool()
	pool.AddCert(cert.Leaf)
	client := &http.Client{Timeout: 2 * time.Second, Transport: &http.Transport{TLSClientConfig: &tls.Config{RootCAs: pool, ServerName: "localhost"}}}
	var body []byte
	for i := 0; i < 50; i++ { // the listener starts in a goroutine
		resp, err := client.Get("https://" + addr + "/")
		if err == nil {
			body, _ = io.ReadAll(resp.Body)
			resp.Body.Close()
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if string(body) != "tls" {
		t.Fatalf("body = %q", body)
	}
}

func TestServerTimeoutsFollowTheContract(t *testing.T) {
	srv := newServer("127.0.0.1:0", http.NotFoundHandler())
	if srv.ReadTimeout != 5*time.Minute || srv.WriteTimeout != 45*time.Minute ||
		srv.ReadHeaderTimeout != 10*time.Second || srv.IdleTimeout != 60*time.Second {
		t.Fatalf("timeouts %v %v %v %v", srv.ReadTimeout, srv.WriteTimeout, srv.ReadHeaderTimeout, srv.IdleTimeout)
	}
}

func TestProxyTLSAddrFollowsTheTLSListener(t *testing.T) {
	for _, c := range []struct{ proxy, listenTLS, want string }{
		{"0.0.0.0:3001", ":443", ":3443"},
		{"0.0.0.0:3001", "192.168.1.1:443", "192.168.1.1:3443"},
		{"", ":443", ""},            // proxy off
		{"0.0.0.0:3001", "", ""},    // TLS off
		{"0.0.0.0:3001", "bad", ""}, // unparsable
	} {
		if got := proxyTLSAddr(c.proxy, c.listenTLS); got != c.want {
			t.Errorf("proxyTLSAddr(%q, %q) = %q, want %q", c.proxy, c.listenTLS, got, c.want)
		}
	}
}
