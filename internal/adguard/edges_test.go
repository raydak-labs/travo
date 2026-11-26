package adguard

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"
	"time"
)

func TestHTTPReadyPollsUntilStatusAnswers(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/control/status" || calls.Add(1) < 3 {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		_, _ = w.Write([]byte(`{"running":true}`))
	}))
	defer srv.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := HTTPReady(srv.Client(), srv.URL, time.Millisecond)(ctx); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 3 {
		t.Fatalf("calls = %d", calls.Load())
	}
}

func TestHTTPReadyGivesUpAtDeadline(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(500) }))
	defer srv.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	if err := HTTPReady(srv.Client(), srv.URL, 5*time.Millisecond)(ctx); err == nil {
		t.Fatal("want deadline error")
	}
}

func TestFreeBytesFallsBackToTheNextPath(t *testing.T) {
	free, err := FreeBytes("/nonexistent-overlay", os.TempDir())()
	if err != nil || free == 0 {
		t.Fatalf("%d %v", free, err)
	}
	if _, err := FreeBytes("/nonexistent-overlay")(); err == nil {
		t.Fatal("want error")
	}
}
