package auth

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	"github.com/openwrt-travel-gui/backend/internal/ubus"
)

// defaultTokenTTL is the session lifetime when no session registry is attached.
const defaultTokenTTL = 24 * time.Hour

// expValidationLeeway tolerates small clock skew when falling back to
// wall-clock exp validation (tokens issued before a backend restart).
const expValidationLeeway = 2 * time.Minute

// AuthService handles authentication and JWT tokens.
type AuthService struct {
	passwordHash   []byte
	jwtSecret      []byte
	blocklist      *TokenBlocklist
	sessions       *SessionRegistry
	ubus           ubus.Ubus
	rootPassword   *RootPassword
	authConfigPath string // path to auth.json; used to persist sealed rpcd login password
}

// SetBlocklist attaches a token blocklist to the auth service.
func (a *AuthService) SetBlocklist(bl *TokenBlocklist) {
	a.blocklist = bl
}

// Blocklist returns the attached token blocklist (may be nil).
func (a *AuthService) Blocklist() *TokenBlocklist {
	return a.blocklist
}

// SetSessionRegistry attaches a monotonic session registry. When set, session
// validity is decided by the registry (immune to wall-clock jumps); tokens with
// an unknown jti fall back to exp validation.
func (a *AuthService) SetSessionRegistry(r *SessionRegistry) {
	a.sessions = r
}

// TokenTTL returns the session lifetime used for issued tokens.
func (a *AuthService) TokenTTL() time.Duration {
	if a.sessions != nil {
		return a.sessions.TTL()
	}
	return defaultTokenTTL
}

// NewAuthService creates an AuthService with the given password and JWT secret.
func NewAuthService(password, jwtSecret string) *AuthService {
	hash, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return &AuthService{
		passwordHash: hash,
		jwtSecret:    []byte(jwtSecret),
	}
}

// NewAuthServiceWithHash creates an AuthService with an existing bcrypt hash.
func NewAuthServiceWithHash(passwordBcrypt, jwtSecret string) *AuthService {
	return &AuthService{
		passwordHash: []byte(passwordBcrypt),
		jwtSecret:    []byte(jwtSecret),
	}
}

// NewAuthServiceWithUbus validates credentials via rpcd/LuCI session login.
// authConfigPath is the path to auth.json (same as FileAuthStore); used to read/write
// rpcd-login.sealed next to it. Pass empty to disable persistence (e.g. tests).
func NewAuthServiceWithUbus(ub ubus.Ubus, jwtSecret string, rootPassword *RootPassword, authConfigPath string) *AuthService {
	return &AuthService{
		jwtSecret:      []byte(jwtSecret),
		ubus:           ub,
		rootPassword:   rootPassword,
		authConfigPath: authConfigPath,
	}
}

// Login verifies the password and returns a JWT token with expiry.
func (a *AuthService) Login(password string) (string, time.Time, error) {
	if a.ubus != nil {
		if err := a.verifyWithUbus(password); err != nil {
			return "", time.Time{}, errors.New("invalid password")
		}
	} else {
		if err := bcrypt.CompareHashAndPassword(a.passwordHash, []byte(password)); err != nil {
			return "", time.Time{}, errors.New("invalid password")
		}
	}

	jti := newJTI()
	expiry := time.Now().Add(a.TokenTTL())
	claims := jwt.RegisteredClaims{
		ID:        jti,
		ExpiresAt: jwt.NewNumericDate(expiry),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
		Subject:   "admin",
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(a.jwtSecret)
	if err != nil {
		return "", time.Time{}, err
	}

	if a.sessions != nil {
		a.sessions.Register(jti)
	}
	return signed, expiry, nil
}

// newJTI returns a random 128-bit hex token ID.
func newJTI() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func (a *AuthService) tryUbusLogin(password string) error {
	resp, err := a.ubus.Call("session", "login", map[string]interface{}{
		"username": "root",
		"password": password,
	})
	if err != nil {
		return err
	}
	if s, _ := resp["ubus_rpc_session"].(string); s != "" {
		return nil
	}
	if sid := ubus.ExtractSessionID(resp); sid != "" {
		return nil
	}
	return fmt.Errorf("login failed")
}

func (a *AuthService) persistSealedLogin(password string) {
	if a.authConfigPath == "" {
		return
	}
	if err := SaveSealedRPCDPassword(a.authConfigPath, string(a.jwtSecret), password); err != nil {
		log.Printf("WARNING: could not persist rpcd-login seal: %v", err)
	}
}

func (a *AuthService) verifyWithUbus(password string) error {
	if err := a.tryUbusLogin(password); err != nil {
		return err
	}
	if a.rootPassword != nil {
		a.rootPassword.Set(password)
	}
	a.persistSealedLogin(password)
	return nil
}

// parseSignedClaims verifies the token signature and returns its claims
// without validating time-based claims — session lifetime is decided by the
// registry (monotonic clock) or the exp fallback in validateLifetime.
func (a *AuthService) parseSignedClaims(tokenStr string) (jwt.MapClaims, error) {
	parser := jwt.NewParser(jwt.WithoutClaimsValidation())
	token, err := parser.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return a.jwtSecret, nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, errors.New("invalid claims")
	}
	return claims, nil
}

