package auth

import (
	"errors"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

// AuthService handles authentication and JWT tokens.
type AuthService struct {
	passwordHash []byte
	jwtSecret    []byte
	blocklist    *TokenBlocklist
}

// SetBlocklist attaches a token blocklist to the auth service.
func (a *AuthService) SetBlocklist(bl *TokenBlocklist) {
	a.blocklist = bl
}

// Blocklist returns the attached token blocklist (may be nil).
func (a *AuthService) Blocklist() *TokenBlocklist {
	return a.blocklist
}

// NewAuthService creates an AuthService with the given password and JWT secret.
func NewAuthService(password, jwtSecret string) *AuthService {
	hash, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return &AuthService{
		passwordHash: hash,
		jwtSecret:    []byte(jwtSecret),
	}
}

// Login verifies the password and returns a JWT token with expiry.
func (a *AuthService) Login(password string) (string, time.Time, error) {
	if err := bcrypt.CompareHashAndPassword(a.passwordHash, []byte(password)); err != nil {
		return "", time.Time{}, errors.New("invalid password")
	}

	expiry := time.Now().Add(24 * time.Hour)
	claims := jwt.RegisteredClaims{
		ExpiresAt: jwt.NewNumericDate(expiry),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
		Subject:   "admin",
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(a.jwtSecret)
	if err != nil {
		return "", time.Time{}, err
	}

	return signed, expiry, nil
}

// ValidateToken parses and validates a JWT token string.
func (a *AuthService) ValidateToken(tokenStr string) error {
	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return a.jwtSecret, nil
	})
	if err != nil {
		return err
	}
	if !token.Valid {
		return errors.New("invalid token")
	}
	return nil
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
	return func(c *fiber.Ctx) error {
		path := c.Path()

		if !strings.HasPrefix(path, "/api/") || path == "/api/health" || path == "/api/v1/auth/login" || path == "/api/v1/ws" || path == "/api/v1/system/time-sync" {
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
