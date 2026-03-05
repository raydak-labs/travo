package services

import (
	"fmt"
	"testing"
)

func TestCaptivePortal_NoRedirect_NoPortal(t *testing.T) {
	prober := &MockHTTPProber{
		StatusCode: 200,
		Body:       "success\n",
	}
	svc := NewCaptiveService(prober)

	status, err := svc.CheckCaptivePortal()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status.Detected {
		t.Error("expected no captive portal detected")
	}
	if !status.CanReachInternet {
		t.Error("expected internet reachable")
	}
	if status.PortalURL != nil {
		t.Error("expected no portal URL")
	}
}

func TestCaptivePortal_Redirect_PortalDetected(t *testing.T) {
	prober := &MockHTTPProber{
		StatusCode:  302,
		Body:        "",
		RedirectURL: "http://portal.hotel.com/login",
	}
	svc := NewCaptiveService(prober)

	status, err := svc.CheckCaptivePortal()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !status.Detected {
		t.Error("expected captive portal detected")
	}
	if status.CanReachInternet {
		t.Error("expected internet NOT reachable")
	}
	if status.PortalURL == nil {
		t.Fatal("expected portal URL")
	}
	if *status.PortalURL != "http://portal.hotel.com/login" {
		t.Errorf("expected portal URL 'http://portal.hotel.com/login', got %q", *status.PortalURL)
	}
}

func TestCaptivePortal_ConnectionFailed_NoInternet(t *testing.T) {
	prober := &MockHTTPProber{
		Err: fmt.Errorf("dial tcp: connection refused"),
	}
	svc := NewCaptiveService(prober)

	status, err := svc.CheckCaptivePortal()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status.Detected {
		t.Error("expected no captive portal detected on connection failure")
	}
	if status.CanReachInternet {
		t.Error("expected internet NOT reachable")
	}
}

func TestCaptivePortal_WrongBody_PortalDetected(t *testing.T) {
	prober := &MockHTTPProber{
		StatusCode: 200,
		Body:       "<html><body>Please login to continue</body></html>",
	}
	svc := NewCaptiveService(prober)

	status, err := svc.CheckCaptivePortal()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !status.Detected {
		t.Error("expected captive portal detected when body is wrong")
	}
	if status.CanReachInternet {
		t.Error("expected internet NOT reachable")
	}
}

func TestCaptivePortal_301Redirect(t *testing.T) {
	prober := &MockHTTPProber{
		StatusCode:  301,
		Body:        "",
		RedirectURL: "http://captive.example.com",
	}
	svc := NewCaptiveService(prober)

	status, err := svc.CheckCaptivePortal()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !status.Detected {
		t.Error("expected captive portal detected on 301")
	}
}

func TestCaptivePortal_307Redirect(t *testing.T) {
	prober := &MockHTTPProber{
		StatusCode:  307,
		Body:        "",
		RedirectURL: "http://captive.example.com",
	}
	svc := NewCaptiveService(prober)

	status, err := svc.CheckCaptivePortal()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !status.Detected {
		t.Error("expected captive portal detected on 307")
	}
}
