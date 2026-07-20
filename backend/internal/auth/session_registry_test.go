package auth

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestSessionRegistry_RegisterAndStatus(t *testing.T) {
	r := NewSessionRegistry(24 * time.Hour)
	r.Register("abc")
	remaining, known := r.Status("abc")
	if !known {
		t.Fatal("expected session to be known")
	}
	if remaining <= 23*time.Hour || remaining > 24*time.Hour {
		t.Errorf("expected remaining close to 24h, got %v", remaining)
	}
}

func TestSessionRegistry_UnknownJti(t *testing.T) {
	r := NewSessionRegistry(24 * time.Hour)
	if _, known := r.Status("nope"); known {
		t.Error("expected unknown jti to report known=false")
	}
}

func TestSessionRegistry_ExpiredSessionStaysKnown(t *testing.T) {
	r := NewSessionRegistry(10 * time.Millisecond)
	r.Register("abc")
	time.Sleep(30 * time.Millisecond)
	remaining, known := r.Status("abc")
	if !known {
		t.Fatal("expected expired session to remain known (so TTL wins over exp fallback)")
	}
	if remaining > 0 {
		t.Errorf("expected non-positive remaining, got %v", remaining)
	}
}

func TestSessionRegistry_Remove(t *testing.T) {
	r := NewSessionRegistry(24 * time.Hour)
	r.Register("abc")
	r.Remove("abc")
	if _, known := r.Status("abc"); known {
		t.Error("expected removed session to be unknown")
	}
}

// issueToken creates a signed token with the given jti and exp for test scenarios.
func issueToken(t *testing.T, secret, jti string, exp time.Time) string {
	t.Helper()
	claims := jwt.RegisteredClaims{
		ID:        jti,
		ExpiresAt: jwt.NewNumericDate(exp),
		IssuedAt:  jwt.NewNumericDate(time.Now().Add(-time.Hour)),
		Subject:   "admin",
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("signing token: %v", err)
	}
	return signed
}

// A registered session must stay valid even when the wall clock says the token
// expired (e.g. NTP or time-sync jumped the clock forward after login).
func TestValidateToken_RegistrySurvivesWallClockJump(t *testing.T) {
	svc := NewAuthService("admin", "test-secret")
	reg := NewSessionRegistry(24 * time.Hour)
	svc.SetSessionRegistry(reg)

	signed := issueToken(t, "test-secret", "jump-jti", time.Now().Add(-time.Hour))
	reg.Register("jump-jti")

	if err := svc.ValidateToken(signed); err != nil {
		t.Errorf("expected registered session to survive wall clock jump, got: %v", err)
	}
}

// A session the registry knows to be expired must be rejected even if the wall
// clock was set backwards so the exp claim still looks valid.
func TestValidateToken_RegistryExpiryWinsOverExp(t *testing.T) {
	svc := NewAuthService("admin", "test-secret")
	reg := NewSessionRegistry(10 * time.Millisecond)
	svc.SetSessionRegistry(reg)

	signed := issueToken(t, "test-secret", "old-jti", time.Now().Add(24*time.Hour))
	reg.Register("old-jti")
	time.Sleep(30 * time.Millisecond)

	if err := svc.ValidateToken(signed); err == nil {
		t.Error("expected expired registered session to be rejected")
	}
}

// Tokens with an unknown jti (e.g. issued before a backend restart) fall back
// to standard exp validation against the wall clock.
func TestValidateToken_UnknownJtiFallsBackToExp(t *testing.T) {
	svc := NewAuthService("admin", "test-secret")
	svc.SetSessionRegistry(NewSessionRegistry(24 * time.Hour))

	valid := issueToken(t, "test-secret", "restart-jti", time.Now().Add(time.Hour))
	if err := svc.ValidateToken(valid); err != nil {
		t.Errorf("expected unknown-jti token with future exp to validate, got: %v", err)
	}

	expired := issueToken(t, "test-secret", "restart-jti-2", time.Now().Add(-time.Hour))
	if err := svc.ValidateToken(expired); err == nil {
		t.Error("expected unknown-jti token with past exp to be rejected")
	}
}

func TestLogin_RegistersSession(t *testing.T) {
	svc := NewAuthService("admin", "test-secret")
	reg := NewSessionRegistry(24 * time.Hour)
	svc.SetSessionRegistry(reg)

	token, _, err := svc.Login("admin")
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}

	parsed, err := jwt.Parse(token, func(t *jwt.Token) (interface{}, error) {
		return []byte("test-secret"), nil
	})
	if err != nil {
		t.Fatalf("parsing token: %v", err)
	}
	claims, ok := parsed.Claims.(jwt.MapClaims)
	if !ok {
		t.Fatal("expected map claims")
	}
	jti, _ := claims["jti"].(string)
	if jti == "" {
		t.Fatal("expected token to carry a jti claim")
	}
	if _, known := reg.Status(jti); !known {
		t.Error("expected login to register the session jti")
	}
}

func TestTokenRemaining(t *testing.T) {
	svc := NewAuthService("admin", "test-secret")
	svc.SetSessionRegistry(NewSessionRegistry(24 * time.Hour))

	token, _, err := svc.Login("admin")
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}
	remaining, err := svc.TokenRemaining(token)
	if err != nil {
		t.Fatalf("TokenRemaining failed: %v", err)
	}
	if remaining <= 23*time.Hour || remaining > 24*time.Hour {
		t.Errorf("expected remaining close to 24h, got %v", remaining)
	}
}

func TestRevokeSession_RemovesJti(t *testing.T) {
	svc := NewAuthService("admin", "test-secret")
	reg := NewSessionRegistry(24 * time.Hour)
	svc.SetSessionRegistry(reg)

	token, _, err := svc.Login("admin")
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}
	svc.RevokeSession(token)

	parsed, _ := jwt.Parse(token, func(t *jwt.Token) (interface{}, error) {
		return []byte("test-secret"), nil
	})
	claims := parsed.Claims.(jwt.MapClaims)
	jti := claims["jti"].(string)
	if _, known := reg.Status(jti); known {
		t.Error("expected revoked session jti to be removed from registry")
	}
}
