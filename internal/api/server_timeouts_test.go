package api

import (
	"bufio"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"

	"github.com/blackdark-org/travo-greenfield/internal/auth"
)

// The daemon's http.Server sets ReadTimeout and WriteTimeout (invariants §4). These tests
// run a real server with tiny timeouts and prove the two long requests set their own
// per-request deadlines: the dashboard stream outlives WriteTimeout, an upload with a
// BodyReadTimeout (the firmware image) outlives ReadTimeout.
const tinyTimeout = 150 * time.Millisecond

func TestDashboardStreamOutlivesServerWriteTimeout(t *testing.T) {
	store := auth.NewStore(zeroEntropy())
	clock := &fakeClock{t: time.Now()}
	token, _, err := store.Create(clock.Now())
	if err != nil {
		t.Fatal(err)
	}
	s := &server{sessions: store, clock: clock}
	svc := newDashboardEventService(context.Background(), 8, nil)
	heartbeat := make(chan time.Time, 1)
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sub, err := svc.subscribe()
		if err != nil {
			return
		}
		defer sub.close()
		w.Header().Set("Content-Type", "text/event-stream")
		s.serveDashboardEvents(r.Context(), w, sub, authed{token: token}, heartbeat, time.Second)
	}))
	srv.Config.WriteTimeout = tinyTimeout
	srv.Start()
	defer srv.Close()

	resp, err := http.Get(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	lines := bufio.NewReader(resp.Body)
	if line, err := lines.ReadString('\n'); err != nil || line != ": connected\n" {
		t.Fatalf("first line %q, %v", line, err)
	}
	time.Sleep(4 * tinyTimeout)
	heartbeat <- time.Now()
	for {
		line, err := lines.ReadString('\n')
		if err != nil {
			t.Fatalf("stream ended after the server write timeout: %v", err)
		}
		if line == ": heartbeat\n" {
			return
		}
	}
}

func TestBodyReadTimeoutOutlivesServerReadTimeout(t *testing.T) {
	mux := http.NewServeMux()
	a := humago.New(mux, huma.DefaultConfig("test", "1"))
	type upload struct {
		RawBody []byte `contentType:"application/octet-stream"`
	}
	type out struct{ Body struct{ Size int } }
	huma.Register(a, huma.Operation{OperationID: "up", Method: http.MethodPost, Path: "/up", BodyReadTimeout: time.Minute},
		func(_ context.Context, in *upload) (*out, error) {
			o := &out{}
			o.Body.Size = len(in.RawBody)
			return o, nil
		})
	srv := httptest.NewUnstartedServer(mux)
	srv.Config.ReadTimeout = tinyTimeout
	srv.Start()
	defer srv.Close()

	body, w := io.Pipe()
	go func() {
		for range 4 { // trickle past the server ReadTimeout
			_, _ = w.Write([]byte("chunk"))
			time.Sleep(tinyTimeout)
		}
		_ = w.Close()
	}()
	resp, err := http.Post(srv.URL+"/up", "application/octet-stream", body)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	got, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK || !strings.Contains(string(got), `"Size":20`) {
		t.Fatalf("%d %s", resp.StatusCode, got)
	}
}
