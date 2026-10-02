package auth

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func newSessionService(t *testing.T) (*AuthService, *SessionRegistry, *TokenBlocklist) {
	t.Helper()
	svc := NewAuthService("admin", "test-secret")
	reg := NewSessionRegistry(24 * time.Hour)
	bl := NewTokenBlocklist()
	svc.SetSessionRegistry(reg)
	svc.SetBlocklist(bl)
	return svc, reg, bl
}

// A password change must invalidate every live session (the attacker may hold
// one) and hand the caller a fresh token so it stays logged in.
func TestChangePassword_RevokesAllSessionsAndRotatesCallerToken(t *testing.T) {
	svc, _, _ := newSessionService(t)

	attacker, _, err := svc.Login("admin")
	if err != nil {
		t.Fatalf("attacker login: %v", err)
	}
	caller, _, err := svc.Login("admin")
	if err != nil {
		t.Fatalf("caller login: %v", err)
	}
	if err := svc.ValidateToken(attacker); err != nil {
		t.Fatalf("attacker token should be valid before the change: %v", err)
	}

	res, err := svc.ChangePassword("admin", "newpassword123")
	if err != nil {
		t.Fatalf("ChangePassword: %v", err)
	}
	if res.Token == "" {
		t.Fatal("expected a fresh token for the caller")
	}
	if res.RevokedSessions < 2 {
		t.Errorf("expected both live sessions to be revoked, got %d", res.RevokedSessions)
	}
	if res.Token == caller || res.Token == attacker {
		t.Error("expected a newly issued token, not one of the old ones")
	}

	if err := svc.ValidateToken(caller); err == nil {
		t.Error("expected the caller's previous token to be revoked")
	}
	if err := svc.ValidateToken(attacker); err == nil {
		t.Error("expected the other session to be revoked")
	}
	if err := svc.ValidateToken(res.Token); err != nil {
		t.Errorf("expected the fresh token to validate, got %v", err)
	}
}

// The registry is in-memory, so a revocation only survives a restart because
// the jti is blocklisted. Without that, every token issued before a password
// change would resurrect for the rest of its exp after a redeploy.
func TestChangePassword_RevokedTokensStayInvalidAfterRestart(t *testing.T) {
	svc, _, bl := newSessionService(t)
	secret := "test-secret"

	stolen, _, err := svc.Login("admin")
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if _, err := svc.ChangePassword("admin", "newpassword123"); err != nil {
		t.Fatalf("ChangePassword: %v", err)
	}

	// Simulate a backend restart: fresh registry, same JWT secret, same
	// (persistent) blocklist.
	restarted := NewAuthServiceWithHash("", secret)
	restarted.SetSessionRegistry(NewSessionRegistry(24 * time.Hour))
	restarted.SetBlocklist(bl)

	if err := restarted.ValidateToken(stolen); err == nil {
		t.Error("expected a token revoked by the password change to stay invalid after a restart")
	}
}

// Logout must have the same restart-proof property, otherwise a logged-out
// token comes back to life on the next deploy.
func TestRevokeSession_StaysInvalidAfterRestart(t *testing.T) {
	svc, _, bl := newSessionService(t)

	token, _, err := svc.Login("admin")
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	svc.RevokeSession(token)

	restarted := NewAuthServiceWithHash("", "test-secret")
	restarted.SetSessionRegistry(NewSessionRegistry(24 * time.Hour))
	restarted.SetBlocklist(bl)
	if err := restarted.ValidateToken(token); err == nil {
		t.Error("expected a logged-out token to stay invalid after a restart")
	}
}

func TestChangePassword_PolicyFloor(t *testing.T) {
	svc := NewAuthService("admin", "test-secret")
	if MinPasswordLength < 8 {
		t.Fatalf("password policy floor must be at least 8, got %d", MinPasswordLength)
	}
	_, err := svc.ChangePassword("admin", "short12")
	if err == nil {
		t.Fatal("expected a password below the policy floor to be rejected")
	}
	want := fmt.Sprintf("new password must be at least %d characters", MinPasswordLength)
	if err.Error() != want {
		t.Errorf("got %q, want %q", err.Error(), want)
	}
	// Exactly at the floor is accepted (the rejected call left the password unchanged).
	if _, err := svc.ChangePassword("admin", "abcdefgh"); err != nil {
		t.Errorf("expected a password at the policy floor to be accepted, got %v", err)
	}
}

// passwordHash is written by ChangePassword while Login reads it; concurrent
// use must not race (run with -race).
func TestAuthService_ConcurrentLoginAndChangePassword(t *testing.T) {
	svc := NewAuthService("admin", "test-secret")
	svc.SetSessionRegistry(NewSessionRegistry(time.Hour))

	var wg sync.WaitGroup
	for i := range 8 {
		current := "admin"
		if i%2 == 1 {
			current = "rotatedpassword"
		}
		wg.Add(2)
		go func() {
			defer wg.Done()
			// Either outcome is fine — only the absence of a data race matters.
			_, _, _ = svc.Login(current)
		}()
		go func() {
			defer wg.Done()
			_, _ = svc.ChangePassword(current, "rotatedpassword")
		}()
	}
	wg.Wait()
}