// validateLifetime decides whether a token's session is still alive.
// Registry verdicts (monotonic clock) win over the wall-clock exp claim; the
// exp fallback only applies to tokens the registry does not know (issued
// before a restart, or no registry attached).
func (a *AuthService) validateLifetime(claims jwt.MapClaims) error {
	if a.sessions != nil {
		if jti, _ := claims["jti"].(string); jti != "" {
			if remaining, known := a.sessions.Status(jti); known {
				if remaining <= 0 {
					return errors.New("session expired")
				}
				return nil
			}
		}
	}
	exp, err := claims.GetExpirationTime()
	if err != nil || exp == nil {
		return errors.New("missing expiry claim")
	}
	if time.Now().After(exp.Add(expValidationLeeway)) {
		return errors.New("token expired")
	}
	return nil
}

// ValidateToken parses and validates a JWT token string.
func (a *AuthService) ValidateToken(tokenStr string) error {
	claims, err := a.parseSignedClaims(tokenStr)
	if err != nil {
		return err
	}
	return a.validateLifetime(claims)
}

// TokenRemaining returns how long the token's session remains valid. Uses the
// monotonic session registry when the token is known there, otherwise the exp
// claim relative to the wall clock.
func (a *AuthService) TokenRemaining(tokenStr string) (time.Duration, error) {
	claims, err := a.parseSignedClaims(tokenStr)
	if err != nil {
		return 0, err
	}
	if a.sessions != nil {
		if jti, _ := claims["jti"].(string); jti != "" {
			if remaining, known := a.sessions.Status(jti); known {
				return remaining, nil
			}
		}
	}
	exp, err := claims.GetExpirationTime()
	if err != nil || exp == nil {
		return 0, errors.New("missing expiry claim")
	}
	return time.Until(exp.Time), nil
}

// RevokeSession removes the token's session from the registry (logout).
// Blocklist entries handle tokens the registry does not know.
func (a *AuthService) RevokeSession(tokenStr string) {
	if a.sessions == nil {
		return
	}
	claims, err := a.parseSignedClaims(tokenStr)
	if err != nil {
		return
	}
	if jti, _ := claims["jti"].(string); jti != "" {
		a.sessions.Remove(jti)
	}
}

// TokenExpiry parses a JWT token and returns its expiration time.
func (a *AuthService) TokenExpiry(tokenStr string) (time.Time, error) {
	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return a.jwtSecret, nil
	})
	if err != nil {
		return time.Time{}, err
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return time.Time{}, errors.New("invalid claims")
	}
	exp, err := claims.GetExpirationTime()
	if err != nil {
		return time.Time{}, err
	}
	return exp.Time, nil
}

// ChangePassword verifies the current password and updates to the new one.
func (a *AuthService) ChangePassword(currentPassword, newPassword string) error {
	if a.ubus != nil {
		if err := a.tryUbusLogin(currentPassword); err != nil {
			return errors.New("invalid current password")
		}
		if len(newPassword) < 6 {
			return errors.New("new password must be at least 6 characters")
		}
		_, err := a.ubus.Call("luci", "setPassword", map[string]interface{}{
			"username": "root",
			"password": newPassword,
		})
		if err != nil {
			return err
		}
		if a.rootPassword != nil {
			a.rootPassword.Set(newPassword)
		}
		a.persistSealedLogin(newPassword)
		return nil
	}

	if err := bcrypt.CompareHashAndPassword(a.passwordHash, []byte(currentPassword)); err != nil {
		return errors.New("invalid current password")
	}
	if len(newPassword) < 6 {
		return errors.New("new password must be at least 6 characters")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	a.passwordHash = hash
	return nil
}

// Middleware returns a Fiber middleware that checks for a valid Bearer token.
func (a *AuthService) Middleware() fiber.Handler {
	return func(c fiber.Ctx) error {
		path := c.Path()

		if !strings.HasPrefix(path, "/api/") || path == "/api/health" || path == "/api/openapi.json" || path == "/api/v1/auth/login" || path == "/api/v1/ws" || path == "/api/v1/system/time-sync" {
			return c.Next()
		}

		authHeader := c.Get("Authorization")
		if authHeader == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "missing authorization header",
			})
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "invalid authorization format",
			})
		}

		if err := a.ValidateToken(parts[1]); err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "invalid token",
			})
		}

		if a.blocklist != nil && a.blocklist.IsBlocked(parts[1]) {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "token has been revoked",
			})
		}

		return c.Next()
	}
}
