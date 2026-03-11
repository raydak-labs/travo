package auth

import (
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
)

func TestLoginSuccess(t *testing.T) {
	svc := NewAuthService("admin", "test-secret")
	token, expiry, err := svc.Login("admin")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token == "" {
		t.Error("expected non-empty token")
	}
	if expiry.Before(time.Now()) {
		t.Error("expected future expiry")
	}
}

func TestLoginFailure(t *testing.T) {
	svc := NewAuthService("admin", "test-secret")
	_, _, err := svc.Login("wrongpassword")
	if err == nil {
		t.Error("expected error for wrong password")
	}
}

func TestValidateTokenValid(t *testing.T) {
	svc := NewAuthService("admin", "test-secret")
	token, _, _ := svc.Login("admin")
	if err := svc.ValidateToken(token); err != nil {
		t.Fatalf("expected valid token, got error: %v", err)
	}
}

func TestValidateTokenExpired(t *testing.T) {
	svc := NewAuthService("admin", "test-secret")
	claims := jwt.RegisteredClaims{
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(-1 * time.Hour)),
		IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
		Subject:   "admin",
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, _ := token.SignedString([]byte("test-secret"))
	if err := svc.ValidateToken(signed); err == nil {
		t.Error("expected error for expired token")
	}
}

func TestValidateTokenInvalid(t *testing.T) {
	svc := NewAuthService("admin", "test-secret")
	if err := svc.ValidateToken("not-a-valid-token"); err == nil {
		t.Error("expected error for invalid token")
	}
}

func TestMiddlewareBlocksWithoutToken(t *testing.T) {
	svc := NewAuthService("admin", "test-secret")
	app := fiber.New()
	app.Use(svc.Middleware())
	app.Get("/api/v1/test", func(c *fiber.Ctx) error {
		return c.SendString("ok")
	})
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/test", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
}

func TestMiddlewareAllowsWithValidToken(t *testing.T) {
	svc := NewAuthService("admin", "test-secret")
	token, _, _ := svc.Login("admin")
	app := fiber.New()
	app.Use(svc.Middleware())
	app.Get("/api/v1/test", func(c *fiber.Ctx) error {
		return c.SendString("ok")
	})
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Errorf("expected 200, got %d, body: %s", resp.StatusCode, body)
	}
}

func TestMiddlewareAllowsHealthWithoutToken(t *testing.T) {
	svc := NewAuthService("admin", "test-secret")
	app := fiber.New()
	app.Use(svc.Middleware())
	app.Get("/api/health", func(c *fiber.Ctx) error {
		return c.SendString("ok")
	})
	req, _ := http.NewRequest(http.MethodGet, "/api/health", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestMiddlewareAllowsStaticFilesWithoutToken(t *testing.T) {
	svc := NewAuthService("admin", "test-secret")
	app := fiber.New()
	app.Use(svc.Middleware())
	app.Get("/*", func(c *fiber.Ctx) error {
		return c.SendString("ok")
	})

	paths := []string{"/", "/index.html", "/assets/style.css", "/assets/main.js"}
	for _, p := range paths {
		req, _ := http.NewRequest(http.MethodGet, p, nil)
		resp, err := app.Test(req, -1)
		if err != nil {
			t.Fatalf("request to %s failed: %v", p, err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected 200 for %s, got %d", p, resp.StatusCode)
		}
	}
}

func TestMiddlewareAllowsLoginWithoutToken(t *testing.T) {
	svc := NewAuthService("admin", "test-secret")
	app := fiber.New()
	app.Use(svc.Middleware())
	app.Post("/api/v1/auth/login", func(c *fiber.Ctx) error {
		return c.SendString("ok")
	})
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestChangePasswordSuccess(t *testing.T) {
	svc := NewAuthService("admin", "test-secret")
	if err := svc.ChangePassword("admin", "newpassword123"); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	// Old password should no longer work
	if _, _, err := svc.Login("admin"); err == nil {
		t.Error("expected old password to fail")
	}
	// New password should work
	if _, _, err := svc.Login("newpassword123"); err != nil {
		t.Errorf("expected new password to work, got: %v", err)
	}
}

func TestChangePasswordWrongCurrent(t *testing.T) {
	svc := NewAuthService("admin", "test-secret")
	err := svc.ChangePassword("wrongpassword", "newpassword123")
	if err == nil {
		t.Error("expected error for wrong current password")
	}
	if err.Error() != "invalid current password" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestChangePasswordTooShort(t *testing.T) {
	svc := NewAuthService("admin", "test-secret")
	err := svc.ChangePassword("admin", "short")
	if err == nil {
		t.Error("expected error for short password")
	}
	if err.Error() != "new password must be at least 6 characters" {
		t.Errorf("unexpected error: %v", err)
	}
}
